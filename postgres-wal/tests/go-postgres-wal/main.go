package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
)

type config struct {
	Host               string
	Port               int
	Database           string
	User               string
	Password           string
	SuperuserPassword  string
	Concurrency        int
	Requests           int
	SkipRestore        bool
	EnvPath            string
	PrimaryContainer   string
	BackupDir          string
	WALArchiveDir      string
	StatementTimeout   time.Duration
	DockerCommandDelay time.Duration
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "\nFAIL: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("\nPASS: postgres-wal functional tests completed")
}

func run() error {
	root, err := stackRoot()
	if err != nil {
		return err
	}

	envPath := filepath.Join(root, "..", "postgres-wal.env")
	env, err := readEnv(envPath)
	if err != nil {
		return err
	}

	cfg := config{
		Host:               "localhost",
		Port:               atoiDefault(env["POSTGRES_HOST_PORT"], 55432),
		Database:           envDefault(env, "POSTGRES_DB", "appdb"),
		User:               envDefault(env, "POSTGRES_USER", "appuser"),
		Password:           env["POSTGRES_PASSWORD"],
		SuperuserPassword:  env["POSTGRES_SUPERUSER_PASSWORD"],
		Concurrency:        25,
		Requests:           250,
		EnvPath:            envPath,
		PrimaryContainer:   "postgres-wal-pg-0",
		BackupDir:          filepath.Join(root, "backups"),
		WALArchiveDir:      filepath.Join(root, "wal-archive"),
		StatementTimeout:   15 * time.Second,
		DockerCommandDelay: 2 * time.Second,
	}

	flag.StringVar(&cfg.Host, "host", cfg.Host, "database host exposed by Pgpool")
	flag.IntVar(&cfg.Port, "port", cfg.Port, "database host port exposed by Pgpool")
	flag.IntVar(&cfg.Concurrency, "concurrency", cfg.Concurrency, "number of concurrent workers")
	flag.IntVar(&cfg.Requests, "requests", cfg.Requests, "total SQL requests for load test")
	flag.BoolVar(&cfg.SkipRestore, "skip-restore", false, "skip pg_dump/restore test")
	flag.Parse()

	if cfg.Password == "" || cfg.SuperuserPassword == "" {
		return fmt.Errorf("missing POSTGRES_PASSWORD or POSTGRES_SUPERUSER_PASSWORD in %s", cfg.EnvPath)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	printStep("checking Pgpool application connection")
	if err := checkAppConnection(ctx, cfg); err != nil {
		return err
	}

	printStep("running concurrent request test")
	if err := runConcurrentRequests(ctx, cfg); err != nil {
		return err
	}

	printStep("checking active database connections")
	if err := checkConnectionCount(ctx, cfg); err != nil {
		return err
	}

	printStep("checking replication status from primary")
	if err := checkReplication(ctx, cfg); err != nil {
		return err
	}

	printStep("checking WAL archive generation")
	if err := checkWALArchive(ctx, cfg); err != nil {
		return err
	}

	if !cfg.SkipRestore {
		printStep("checking backup and restore round trip")
		if err := checkBackupRestore(ctx, cfg); err != nil {
			return err
		}
	}

	return nil
}

func checkAppConnection(ctx context.Context, cfg config) error {
	conn, err := pgx.Connect(ctx, appDSN(cfg))
	if err != nil {
		return fmt.Errorf("connect through Pgpool: %w", err)
	}
	defer conn.Close(ctx)

	var db, user string
	if err := conn.QueryRow(ctx, "select current_database(), current_user").Scan(&db, &user); err != nil {
		return fmt.Errorf("query through Pgpool: %w", err)
	}
	fmt.Printf("  connected database=%s user=%s host=%s port=%d\n", db, user, cfg.Host, cfg.Port)
	return nil
}

func runConcurrentRequests(ctx context.Context, cfg config) error {
	if cfg.Concurrency < 1 || cfg.Requests < 1 {
		return errors.New("concurrency and requests must be greater than zero")
	}

	jobs := make(chan int)
	var ok atomic.Int64
	var failed atomic.Int64
	var firstErr atomic.Value
	var wg sync.WaitGroup

	start := time.Now()
	for worker := 0; worker < cfg.Concurrency; worker++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			conn, err := pgx.Connect(ctx, appDSN(cfg))
			if err != nil {
				failed.Add(1)
				firstErr.CompareAndSwap(nil, fmt.Errorf("worker %d connect: %w", workerID, err))
				return
			}
			defer conn.Close(ctx)

			for job := range jobs {
				var result int
				err := conn.QueryRow(ctx, "select $1::int + 1", job).Scan(&result)
				if err != nil || result != job+1 {
					failed.Add(1)
					if err == nil {
						err = fmt.Errorf("unexpected result for job %d: %d", job, result)
					}
					firstErr.CompareAndSwap(nil, err)
					continue
				}
				ok.Add(1)
			}
		}(worker)
	}

	for i := 0; i < cfg.Requests; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	duration := time.Since(start)
	fmt.Printf("  requests=%d ok=%d failed=%d concurrency=%d duration=%s\n", cfg.Requests, ok.Load(), failed.Load(), cfg.Concurrency, duration.Round(time.Millisecond))

	if failed.Load() > 0 {
		if err, _ := firstErr.Load().(error); err != nil {
			return err
		}
		return fmt.Errorf("%d concurrent requests failed", failed.Load())
	}
	return nil
}

func checkConnectionCount(ctx context.Context, cfg config) error {
	conn, err := pgx.Connect(ctx, superDSN(cfg, "postgres"))
	if err != nil {
		return fmt.Errorf("connect as postgres through Pgpool: %w", err)
	}
	defer conn.Close(ctx)

	var total, active int
	if err := conn.QueryRow(ctx, "select count(*), count(*) filter (where state = 'active') from pg_stat_activity").Scan(&total, &active); err != nil {
		return fmt.Errorf("read pg_stat_activity: %w", err)
	}
	fmt.Printf("  total_connections=%d active_connections=%d\n", total, active)
	return nil
}

func checkReplication(ctx context.Context, cfg config) error {
	out, err := dockerExec(ctx, cfg.PrimaryContainer, []string{
		"psql", "-U", "postgres", "-d", "postgres", "-c",
		"select application_name, state, sync_state from pg_stat_replication order by application_name;",
	}, map[string]string{"PGPASSWORD": cfg.SuperuserPassword})
	if err != nil {
		return fmt.Errorf("replication query failed: %w\n%s", err, out)
	}
	fmt.Print(indent(out))
	if !strings.Contains(out, "pg-1") || !strings.Contains(out, "pg-2") || !strings.Contains(out, "streaming") {
		return errors.New("expected pg-1 and pg-2 to be streaming")
	}
	return nil
}

func checkWALArchive(ctx context.Context, cfg config) error {
	beforeFiles, err := countWALFiles(cfg.WALArchiveDir)
	if err != nil {
		return err
	}
	beforeArchived, err := archivedWALCount(ctx, cfg)
	if err != nil {
		return err
	}

	table := fmt.Sprintf("wal_test_probe_%d", time.Now().UnixNano())
	writeSQL := fmt.Sprintf(
		"create table %s(id int primary key, note text); insert into %s select g, repeat('wal-test-', 50) from generate_series(1, 5000) g; drop table %s;",
		table, table, table,
	)
	if out, err := dockerExec(ctx, cfg.PrimaryContainer, []string{
		"psql", "-v", "ON_ERROR_STOP=1", "-U", "postgres", "-d", cfg.Database, "-c", writeSQL,
	}, map[string]string{"PGPASSWORD": cfg.SuperuserPassword}); err != nil {
		return fmt.Errorf("generate WAL activity failed: %w\n%s", err, out)
	}

	out, err := dockerExec(ctx, cfg.PrimaryContainer, []string{
		"psql", "-U", "postgres", "-d", "postgres", "-c", "select pg_switch_wal();",
	}, map[string]string{"PGPASSWORD": cfg.SuperuserPassword})
	if err != nil {
		return fmt.Errorf("pg_switch_wal failed: %w\n%s", err, out)
	}

	var afterFiles, afterArchived int
	deadline := time.Now().Add(20 * time.Second)
	for {
		afterFiles, err = countWALFiles(cfg.WALArchiveDir)
		if err != nil {
			return err
		}
		afterArchived, err = archivedWALCount(ctx, cfg)
		if err != nil {
			return err
		}
		if afterFiles > beforeFiles || afterArchived > beforeArchived {
			break
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(1 * time.Second)
	}

	fmt.Printf("  wal_files_before=%d wal_files_after=%d archived_count_before=%d archived_count_after=%d archive_dir=%s\n",
		beforeFiles, afterFiles, beforeArchived, afterArchived, cfg.WALArchiveDir)
	if afterFiles <= beforeFiles && afterArchived <= beforeArchived {
		return errors.New("WAL archive did not advance after generating WAL and running pg_switch_wal")
	}
	return nil
}

func archivedWALCount(ctx context.Context, cfg config) (int, error) {
	out, err := dockerExec(ctx, cfg.PrimaryContainer, []string{
		"psql", "-tA", "-U", "postgres", "-d", "postgres", "-c", "select archived_count from pg_stat_archiver;",
	}, map[string]string{"PGPASSWORD": cfg.SuperuserPassword})
	if err != nil {
		return 0, fmt.Errorf("read pg_stat_archiver: %w\n%s", err, out)
	}
	value := strings.TrimSpace(out)
	count, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse archived_count %q: %w", value, err)
	}
	return count, nil
}

func checkBackupRestore(ctx context.Context, cfg config) error {
	if err := os.MkdirAll(cfg.BackupDir, 0755); err != nil {
		return err
	}

	table := fmt.Sprintf("wal_test_restore_%d", time.Now().Unix())
	dumpInContainer := "/tmp/" + table + ".sql"
	hostDump := filepath.Join(cfg.BackupDir, table+".sql")

	createSQL := fmt.Sprintf(
		"drop table if exists %s; create table %s(id int primary key, note text); insert into %s values (1, 'backup restore ok'), (2, 'wal stack ok');",
		table, table, table,
	)
	if out, err := dockerExec(ctx, cfg.PrimaryContainer, []string{"psql", "-U", "postgres", "-d", cfg.Database, "-c", createSQL}, map[string]string{"PGPASSWORD": cfg.SuperuserPassword}); err != nil {
		return fmt.Errorf("create restore test table failed: %w\n%s", err, out)
	}

	if out, err := dockerExec(ctx, cfg.PrimaryContainer, []string{"pg_dump", "-U", "postgres", "-d", cfg.Database, "-t", table, "-f", dumpInContainer}, map[string]string{"PGPASSWORD": cfg.SuperuserPassword}); err != nil {
		return fmt.Errorf("pg_dump failed: %w\n%s", err, out)
	}

	if out, err := runCommand(ctx, "docker", "cp", cfg.PrimaryContainer+":"+dumpInContainer, hostDump); err != nil {
		return fmt.Errorf("copy dump to host failed: %w\n%s", err, out)
	}

	if out, err := dockerExec(ctx, cfg.PrimaryContainer, []string{"psql", "-U", "postgres", "-d", cfg.Database, "-c", "drop table " + table + ";"}, map[string]string{"PGPASSWORD": cfg.SuperuserPassword}); err != nil {
		return fmt.Errorf("drop restore test table failed: %w\n%s", err, out)
	}

	if out, err := dockerExec(ctx, cfg.PrimaryContainer, []string{"psql", "-U", "postgres", "-d", cfg.Database, "-f", dumpInContainer}, map[string]string{"PGPASSWORD": cfg.SuperuserPassword}); err != nil {
		return fmt.Errorf("restore dump failed: %w\n%s", err, out)
	}

	var count int
	conn, err := pgx.Connect(ctx, superDSN(cfg, cfg.Database))
	if err != nil {
		return fmt.Errorf("connect for restore validation: %w", err)
	}
	defer conn.Close(ctx)
	if err := conn.QueryRow(ctx, "select count(*) from "+table).Scan(&count); err != nil {
		return fmt.Errorf("validate restored table: %w", err)
	}

	if _, err := conn.Exec(ctx, "drop table "+table); err != nil {
		return fmt.Errorf("cleanup restored table: %w", err)
	}
	if count != 2 {
		return fmt.Errorf("expected restored row count 2, got %d", count)
	}
	fmt.Printf("  restored_table=%s rows=%d dump=%s\n", table, count, hostDump)
	return nil
}

func appDSN(cfg config) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable&connect_timeout=%d",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database, int(cfg.StatementTimeout.Seconds()))
}

func superDSN(cfg config, db string) string {
	return fmt.Sprintf("postgres://postgres:%s@%s:%d/%s?sslmode=disable&connect_timeout=%d",
		cfg.SuperuserPassword, cfg.Host, cfg.Port, db, int(cfg.StatementTimeout.Seconds()))
}

func dockerExec(ctx context.Context, container string, args []string, env map[string]string) (string, error) {
	cmdArgs := []string{"exec"}
	for key, value := range env {
		cmdArgs = append(cmdArgs, "-e", key+"="+value)
	}
	cmdArgs = append(cmdArgs, container)
	cmdArgs = append(cmdArgs, args...)
	return runCommand(ctx, "docker", cmdArgs...)
}

func runCommand(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func countWALFiles(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("read WAL archive dir: %w", err)
	}
	count := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || strings.HasPrefix(name, ".") || strings.Contains(name, ".backup") {
			continue
		}
		if len(name) == 24 {
			count++
		}
	}
	return count, nil
}

func readEnv(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	values := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"'`)
	}
	return values, scanner.Err()
}

func stackRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if exists(filepath.Join(wd, "docker-compose.yml")) && filepath.Base(wd) == "postgres-wal" {
			return wd, nil
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			return "", errors.New("could not find postgres-wal stack root")
		}
		wd = parent
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func envDefault(env map[string]string, key, fallback string) string {
	if env[key] == "" {
		return fallback
	}
	return env[key]
}

func atoiDefault(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func indent(value string) string {
	lines := strings.Split(strings.TrimRight(value, "\n"), "\n")
	for i := range lines {
		lines[i] = "  " + lines[i]
	}
	return strings.Join(lines, "\n") + "\n"
}

func printStep(name string) {
	fmt.Printf("\n==> %s\n", name)
}
