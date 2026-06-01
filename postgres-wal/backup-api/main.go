package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/jackc/pgx/v5"
)

type appConfig struct {
	HTTPAddr             string
	APIKey               string
	PostgresHost         string
	PostgresPort         string
	PostgresDB           string
	PostgresUser         string
	PostgresPassword     string
	PostgresSuperPass    string
	BackupDir            string
	WALDir               string
	ScheduleEnabled      bool
	DailyTime            string
	Timezone             string
	RetentionCount       int
	WALUploadEnabled     bool
	WALUploadInterval    time.Duration
	WALUploadBatchSize   int
	WALUploadForceSwitch bool
	S3Enabled            bool
	S3Endpoint           string
	S3Region             string
	S3Bucket             string
	S3Prefix             string
	S3AccessKeyID        string
	S3SecretAccessKey    string
	S3ForcePathStyle     bool
	BackupTimeout        time.Duration
	SchedulerTickEvery   time.Duration
}

type server struct {
	cfg           appConfig
	s3            *s3.Client
	mu            sync.Mutex
	running       bool
	lastRun       *backupResult
	lastWALUpload *walUploadResult
	started       time.Time
	state         backupState
	stateMu       sync.Mutex
	stateLoc      string
}

type backupState struct {
	UploadedWAL    map[string]time.Time         `json:"uploaded_wal"`
	ActiveBackupID string                       `json:"active_backup_id,omitempty"`
	Generations    map[string]*backupGeneration `json:"generations,omitempty"`
}

type backupGeneration struct {
	ID             string               `json:"id"`
	BackupFile     string               `json:"backup_file"`
	S3BackupKey    string               `json:"s3_backup_key,omitempty"`
	SourceDatabase string               `json:"source_database"`
	StartedAt      time.Time            `json:"started_at"`
	FinishedAt     time.Time            `json:"finished_at"`
	LSNStart       string               `json:"lsn_start,omitempty"`
	LSNFinish      string               `json:"lsn_finish,omitempty"`
	StartWAL       string               `json:"start_wal,omitempty"`
	WALPrefix      string               `json:"wal_prefix,omitempty"`
	UploadedWAL    map[string]time.Time `json:"uploaded_wal,omitempty"`
	RestoreNote    string               `json:"restore_note"`
}

type backupResult struct {
	ID             string    `json:"id"`
	StartedAt      time.Time `json:"started_at"`
	FinishedAt     time.Time `json:"finished_at"`
	Duration       string    `json:"duration"`
	BackupFile     string    `json:"backup_file"`
	BackupSize     int64     `json:"backup_size"`
	LogicalDump    string    `json:"logical_dump"`
	S3BackupKey    string    `json:"s3_backup_key,omitempty"`
	WALUploaded    int       `json:"wal_uploaded"`
	LSNStart       string    `json:"lsn_start,omitempty"`
	LSNFinish      string    `json:"lsn_finish,omitempty"`
	Database       string    `json:"database"`
	PGBaseBackup   string    `json:"pg_basebackup"`
	RetentionCount int       `json:"retention_count"`
	RestoreNote    string    `json:"restore_note"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type restoreOption struct {
	Name                  string    `json:"name"`
	Path                  string    `json:"path"`
	Size                  int64     `json:"size"`
	Modified              time.Time `json:"modified"`
	SourceDatabase        string    `json:"source_database,omitempty"`
	LogicalDump           string    `json:"logical_dump,omitempty"`
	CanRestoreToDatabase  bool      `json:"can_restore_to_database"`
	RestoreKind           string    `json:"restore_kind"`
	PhysicalPITRSupported bool      `json:"physical_pitr_supported"`
	Note                  string    `json:"note"`
}

type restoreRequest struct {
	BackupName     string `json:"backup_name"`
	TargetDatabase string `json:"target_database"`
	DropIfExists   bool   `json:"drop_if_exists"`
}

type restoreResult struct {
	BackupName     string    `json:"backup_name"`
	TargetDatabase string    `json:"target_database"`
	StartedAt      time.Time `json:"started_at"`
	FinishedAt     time.Time `json:"finished_at"`
	Duration       string    `json:"duration"`
	RowsChecked    int       `json:"rows_checked"`
	Message        string    `json:"message"`
}

type pitrDownloadRequest struct {
	ChainID   string `json:"chain_id"`
	Latest    bool   `json:"latest"`
	Overwrite bool   `json:"overwrite"`
}

type pitrDownloadResult struct {
	ChainID        string    `json:"chain_id"`
	BackupName     string    `json:"backup_name"`
	BackupPath     string    `json:"backup_path"`
	S3BackupKey    string    `json:"s3_backup_key"`
	WALPrefix      string    `json:"wal_prefix"`
	WALDownloadDir string    `json:"wal_download_dir"`
	WALDownloaded  int       `json:"wal_downloaded"`
	StartedAt      time.Time `json:"started_at"`
	FinishedAt     time.Time `json:"finished_at"`
	Duration       string    `json:"duration"`
	NextStep       string    `json:"next_step"`
}

type walUploadResult struct {
	StartedAt      time.Time `json:"started_at"`
	FinishedAt     time.Time `json:"finished_at"`
	Duration       string    `json:"duration"`
	Uploaded       int       `json:"uploaded"`
	ActiveBackupID string    `json:"active_backup_id,omitempty"`
	WALPrefix      string    `json:"wal_prefix,omitempty"`
	Automatic      bool      `json:"automatic"`
	ForcedSwitch   bool      `json:"forced_switch"`
	Error          string    `json:"error,omitempty"`
}

type pitrChain struct {
	ID               string    `json:"id"`
	BackupFile       string    `json:"backup_file"`
	S3BackupKey      string    `json:"s3_backup_key,omitempty"`
	SourceDatabase   string    `json:"source_database"`
	StartedAt        time.Time `json:"started_at"`
	FinishedAt       time.Time `json:"finished_at"`
	LSNStart         string    `json:"lsn_start,omitempty"`
	LSNFinish        string    `json:"lsn_finish,omitempty"`
	StartWAL         string    `json:"start_wal,omitempty"`
	WALPrefix        string    `json:"wal_prefix,omitempty"`
	UploadedWALCount int       `json:"uploaded_wal_count"`
	Active           bool      `json:"active"`
	RestoreUse       string    `json:"restore_use"`
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		port := env("BACKUP_API_PORT", "8080")
		resp, err := http.Get("http://127.0.0.1:" + port + "/health")
		if err != nil {
			log.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			log.Fatalf("healthcheck returned %s", resp.Status)
		}
		return
	}

	cfg := loadConfig()
	if cfg.APIKey == "" {
		log.Fatal("BACKUP_API_KEY is required")
	}
	if cfg.PostgresSuperPass == "" {
		log.Fatal("POSTGRES_SUPERUSER_PASSWORD is required")
	}

	srv, err := newServer(cfg)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.scheduler(ctx)
	go srv.walUploader(ctx)

	mux := srv.routes()

	httpSrv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("backup API listening on %s", cfg.HTTPAddr)
	log.Fatal(httpSrv.ListenAndServe())
}

func newServer(cfg appConfig) (*server, error) {
	if err := os.MkdirAll(cfg.BackupDir, 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cfg.WALDir, 0755); err != nil {
		return nil, err
	}

	srv := &server{
		cfg:      cfg,
		started:  time.Now(),
		stateLoc: filepath.Join(cfg.BackupDir, "backup-api-state.json"),
		state: backupState{
			UploadedWAL: map[string]time.Time{},
			Generations: map[string]*backupGeneration{},
		},
	}
	if err := srv.loadState(); err != nil {
		return nil, err
	}
	if cfg.S3Enabled {
		client, err := newS3Client(context.Background(), cfg)
		if err != nil {
			return nil, err
		}
		srv.s3 = client
	}
	return srv, nil
}

func loadConfig() appConfig {
	return appConfig{
		HTTPAddr:             ":" + env("BACKUP_API_PORT", "8080"),
		APIKey:               env("BACKUP_API_KEY", ""),
		PostgresHost:         env("BACKUP_POSTGRES_HOST", "pg-0"),
		PostgresPort:         env("BACKUP_POSTGRES_PORT", "5432"),
		PostgresDB:           env("POSTGRES_DB", "appdb"),
		PostgresUser:         env("POSTGRES_USER", "appuser"),
		PostgresPassword:     env("POSTGRES_PASSWORD", ""),
		PostgresSuperPass:    env("POSTGRES_SUPERUSER_PASSWORD", ""),
		BackupDir:            env("BACKUP_LOCAL_DIR", "/backups"),
		WALDir:               env("BACKUP_WAL_DIR", "/wal-archive"),
		ScheduleEnabled:      envBool("BACKUP_SCHEDULE_ENABLED", true),
		DailyTime:            env("BACKUP_DAILY_TIME", "02:00"),
		Timezone:             env("BACKUP_TIMEZONE", "Asia/Manila"),
		RetentionCount:       envInt("BACKUP_RETENTION_COUNT", 7),
		WALUploadEnabled:     envBool("BACKUP_WAL_UPLOAD_ENABLED", true),
		WALUploadInterval:    time.Duration(envInt("BACKUP_WAL_UPLOAD_INTERVAL_SECONDS", 60)) * time.Second,
		WALUploadBatchSize:   envInt("BACKUP_WAL_UPLOAD_BATCH_SIZE", 10),
		WALUploadForceSwitch: envBool("BACKUP_WAL_UPLOAD_FORCE_SWITCH", false),
		S3Enabled:            envBool("BACKUP_S3_ENABLED", false),
		S3Endpoint:           env("BACKUP_S3_ENDPOINT", ""),
		S3Region:             env("BACKUP_S3_REGION", "us-east-1"),
		S3Bucket:             env("BACKUP_S3_BUCKET", ""),
		S3Prefix:             cleanPrefix(env("BACKUP_S3_PREFIX", "postgres-wal")),
		S3AccessKeyID:        env("BACKUP_S3_ACCESS_KEY_ID", ""),
		S3SecretAccessKey:    env("BACKUP_S3_SECRET_ACCESS_KEY", ""),
		S3ForcePathStyle:     envBool("BACKUP_S3_FORCE_PATH_STYLE", true),
		BackupTimeout:        time.Duration(envInt("BACKUP_TIMEOUT_MINUTES", 120)) * time.Minute,
		SchedulerTickEvery:   30 * time.Second,
	}
}

func (s *server) scheduler(ctx context.Context) {
	if !s.cfg.ScheduleEnabled {
		log.Print("backup schedule disabled")
		return
	}
	loc, err := time.LoadLocation(s.cfg.Timezone)
	if err != nil {
		log.Printf("invalid BACKUP_TIMEZONE=%s, using local timezone: %v", s.cfg.Timezone, err)
		loc = time.Local
	}
	var lastRunDate string
	ticker := time.NewTicker(s.cfg.SchedulerTickEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now().In(loc)
			if now.Format("15:04") != s.cfg.DailyTime {
				continue
			}
			date := now.Format("2006-01-02")
			if date == lastRunDate {
				continue
			}
			lastRunDate = date
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), s.cfg.BackupTimeout)
				defer cancel()
				if _, err := s.runBackup(ctx); err != nil {
					log.Printf("scheduled backup failed: %v", err)
				}
			}()
		}
	}
}

func (s *server) walUploader(ctx context.Context) {
	if !s.cfg.WALUploadEnabled {
		log.Print("automatic WAL upload disabled")
		return
	}
	if !s.cfg.S3Enabled {
		log.Print("automatic WAL upload disabled because S3 is disabled")
		return
	}
	interval := s.cfg.WALUploadInterval
	if interval < 10*time.Second {
		interval = 10 * time.Second
	}
	log.Printf("automatic WAL upload enabled every %s, batch_size=%d, force_switch=%v", interval, s.cfg.WALUploadBatchSize, s.cfg.WALUploadForceSwitch)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			uploadCtx, cancel := context.WithTimeout(ctx, interval)
			result := s.runWALUpload(uploadCtx, true, s.cfg.WALUploadBatchSize)
			cancel()
			if result.Error != "" && !strings.Contains(result.Error, "no active PITR backup generation") {
				log.Printf("automatic WAL upload failed: %s", result.Error)
			}
		}
	}
}

func (s *server) runWALUpload(ctx context.Context, automatic bool, limit int) *walUploadResult {
	started := time.Now().UTC()
	result := &walUploadResult{
		StartedAt:    started,
		Automatic:    automatic,
		ForcedSwitch: s.cfg.WALUploadForceSwitch,
	}
	if s.cfg.WALUploadForceSwitch {
		if _, err := s.scalar(ctx, "select pg_switch_wal()"); err != nil {
			result.Error = "pg_switch_wal failed: " + err.Error()
			result.FinishedAt = time.Now().UTC()
			result.Duration = result.FinishedAt.Sub(result.StartedAt).Round(time.Second).String()
			s.recordWALUpload(result)
			return result
		}
	}
	count, err := s.uploadPendingWAL(ctx, limit)
	if err != nil {
		result.Error = err.Error()
	}
	result.Uploaded = count
	result.ActiveBackupID, result.WALPrefix = s.activeGenerationRef()
	result.FinishedAt = time.Now().UTC()
	result.Duration = result.FinishedAt.Sub(result.StartedAt).Round(time.Second).String()
	s.recordWALUpload(result)
	return result
}

func (s *server) recordWALUpload(result *walUploadResult) {
	s.mu.Lock()
	s.lastWALUpload = result
	s.mu.Unlock()
}

func (s *server) runBackup(ctx context.Context) (*backupResult, error) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil, errors.New("backup already running")
	}
	s.running = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	started := time.Now().UTC()
	id := "backup-" + started.Format("20060102-150405")
	workDir := filepath.Join(s.cfg.BackupDir, id+"-work")
	zipPath := filepath.Join(s.cfg.BackupDir, id+".zip")
	if err := os.RemoveAll(workDir); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return nil, err
	}
	defer os.RemoveAll(workDir)

	lsnStart, _ := s.scalar(ctx, "select pg_current_wal_lsn()")
	if _, err := s.scalar(ctx, "select pg_switch_wal()"); err != nil {
		return nil, fmt.Errorf("pre-backup pg_switch_wal failed: %w", err)
	}

	args := []string{
		"-h", s.cfg.PostgresHost,
		"-p", s.cfg.PostgresPort,
		"-U", "postgres",
		"-D", workDir,
		"-Ft",
		"-z",
		"-X", "stream",
		"-P",
	}
	out, err := runCommand(ctx, map[string]string{"PGPASSWORD": s.cfg.PostgresSuperPass}, "pg_basebackup", args...)
	if err != nil {
		return nil, fmt.Errorf("pg_basebackup failed: %w\n%s", err, out)
	}

	logicalDumpRel := filepath.ToSlash(filepath.Join("logical", s.cfg.PostgresDB+".sql"))
	logicalDumpPath := filepath.Join(workDir, filepath.FromSlash(logicalDumpRel))
	if err := os.MkdirAll(filepath.Dir(logicalDumpPath), 0755); err != nil {
		return nil, err
	}
	dumpOut, err := runCommand(ctx, map[string]string{"PGPASSWORD": s.cfg.PostgresSuperPass}, "pg_dump",
		"-h", s.cfg.PostgresHost,
		"-p", s.cfg.PostgresPort,
		"-U", "postgres",
		"-d", s.cfg.PostgresDB,
		"--format=plain",
		"--no-owner",
		"--no-privileges",
		"--clean",
		"--if-exists",
		"-f", logicalDumpPath,
	)
	if err != nil {
		return nil, fmt.Errorf("logical pg_dump failed: %w\n%s", err, dumpOut)
	}

	lsnFinish, _ := s.scalar(ctx, "select pg_current_wal_lsn()")
	startWAL := ""
	if lsnFinish != "" {
		startWAL, _ = s.scalar(ctx, "select pg_walfile_name("+sqlLiteral(lsnFinish)+"::pg_lsn)")
	}
	if startWAL == "" {
		startWAL, _ = s.scalar(ctx, "select pg_walfile_name(pg_current_wal_lsn())")
	}
	if _, err := s.scalar(ctx, "select pg_switch_wal()"); err != nil {
		return nil, fmt.Errorf("post-backup pg_switch_wal failed: %w", err)
	}

	manifest := map[string]any{
		"id":              id,
		"started_at":      started,
		"postgres_host":   s.cfg.PostgresHost,
		"postgres_port":   s.cfg.PostgresPort,
		"database":        s.cfg.PostgresDB,
		"logical_dump":    logicalDumpRel,
		"lsn_start":       lsnStart,
		"lsn_finish":      lsnFinish,
		"start_wal":       startWAL,
		"backup_format":   "pg_basebackup tar gzip plus logical SQL dump wrapped in zip",
		"restore_summary": "Use logical dump for database-level restore. Use physical base backup plus this backup generation's WAL archive folder for full cluster PITR.",
	}
	manifestBytes, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(workDir, "backup-manifest.json"), manifestBytes, 0644); err != nil {
		return nil, err
	}
	if err := zipDir(zipPath, workDir); err != nil {
		return nil, err
	}
	info, err := os.Stat(zipPath)
	if err != nil {
		return nil, err
	}

	var s3Key string
	if s.cfg.S3Enabled {
		s3Key = s.s3Key("base/" + filepath.Base(zipPath))
		if err := s.uploadFile(ctx, zipPath, s3Key); err != nil {
			return nil, err
		}
	}

	generation := &backupGeneration{
		ID:             id,
		BackupFile:     zipPath,
		S3BackupKey:    s3Key,
		SourceDatabase: s.cfg.PostgresDB,
		StartedAt:      started,
		FinishedAt:     time.Now().UTC(),
		LSNStart:       lsnStart,
		LSNFinish:      lsnFinish,
		StartWAL:       startWAL,
		WALPrefix:      s.s3Key("wal/" + id + "/"),
		UploadedWAL:    map[string]time.Time{},
		RestoreNote:    "For full PITR, restore this base backup into a separate PostgreSQL instance and replay WAL from this generation folder.",
	}
	if err := s.registerBackupGeneration(generation); err != nil {
		return nil, err
	}

	walUploaded, err := s.uploadPendingWAL(ctx, 0)
	if err != nil {
		return nil, err
	}
	if err := s.enforceRetention(ctx); err != nil {
		return nil, err
	}

	result := &backupResult{
		ID:             id,
		StartedAt:      started,
		FinishedAt:     time.Now().UTC(),
		BackupFile:     zipPath,
		BackupSize:     info.Size(),
		LogicalDump:    logicalDumpRel,
		S3BackupKey:    s3Key,
		WALUploaded:    walUploaded,
		LSNStart:       lsnStart,
		LSNFinish:      lsnFinish,
		Database:       s.cfg.PostgresDB,
		PGBaseBackup:   strings.TrimSpace(out),
		RetentionCount: s.cfg.RetentionCount,
		RestoreNote:    "Use the logical dump for restore into a new database. Use this backup's PITR chain for full cluster point-in-time recovery.",
	}
	result.Duration = result.FinishedAt.Sub(result.StartedAt).Round(time.Second).String()

	s.mu.Lock()
	s.lastRun = result
	s.mu.Unlock()
	return result, nil
}

func (s *server) uploadPendingWAL(ctx context.Context, limit int) (int, error) {
	if !s.cfg.S3Enabled {
		return 0, nil
	}
	s.stateMu.Lock()
	activeID := s.state.ActiveBackupID
	gen := s.state.Generations[activeID]
	s.stateMu.Unlock()
	if activeID == "" || gen == nil {
		return 0, errors.New("no active PITR backup generation; run /v1/backups/run first")
	}

	entries, err := os.ReadDir(s.cfg.WALDir)
	if err != nil {
		return 0, err
	}
	uploaded := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if !isWALArchiveName(name) {
			continue
		}
		if !walAtOrAfter(name, gen.StartWAL) {
			continue
		}
		path := filepath.Join(s.cfg.WALDir, name)
		s.stateMu.Lock()
		_, exists := gen.UploadedWAL[name]
		s.stateMu.Unlock()
		if exists {
			continue
		}
		if err := s.uploadFile(ctx, path, s.s3Key("wal/"+activeID+"/"+name)); err != nil {
			return uploaded, err
		}
		s.stateMu.Lock()
		gen.UploadedWAL[name] = time.Now().UTC()
		s.state.UploadedWAL[name] = time.Now().UTC()
		s.stateMu.Unlock()
		uploaded++
		if limit > 0 && uploaded >= limit {
			break
		}
	}
	return uploaded, s.saveState()
}

func (s *server) enforceRetention(ctx context.Context) error {
	if s.cfg.RetentionCount <= 0 {
		return nil
	}
	local, err := localBackups(s.cfg.BackupDir)
	if err != nil {
		return err
	}
	for len(local) > s.cfg.RetentionCount {
		oldest := local[0]
		if err := os.Remove(oldest.Path); err != nil {
			return err
		}
		local = local[1:]
	}
	if s.cfg.S3Enabled {
		if err := s.enforceS3BackupRetention(ctx); err != nil {
			return err
		}
	}
	return s.enforceGenerationRetention(ctx)
}

func (s *server) enforceS3BackupRetention(ctx context.Context) error {
	prefix := s.s3Key("base/")
	var objects []types.Object
	paginator := s3.NewListObjectsV2Paginator(s.s3, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.cfg.S3Bucket),
		Prefix: aws.String(prefix),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return err
		}
		objects = append(objects, page.Contents...)
	}
	sort.Slice(objects, func(i, j int) bool {
		return aws.ToTime(objects[i].LastModified).Before(aws.ToTime(objects[j].LastModified))
	})
	for len(objects) > s.cfg.RetentionCount {
		obj := objects[0]
		_, err := s.s3.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(s.cfg.S3Bucket),
			Key:    obj.Key,
		})
		if err != nil {
			return err
		}
		objects = objects[1:]
	}
	return nil
}

func (s *server) enforceGenerationRetention(ctx context.Context) error {
	if s.cfg.RetentionCount <= 0 {
		return nil
	}
	s.stateMu.Lock()
	gens := make([]*backupGeneration, 0, len(s.state.Generations))
	for _, gen := range s.state.Generations {
		gens = append(gens, gen)
	}
	sort.Slice(gens, func(i, j int) bool {
		return gens[i].FinishedAt.Before(gens[j].FinishedAt)
	})
	var removed []backupGeneration
	for len(gens) > s.cfg.RetentionCount {
		oldest := gens[0]
		removed = append(removed, *oldest)
		delete(s.state.Generations, oldest.ID)
		if s.state.ActiveBackupID == oldest.ID {
			s.state.ActiveBackupID = ""
		}
		gens = gens[1:]
	}
	if s.state.ActiveBackupID == "" && len(gens) > 0 {
		newest := gens[len(gens)-1]
		s.state.ActiveBackupID = newest.ID
	}
	err := s.saveStateLocked()
	s.stateMu.Unlock()
	if err != nil {
		return err
	}
	if !s.cfg.S3Enabled {
		return nil
	}
	for _, gen := range removed {
		if gen.WALPrefix == "" {
			continue
		}
		if err := s.deleteS3Prefix(ctx, gen.WALPrefix); err != nil {
			return err
		}
	}
	return nil
}

func (s *server) deleteS3Prefix(ctx context.Context, prefix string) error {
	paginator := s3.NewListObjectsV2Paginator(s.s3, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.cfg.S3Bucket),
		Prefix: aws.String(prefix),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return err
		}
		if len(page.Contents) == 0 {
			continue
		}
		objects := make([]types.ObjectIdentifier, 0, len(page.Contents))
		for _, obj := range page.Contents {
			objects = append(objects, types.ObjectIdentifier{Key: obj.Key})
		}
		_, err = s.s3.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(s.cfg.S3Bucket),
			Delete: &types.Delete{Objects: objects},
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *server) uploadFile(ctx context.Context, path, key string) error {
	if !s.cfg.S3Enabled {
		return nil
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = s.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.cfg.S3Bucket),
		Key:    aws.String(key),
		Body:   file,
	})
	return err
}

func newS3Client(ctx context.Context, cfg appConfig) (*s3.Client, error) {
	if cfg.S3Bucket == "" {
		return nil, errors.New("BACKUP_S3_BUCKET is required when BACKUP_S3_ENABLED=true")
	}
	opts := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.S3Region),
	}
	if cfg.S3AccessKeyID != "" || cfg.S3SecretAccessKey != "" {
		opts = append(opts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.S3AccessKeyID, cfg.S3SecretAccessKey, "")))
	}
	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = cfg.S3ForcePathStyle
		if cfg.S3Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.S3Endpoint)
		}
	}), nil
}

func (s *server) scalar(ctx context.Context, sql string) (string, error) {
	conn, err := pgx.Connect(ctx, s.superDSN("postgres"))
	if err != nil {
		return "", err
	}
	defer conn.Close(ctx)
	var value string
	if err := conn.QueryRow(ctx, sql).Scan(&value); err != nil {
		return "", err
	}
	return value, nil
}

func (s *server) superDSN(db string) string {
	return fmt.Sprintf("postgres://postgres:%s@%s:%s/%s?sslmode=disable",
		s.cfg.PostgresSuperPass, s.cfg.PostgresHost, s.cfg.PostgresPort, db)
}

func (s *server) s3Key(suffix string) string {
	if s.cfg.S3Prefix == "" {
		return suffix
	}
	return s.cfg.S3Prefix + "/" + strings.TrimLeft(suffix, "/")
}

func (s *server) loadState() error {
	bytes, err := os.ReadFile(s.stateLoc)
	if errors.Is(err, os.ErrNotExist) {
		s.ensureStateInitialized()
		return nil
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(bytes, &s.state); err != nil {
		return err
	}
	s.ensureStateInitialized()
	return nil
}

func (s *server) ensureStateInitialized() {
	if s.state.UploadedWAL == nil {
		s.state.UploadedWAL = map[string]time.Time{}
	}
	if s.state.Generations == nil {
		s.state.Generations = map[string]*backupGeneration{}
	}
	for _, gen := range s.state.Generations {
		if gen.UploadedWAL == nil {
			gen.UploadedWAL = map[string]time.Time{}
		}
	}
}

func (s *server) saveState() error {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	return s.saveStateLocked()
}

func (s *server) saveStateLocked() error {
	bytes, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.stateLoc, bytes, 0644)
}

func (s *server) registerBackupGeneration(gen *backupGeneration) error {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	s.ensureStateInitialized()
	s.state.Generations[gen.ID] = gen
	s.state.ActiveBackupID = gen.ID
	return s.saveStateLocked()
}

func (s *server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-Backup-API-Key")
		if subtle.ConstantTimeCompare([]byte(key), []byte(s.cfg.APIKey)) != 1 {
			writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "missing or invalid X-Backup-API-Key"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "service": "postgres-wal-backup-api"})
}

func (s *server) status(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	running := s.running
	last := s.lastRun
	lastWALUpload := s.lastWALUpload
	s.mu.Unlock()
	s.stateMu.Lock()
	activeBackupID := s.state.ActiveBackupID
	s.stateMu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"running":                 running,
		"started_at":              s.started,
		"schedule_enabled":        s.cfg.ScheduleEnabled,
		"daily_time":              s.cfg.DailyTime,
		"timezone":                s.cfg.Timezone,
		"retention_count":         s.cfg.RetentionCount,
		"wal_upload_enabled":      s.cfg.WALUploadEnabled,
		"wal_upload_interval":     s.cfg.WALUploadInterval.String(),
		"wal_upload_batch_size":   s.cfg.WALUploadBatchSize,
		"wal_upload_force_switch": s.cfg.WALUploadForceSwitch,
		"s3_enabled":              s.cfg.S3Enabled,
		"s3_bucket":               s.cfg.S3Bucket,
		"s3_prefix":               s.cfg.S3Prefix,
		"backup_dir":              s.cfg.BackupDir,
		"wal_dir":                 s.cfg.WALDir,
		"last_backup_result":      last,
		"last_wal_upload":         lastWALUpload,
		"active_backup_id":        activeBackupID,
		"restore_requirement":     "base backup ZIP + WAL archive files",
	})
}

func (s *server) databaseStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, s.superDSN("postgres"))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: err.Error()})
		return
	}
	defer conn.Close(ctx)
	var total, active int
	var archived, failed int64
	var lastArchived *string
	if err := conn.QueryRow(ctx, "select count(*), count(*) filter (where state = 'active') from pg_stat_activity").Scan(&total, &active); err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: err.Error()})
		return
	}
	_ = conn.QueryRow(ctx, "select archived_count, failed_count, last_archived_wal from pg_stat_archiver").Scan(&archived, &failed, &lastArchived)
	writeJSON(w, http.StatusOK, map[string]any{
		"total_connections":  total,
		"active_connections": active,
		"archived_wal_count": archived,
		"failed_wal_count":   failed,
		"last_archived_wal":  lastArchived,
	})
}

func (s *server) listBackups(w http.ResponseWriter, _ *http.Request) {
	items, err := localBackups(s.cfg.BackupDir)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"backups": items})
}

func (s *server) runBackupHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.BackupTimeout)
	defer cancel()
	result, err := s.runBackup(ctx)
	if err != nil {
		writeJSON(w, http.StatusConflict, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *server) restoreOptions(w http.ResponseWriter, _ *http.Request) {
	options, err := s.localRestoreOptions()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"restore_options": options,
		"restore_note":    "Database-level restore requires a backup ZIP containing a logical SQL dump. Older physical-only backup ZIPs cannot be restored into a new database by this endpoint.",
	})
}

func (s *server) restoreRunHTTP(w http.ResponseWriter, r *http.Request) {
	var req restoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body: " + err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.BackupTimeout)
	defer cancel()
	result, err := s.restoreDatabase(ctx, req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *server) walStatus(w http.ResponseWriter, _ *http.Request) {
	files, err := walFiles(s.cfg.WALDir)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	s.stateMu.Lock()
	activeID := s.state.ActiveBackupID
	gen := s.state.Generations[activeID]
	uploaded := 0
	legacyUploaded := len(s.state.UploadedWAL)
	startWAL := ""
	walPrefix := ""
	if gen != nil {
		uploaded = len(gen.UploadedWAL)
		startWAL = gen.StartWAL
		walPrefix = gen.WALPrefix
	}
	s.stateMu.Unlock()
	s.mu.Lock()
	lastWALUpload := s.lastWALUpload
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"wal_dir":                       s.cfg.WALDir,
		"wal_files_local":               len(files),
		"active_backup_id":              activeID,
		"active_start_wal":              startWAL,
		"active_wal_prefix":             walPrefix,
		"active_wal_uploaded":           uploaded,
		"legacy_uploaded_known":         legacyUploaded,
		"automatic_upload_enabled":      s.cfg.WALUploadEnabled,
		"automatic_upload_interval":     s.cfg.WALUploadInterval.String(),
		"automatic_upload_batch_size":   s.cfg.WALUploadBatchSize,
		"automatic_upload_force_switch": s.cfg.WALUploadForceSwitch,
		"last_wal_upload":               lastWALUpload,
		"s3_enabled":                    s.cfg.S3Enabled,
		"note":                          "WAL upload is organized by PITR generation. Run a base backup first, then /v1/wal/upload sends later WAL to that backup's folder.",
	})
}

func (s *server) uploadWALHTTP(w http.ResponseWriter, r *http.Request) {
	result := s.runWALUpload(r.Context(), false, 0)
	if result.Error != "" {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: result.Error})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *server) s3Status(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.S3Enabled {
		writeJSON(w, http.StatusOK, map[string]any{
			"s3_enabled": false,
			"message":    "S3 upload is disabled",
		})
		return
	}
	base, err := s.listS3Objects(r.Context(), s.s3Key("base/"), 20)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: err.Error()})
		return
	}
	wal, err := s.listS3Objects(r.Context(), s.s3Key("wal/"), 20)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: err.Error()})
		return
	}
	activeWAL := []s3ObjectInfo{}
	s.stateMu.Lock()
	activeID := s.state.ActiveBackupID
	activePrefix := ""
	if gen := s.state.Generations[activeID]; gen != nil {
		activePrefix = gen.WALPrefix
	}
	s.stateMu.Unlock()
	if activePrefix != "" {
		activeWAL, err = s.listS3Objects(r.Context(), activePrefix, 20)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, errorResponse{Error: err.Error()})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"s3_enabled":         true,
		"bucket":             s.cfg.S3Bucket,
		"prefix":             s.cfg.S3Prefix,
		"base_prefix":        s.s3Key("base/"),
		"wal_prefix":         s.s3Key("wal/"),
		"active_backup_id":   activeID,
		"active_wal_prefix":  activePrefix,
		"pitr_chains":        s.pitrChainList(),
		"base_objects":       base,
		"wal_objects":        wal,
		"active_wal_objects": activeWAL,
		"console_hint":       "Open MinIO console, bucket postgres-wal, then browse app-backup/base and app-backup/wal/<backup-id>.",
		"endpoint_used":      s.cfg.S3Endpoint,
		"retention_note":     "Backup ZIP retention is enforced on base objects. PITR WAL folders are pruned only when their backup generation leaves retention.",
	})
}

func (s *server) pitrChainsHTTP(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"chains": s.pitrChainList(),
		"note":   "Each chain is one base backup plus the WAL folder used to replay changes after that backup.",
	})
}

func (s *server) pitrDownloadHTTP(w http.ResponseWriter, r *http.Request) {
	var req pitrDownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body: " + err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.BackupTimeout)
	defer cancel()
	result, err := s.downloadPITRChain(ctx, req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *server) swaggerUI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, swaggerHTML)
}

func (s *server) openapi(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, openAPIJSON)
}

func (s *server) localRestoreOptions() ([]restoreOption, error) {
	backups, err := localBackups(s.cfg.BackupDir)
	if err != nil {
		return nil, err
	}
	options := make([]restoreOption, 0, len(backups))
	for _, item := range backups {
		dump, sourceDB, err := logicalDumpInZip(item.Path)
		canRestore := err == nil && dump != ""
		note := "Can restore into a new database using logical SQL dump."
		if !canRestore {
			note = "Physical base backup only. Use this for full-cluster PITR, not database-level restore."
		}
		options = append(options, restoreOption{
			Name:                  item.Name,
			Path:                  item.Path,
			Size:                  item.Size,
			Modified:              item.Modified,
			SourceDatabase:        sourceDB,
			LogicalDump:           dump,
			CanRestoreToDatabase:  canRestore,
			RestoreKind:           "database",
			PhysicalPITRSupported: true,
			Note:                  note,
		})
	}
	sort.Slice(options, func(i, j int) bool {
		return options[i].Modified.After(options[j].Modified)
	})
	return options, nil
}

func (s *server) restoreDatabase(ctx context.Context, req restoreRequest) (*restoreResult, error) {
	req.BackupName = filepath.Base(strings.TrimSpace(req.BackupName))
	req.TargetDatabase = strings.TrimSpace(req.TargetDatabase)
	if req.BackupName == "" {
		return nil, errors.New("backup_name is required")
	}
	if !validDatabaseName(req.TargetDatabase) {
		return nil, errors.New("target_database is required and must contain only letters, numbers, and underscores")
	}
	if strings.EqualFold(req.TargetDatabase, s.cfg.PostgresDB) && !req.DropIfExists {
		return nil, errors.New("refusing to restore over source database without drop_if_exists=true")
	}
	backupPath := filepath.Join(s.cfg.BackupDir, req.BackupName)
	if !strings.HasPrefix(filepath.Clean(backupPath), filepath.Clean(s.cfg.BackupDir)) {
		return nil, errors.New("invalid backup_name")
	}
	if _, err := os.Stat(backupPath); err != nil {
		return nil, err
	}

	dumpName, _, err := logicalDumpInZip(backupPath)
	if err != nil {
		return nil, err
	}
	if dumpName == "" {
		return nil, errors.New("backup does not contain a logical SQL dump; create a new backup with the updated API first")
	}

	started := time.Now().UTC()
	tempDir, err := os.MkdirTemp(s.cfg.BackupDir, "restore-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)
	dumpPath := filepath.Join(tempDir, "restore.sql")
	if err := extractZipFile(backupPath, dumpName, dumpPath); err != nil {
		return nil, err
	}

	if req.DropIfExists {
		if _, err := s.adminCommand(ctx, fmt.Sprintf("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = %s", sqlLiteral(req.TargetDatabase))); err != nil {
			return nil, fmt.Errorf("terminate target connections failed: %w", err)
		}
		if _, err := s.adminCommand(ctx, "DROP DATABASE IF EXISTS "+quoteIdent(req.TargetDatabase)); err != nil {
			return nil, fmt.Errorf("drop target database failed: %w", err)
		}
	}
	if _, err := s.adminCommand(ctx, "CREATE DATABASE "+quoteIdent(req.TargetDatabase)+" OWNER "+quoteIdent(s.cfg.PostgresUser)); err != nil {
		return nil, fmt.Errorf("create target database failed: %w", err)
	}

	out, err := runCommand(ctx, map[string]string{"PGPASSWORD": s.cfg.PostgresPassword}, "psql",
		"-v", "ON_ERROR_STOP=1",
		"-h", s.cfg.PostgresHost,
		"-p", s.cfg.PostgresPort,
		"-U", s.cfg.PostgresUser,
		"-d", req.TargetDatabase,
		"-f", dumpPath,
	)
	if err != nil {
		return nil, fmt.Errorf("restore SQL failed: %w\n%s", err, out)
	}

	rowsChecked := 0
	conn, err := pgx.Connect(ctx, s.appDSN(req.TargetDatabase))
	if err == nil {
		defer conn.Close(ctx)
		_ = conn.QueryRow(ctx, "select count(*) from information_schema.tables where table_schema not in ('pg_catalog', 'information_schema')").Scan(&rowsChecked)
	}

	finished := time.Now().UTC()
	return &restoreResult{
		BackupName:     req.BackupName,
		TargetDatabase: req.TargetDatabase,
		StartedAt:      started,
		FinishedAt:     finished,
		Duration:       finished.Sub(started).Round(time.Second).String(),
		RowsChecked:    rowsChecked,
		Message:        "Restore completed. Point PHPMaker to the target database to test it.",
	}, nil
}

func (s *server) adminCommand(ctx context.Context, sql string) (string, error) {
	return runCommand(ctx, map[string]string{"PGPASSWORD": s.cfg.PostgresSuperPass}, "psql",
		"-v", "ON_ERROR_STOP=1",
		"-h", s.cfg.PostgresHost,
		"-p", s.cfg.PostgresPort,
		"-U", "postgres",
		"-d", "postgres",
		"-c", sql,
	)
}

func (s *server) appDSN(db string) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		s.cfg.PostgresUser, s.cfg.PostgresPassword, s.cfg.PostgresHost, s.cfg.PostgresPort, db)
}

type backupInfo struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	Modified  time.Time `json:"modified"`
	RestoreBy string    `json:"restore_by"`
}

type s3ObjectInfo struct {
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
}

func (s *server) listS3Objects(ctx context.Context, prefix string, limit int) ([]s3ObjectInfo, error) {
	if s.s3 == nil {
		return nil, errors.New("S3 client is not configured")
	}
	out, err := s.s3.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(s.cfg.S3Bucket),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int32(int32(limit)),
	})
	if err != nil {
		return nil, err
	}
	objects := make([]s3ObjectInfo, 0, len(out.Contents))
	for _, obj := range out.Contents {
		objects = append(objects, s3ObjectInfo{
			Key:          aws.ToString(obj.Key),
			Size:         aws.ToInt64(obj.Size),
			LastModified: aws.ToTime(obj.LastModified),
		})
	}
	sort.Slice(objects, func(i, j int) bool {
		return objects[i].LastModified.After(objects[j].LastModified)
	})
	return objects, nil
}

func (s *server) listAllS3Objects(ctx context.Context, prefix string) ([]s3ObjectInfo, error) {
	if s.s3 == nil {
		return nil, errors.New("S3 client is not configured")
	}
	var objects []s3ObjectInfo
	paginator := s3.NewListObjectsV2Paginator(s.s3, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.cfg.S3Bucket),
		Prefix: aws.String(prefix),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, obj := range page.Contents {
			objects = append(objects, s3ObjectInfo{
				Key:          aws.ToString(obj.Key),
				Size:         aws.ToInt64(obj.Size),
				LastModified: aws.ToTime(obj.LastModified),
			})
		}
	}
	sort.Slice(objects, func(i, j int) bool {
		return objects[i].Key < objects[j].Key
	})
	return objects, nil
}

func (s *server) downloadPITRChain(ctx context.Context, req pitrDownloadRequest) (*pitrDownloadResult, error) {
	if !s.cfg.S3Enabled || s.s3 == nil {
		return nil, errors.New("S3 is not enabled")
	}
	started := time.Now().UTC()
	gen, err := s.selectGeneration(req.ChainID, req.Latest)
	if err != nil {
		return nil, err
	}
	if gen.S3BackupKey == "" {
		return nil, errors.New("selected chain has no S3 backup key; create a new S3-enabled backup first")
	}
	if gen.WALPrefix == "" {
		return nil, errors.New("selected chain has no WAL prefix")
	}

	backupName := s3ObjectBase(gen.S3BackupKey)
	if backupName == "" {
		return nil, errors.New("selected chain has an invalid S3 backup key")
	}
	backupPath := filepath.Join(s.cfg.BackupDir, backupName)
	if err := s.downloadS3Object(ctx, gen.S3BackupKey, backupPath, req.Overwrite); err != nil {
		return nil, err
	}

	walDir := filepath.Join(s.cfg.BackupDir, "pitr-downloads", gen.ID, "wal")
	if err := os.MkdirAll(walDir, 0755); err != nil {
		return nil, err
	}
	walObjects, err := s.listAllS3Objects(ctx, gen.WALPrefix)
	if err != nil {
		return nil, err
	}
	downloaded := 0
	for _, obj := range walObjects {
		name := s3ObjectBase(obj.Key)
		if name == "" || !isWALArchiveName(name) {
			continue
		}
		if err := s.downloadS3Object(ctx, obj.Key, filepath.Join(walDir, name), req.Overwrite); err != nil {
			return nil, err
		}
		downloaded++
	}
	finished := time.Now().UTC()
	return &pitrDownloadResult{
		ChainID:        gen.ID,
		BackupName:     backupName,
		BackupPath:     backupPath,
		S3BackupKey:    gen.S3BackupKey,
		WALPrefix:      gen.WALPrefix,
		WALDownloadDir: walDir,
		WALDownloaded:  downloaded,
		StartedAt:      started,
		FinishedAt:     finished,
		Duration:       finished.Sub(started).Round(time.Second).String(),
		NextStep:       "Run restore-lab/prepare-pitr-restore.ps1 with this backup_name and use backups/pitr-downloads/<chain_id>/wal as -WalSource.",
	}, nil
}

func (s *server) downloadS3Object(ctx context.Context, key, outPath string, overwrite bool) error {
	if !overwrite {
		if _, err := os.Stat(outPath); err == nil {
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return err
	}
	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer out.Close()
	obj, err := s.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.cfg.S3Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return err
	}
	defer obj.Body.Close()
	_, err = io.Copy(out, obj.Body)
	return err
}

func (s *server) pitrChainList() []pitrChain {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	chains := make([]pitrChain, 0, len(s.state.Generations))
	for _, gen := range s.state.Generations {
		chains = append(chains, pitrChain{
			ID:               gen.ID,
			BackupFile:       gen.BackupFile,
			S3BackupKey:      gen.S3BackupKey,
			SourceDatabase:   gen.SourceDatabase,
			StartedAt:        gen.StartedAt,
			FinishedAt:       gen.FinishedAt,
			LSNStart:         gen.LSNStart,
			LSNFinish:        gen.LSNFinish,
			StartWAL:         gen.StartWAL,
			WALPrefix:        gen.WALPrefix,
			UploadedWALCount: len(gen.UploadedWAL),
			Active:           gen.ID == s.state.ActiveBackupID,
			RestoreUse:       "Use backup_file or s3_backup_key as the base backup, then replay WAL from wal_prefix in a separate restore PostgreSQL instance.",
		})
	}
	sort.Slice(chains, func(i, j int) bool {
		return chains[i].FinishedAt.After(chains[j].FinishedAt)
	})
	return chains
}

func (s *server) selectGeneration(chainID string, latest bool) (*backupGeneration, error) {
	chainID = strings.TrimSpace(chainID)
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if chainID != "" && !latest {
		gen := s.state.Generations[chainID]
		if gen == nil {
			return nil, fmt.Errorf("PITR chain %s not found", chainID)
		}
		copy := *gen
		return &copy, nil
	}
	var selected *backupGeneration
	for _, gen := range s.state.Generations {
		if selected == nil || gen.FinishedAt.After(selected.FinishedAt) {
			copy := *gen
			selected = &copy
		}
	}
	if selected == nil {
		return nil, errors.New("no PITR chains available; run /v1/backups/run first")
	}
	return selected, nil
}

func (s *server) activeGenerationRef() (string, string) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	activeID := s.state.ActiveBackupID
	walPrefix := ""
	if gen := s.state.Generations[activeID]; gen != nil {
		walPrefix = gen.WALPrefix
	}
	return activeID, walPrefix
}

func s3ObjectBase(key string) string {
	key = strings.TrimRight(key, "/")
	if key == "" {
		return ""
	}
	idx := strings.LastIndex(key, "/")
	if idx >= 0 {
		return key[idx+1:]
	}
	return key
}

func localBackups(dir string) ([]backupInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var items []backupInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "backup-") || !strings.HasSuffix(entry.Name(), ".zip") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		path := filepath.Join(dir, entry.Name())
		items = append(items, backupInfo{
			Name:      entry.Name(),
			Path:      path,
			Size:      info.Size(),
			Modified:  info.ModTime(),
			RestoreBy: "unzip physical backup, restore base files, replay WAL archive with restore_command",
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Modified.Before(items[j].Modified)
	})
	return items, nil
}

func walFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if isWALArchiveName(entry.Name()) {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)
	return files, nil
}

func logicalDumpInZip(zipPath string) (string, string, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", "", err
	}
	defer reader.Close()
	var dump string
	var sourceDB string
	for _, file := range reader.File {
		name := filepath.ToSlash(file.Name)
		if strings.HasPrefix(name, "logical/") && strings.HasSuffix(name, ".sql") {
			dump = name
			sourceDB = strings.TrimSuffix(strings.TrimPrefix(name, "logical/"), ".sql")
			break
		}
	}
	return dump, sourceDB, nil
}

func extractZipFile(zipPath, zipName, outPath string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()
	for _, file := range reader.File {
		if filepath.ToSlash(file.Name) != filepath.ToSlash(zipName) {
			continue
		}
		in, err := file.Open()
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.Create(outPath)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	}
	return fmt.Errorf("file %s not found in %s", zipName, zipPath)
}

func isWALArchiveName(name string) bool {
	if strings.HasPrefix(name, ".") {
		return false
	}
	if strings.HasSuffix(name, ".backup") {
		return true
	}
	if len(name) != 24 {
		return false
	}
	for _, r := range name {
		if !((r >= '0' && r <= '9') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

func walAtOrAfter(name, start string) bool {
	if start == "" {
		return true
	}
	comparable := walComparableName(name)
	if len(comparable) != 24 {
		return true
	}
	return comparable >= start
}

func walComparableName(name string) string {
	if strings.HasSuffix(name, ".backup") {
		return strings.Split(name, ".")[0]
	}
	return name
}

func validDatabaseName(name string) bool {
	if name == "" || len(name) > 63 {
		return false
	}
	for i, r := range name {
		valid := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' || (i > 0 && r >= '0' && r <= '9')
		if !valid {
			return false
		}
	}
	return true
}

func quoteIdent(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func sqlLiteral(value string) string {
	return `'` + strings.ReplaceAll(value, `'`, `''`) + `'`
}

func zipDir(zipPath, sourceDir string) error {
	out, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer out.Close()

	zw := zip.NewWriter(out)
	defer zw.Close()

	return filepath.WalkDir(sourceDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = rel
		header.Method = zip.Deflate
		writer, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		_, err = io.Copy(writer, in)
		return err
	})
}

func runCommand(ctx context.Context, env map[string]string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.String(), err
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value := strings.ToLower(env(key, ""))
	if value == "" {
		return fallback
	}
	return value == "true" || value == "1" || value == "yes" || value == "on"
}

func envInt(key string, fallback int) int {
	value := env(key, "")
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func cleanPrefix(prefix string) string {
	return strings.Trim(strings.TrimSpace(prefix), "/")
}

const swaggerHTML = `<!doctype html>
<html>
<head>
  <title>PostgreSQL WAL Backup API</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.ui = SwaggerUIBundle({
      url: "/openapi.json",
      dom_id: "#swagger-ui",
      presets: [SwaggerUIBundle.presets.apis],
      layout: "BaseLayout"
    });
  </script>
</body>
</html>`

const openAPIJSON = `{
  "openapi": "3.0.3",
  "info": {
    "title": "PostgreSQL WAL Backup API",
    "version": "1.0.0",
    "description": "Scheduled physical backups, WAL upload, backup retention, and database status for the postgres-wal stack."
  },
  "servers": [{"url": "/"}],
  "components": {
    "securitySchemes": {
      "BackupApiKey": {
        "type": "apiKey",
        "in": "header",
        "name": "X-Backup-API-Key"
      }
    }
  },
  "security": [{"BackupApiKey": []}],
  "paths": {
    "/health": {
      "get": {
        "security": [],
        "summary": "Service health",
        "responses": {"200": {"description": "OK"}}
      }
    },
    "/v1/status": {
      "get": {
        "summary": "Backup service status",
        "responses": {"200": {"description": "Status"}}
      }
    },
    "/v1/database/status": {
      "get": {
        "summary": "Database connection and WAL archiver status",
        "responses": {"200": {"description": "Database status"}}
      }
    },
    "/v1/backups": {
      "get": {
        "summary": "List local backup ZIP files",
        "responses": {"200": {"description": "Backup list"}}
      }
    },
    "/v1/backups/run": {
      "post": {
        "summary": "Run backup now",
        "responses": {
          "200": {"description": "Backup result"},
          "409": {"description": "Backup already running or failed"}
        }
      }
    },
    "/v1/restores/options": {
      "get": {
        "summary": "List local backups that can be restored into a database",
        "responses": {"200": {"description": "Restore options"}}
      }
    },
    "/v1/restores/run": {
      "post": {
        "summary": "Restore a backup logical dump into a target database",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["backup_name", "target_database"],
                "properties": {
                  "backup_name": {"type": "string", "example": "backup-20260528-071500.zip"},
                  "target_database": {"type": "string", "example": "appdb_restore_test"},
                  "drop_if_exists": {"type": "boolean", "example": true}
                }
              }
            }
          }
        },
        "responses": {
          "200": {"description": "Restore result"},
          "400": {"description": "Invalid restore request"}
        }
      }
    },
    "/v1/s3/status": {
      "get": {
        "summary": "List recent S3 backup and WAL objects",
        "responses": {
          "200": {"description": "S3 object status"},
          "502": {"description": "S3 list failed"}
        }
      }
    },
    "/v1/pitr/chains": {
      "get": {
        "summary": "List PITR restore chains",
        "responses": {"200": {"description": "Base backup and WAL folder chains"}}
      }
    },
    "/v1/pitr/download": {
      "post": {
        "summary": "Download a PITR chain from S3 into local restore staging folders",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": {
                  "chain_id": {"type": "string", "example": "backup-20260528-180000"},
                  "latest": {"type": "boolean", "example": true},
                  "overwrite": {"type": "boolean", "example": true}
                }
              }
            }
          }
        },
        "responses": {
          "200": {"description": "Downloaded PITR chain"},
          "400": {"description": "Invalid request or download failed"}
        }
      }
    },
    "/v1/wal/status": {
      "get": {
        "summary": "WAL archive and active PITR generation status",
        "responses": {"200": {"description": "WAL status"}}
      }
    },
    "/v1/wal/upload": {
      "post": {
        "summary": "Upload pending WAL files to the active PITR generation folder in S3",
        "responses": {"200": {"description": "Upload result"}}
      }
    }
  }
}`
