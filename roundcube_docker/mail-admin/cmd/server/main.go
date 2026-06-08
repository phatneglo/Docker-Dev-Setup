package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed web/dist/*
var embeddedUI embed.FS

type Config struct {
	Port                string
	AdminEmail          string
	AdminPassword       string
	MailDomain          string
	MailserverContainer string
	MailserverConfigDir string
	SessionSecure       bool
}

type Server struct {
	cfg      Config
	runner   CommandRunner
	sessions *SessionStore
}

type CommandRunner interface {
	RunSetup(ctx context.Context, args ...string) (string, error)
	MailserverStatus(ctx context.Context) bool
}

type DockerRunner struct {
	Container string
}

type Session struct {
	Email     string
	CSRFToken string
	ExpiresAt time.Time
}

type SessionStore struct {
	mu       sync.Mutex
	sessions map[string]Session
}

type MailUser struct {
	Email       string `json:"email"`
	Used        string `json:"used"`
	Quota       string `json:"quota"`
	QuotaSet    bool   `json:"quotaSet"`
	PercentUsed int    `json:"percentUsed"`
	Status      string `json:"status"`
}

var quotaPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)(B|k|M|G|T)$`)
var emailListPattern = regexp.MustCompile(`^\*\s+(.+?)\s+\(\s*(.*?)\s*/\s*(.*?)\s*\)\s+\[([0-9]+)%\]`)

func main() {
	cfg := loadConfig()
	server := NewServer(cfg, DockerRunner{Container: cfg.MailserverContainer})

	log.Printf("PNP Mail Admin listening on :%s for domain=%s container=%s", cfg.Port, cfg.MailDomain, cfg.MailserverContainer)
	if err := http.ListenAndServe(":"+cfg.Port, server.routes()); err != nil {
		log.Fatal(err)
	}
}

func loadConfig() Config {
	return Config{
		Port:                getEnv("MAIL_ADMIN_UI_PORT", "8092"),
		AdminEmail:          getEnv("MAIL_ADMIN_UI_EMAIL", "admin@itbsstudio.com"),
		AdminPassword:       getEnv("MAIL_ADMIN_UI_PASSWORD", "ChangeMeNow123!"),
		MailDomain:          getEnv("MAIL_DOMAIN", "itbsstudio.com"),
		MailserverContainer: getEnv("MAILSERVER_CONTAINER", "itbs-pnp-mail-mailserver"),
		MailserverConfigDir: getEnv("MAILSERVER_CONFIG_DIR", "/mailserver-config"),
		SessionSecure:       getEnv("MAIL_ADMIN_COOKIE_SECURE", "") == "true",
	}
}

func NewServer(cfg Config, runner CommandRunner) *Server {
	return &Server{
		cfg:      cfg,
		runner:   runner,
		sessions: NewSessionStore(),
	}
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/login", s.handleLogin)
	mux.HandleFunc("POST /api/logout", s.requireAuth(s.handleLogout, true))
	mux.HandleFunc("GET /api/me", s.requireAuth(s.handleMe, false))
	mux.HandleFunc("GET /api/users", s.requireAuth(s.handleListUsers, false))
	mux.HandleFunc("POST /api/users", s.requireAuth(s.handleCreateUser, true))
	mux.HandleFunc("PATCH /api/users/", s.requireAuth(s.handleUserPatch, true))
	mux.HandleFunc("DELETE /api/users/", s.requireAuth(s.handleUserDelete, true))
	mux.HandleFunc("PUT /api/users/", s.requireAuth(s.handleQuotaPut, true))
	mux.HandleFunc("GET /api/healthz", s.handleHealth)
	mux.HandleFunc("/", s.handleStatic)
	return withSecurityHeaders(mux)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if subtle.ConstantTimeCompare([]byte(req.Email), []byte(s.cfg.AdminEmail)) != 1 ||
		subtle.ConstantTimeCompare([]byte(req.Password), []byte(s.cfg.AdminPassword)) != 1 {
		writeError(w, http.StatusUnauthorized, "invalid login")
		return
	}

	id, session := s.sessions.Create(req.Email)
	s.setSessionCookie(w, id, session.ExpiresAt)
	writeJSON(w, http.StatusOK, map[string]any{
		"email":     session.Email,
		"csrfToken": session.CSRFToken,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("mail_admin_session"); err == nil {
		s.sessions.Delete(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "mail_admin_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.cfg.SessionSecure,
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value(sessionKey{}).(Session)
	writeJSON(w, http.StatusOK, map[string]any{
		"email":     session.Email,
		"csrfToken": session.CSRFToken,
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"mailserver": s.runner.MailserverStatus(r.Context()),
	})
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.listUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	quotaEnabled := 0
	for _, user := range users {
		if user.QuotaSet {
			quotaEnabled++
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"users": users,
		"stats": map[string]any{
			"totalUsers":      len(users),
			"quotaUsers":      quotaEnabled,
			"mailserverReady": s.runner.MailserverStatus(r.Context()),
		},
	})
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Quota    string `json:"quota"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := s.validateEmail(req.Email); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.Password) == "" {
		writeError(w, http.StatusBadRequest, "password is required")
		return
	}
	if err := validateQuota(req.Quota, true); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if _, err := s.runner.RunSetup(r.Context(), "email", "add", strings.TrimSpace(req.Email), req.Password); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if strings.TrimSpace(req.Quota) != "" {
		if _, err := s.runner.RunSetup(r.Context(), "quota", "set", strings.TrimSpace(req.Email), strings.TrimSpace(req.Quota)); err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func (s *Server) handleUserPatch(w http.ResponseWriter, r *http.Request) {
	email, action, ok := splitUserAction(r.URL.Path)
	if !ok || action != "password" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err := s.validateEmail(email); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.Password) == "" {
		writeError(w, http.StatusBadRequest, "password is required")
		return
	}
	if _, err := s.runner.RunSetup(r.Context(), "email", "update", email, req.Password); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleUserDelete(w http.ResponseWriter, r *http.Request) {
	email, action, ok := splitUserAction(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err := s.validateEmail(email); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if action == "quota" {
		if _, err := s.runner.RunSetup(r.Context(), "quota", "del", email); err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}

	var req struct {
		ConfirmEmail string `json:"confirmEmail"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.ConfirmEmail != email {
		writeError(w, http.StatusBadRequest, "confirmation email does not match")
		return
	}
	if _, err := s.runner.RunSetup(r.Context(), "email", "del", email); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleQuotaPut(w http.ResponseWriter, r *http.Request) {
	email, action, ok := splitUserAction(r.URL.Path)
	if !ok || action != "quota" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err := s.validateEmail(email); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var req struct {
		Quota string `json:"quota"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := validateQuota(req.Quota, false); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := s.runner.RunSetup(r.Context(), "quota", "set", email, strings.TrimSpace(req.Quota)); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	dist, err := fs.Sub(embeddedUI, "web/dist")
	if err != nil {
		http.Error(w, "admin UI not built", http.StatusInternalServerError)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	if _, err := fs.Stat(dist, path); err != nil {
		path = "index.html"
	}
	http.ServeFileFS(w, r, dist, path)
}

func (s *Server) listUsers(ctx context.Context) ([]MailUser, error) {
	out, err := s.runner.RunSetup(ctx, "email", "list")
	if err != nil {
		return nil, err
	}
	users := ParseEmailList(out)
	quotas := ReadQuotaFile(s.cfg.MailserverConfigDir)
	accounts := ReadAccountsFile(s.cfg.MailserverConfigDir)
	seen := map[string]bool{}
	for i := range users {
		seen[users[i].Email] = true
		if quota, ok := quotas[users[i].Email]; ok {
			users[i].Quota = quota
			users[i].QuotaSet = quota != "" && quota != "~"
		}
	}
	for _, email := range accounts {
		if seen[email] {
			continue
		}
		quota := quotas[email]
		if quota == "" {
			quota = "~"
		}
		users = append(users, MailUser{
			Email:       email,
			Used:        "0",
			Quota:       quota,
			QuotaSet:    quota != "~",
			PercentUsed: 0,
			Status:      "active",
		})
	}
	return users, nil
}

func (s *Server) validateEmail(value string) error {
	value = strings.TrimSpace(value)
	parsed, err := mail.ParseAddress(value)
	if err != nil || parsed.Address != value {
		return fmt.Errorf("valid email address is required")
	}
	parts := strings.Split(value, "@")
	if len(parts) != 2 || !strings.EqualFold(parts[1], s.cfg.MailDomain) {
		return fmt.Errorf("email must belong to %s", s.cfg.MailDomain)
	}
	return nil
}

func ParseEmailList(output string) []MailUser {
	users := []MailUser{}
	for _, line := range strings.Split(stripANSI(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		match := emailListPattern.FindStringSubmatch(line)
		if len(match) != 5 {
			continue
		}
		percent, _ := strconv.Atoi(match[4])
		quota := strings.TrimSpace(match[3])
		quotaSet := quota != "~"
		status := "active"
		if percent >= 90 && quotaSet {
			status = "near-limit"
		}
		users = append(users, MailUser{
			Email:       strings.TrimSpace(match[1]),
			Used:        strings.TrimSpace(match[2]),
			Quota:       quota,
			QuotaSet:    quotaSet,
			PercentUsed: percent,
			Status:      status,
		})
	}
	return users
}

func ReadQuotaFile(configDir string) map[string]string {
	quotas := map[string]string{}
	raw, err := os.ReadFile(configDir + "/dovecot-quotas.cf")
	if err != nil {
		return quotas
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		email := strings.TrimSpace(parts[0])
		quota := strings.TrimSpace(parts[1])
		if email != "" && quota != "" {
			quotas[email] = quota
		}
	}
	return quotas
}

func ReadAccountsFile(configDir string) []string {
	raw, err := os.ReadFile(configDir + "/postfix-accounts.cf")
	if err != nil {
		return nil
	}
	accounts := []string{}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "|") {
			continue
		}
		email := strings.TrimSpace(strings.SplitN(line, "|", 2)[0])
		if email != "" {
			accounts = append(accounts, email)
		}
	}
	return accounts
}

func validateQuota(value string, allowEmpty bool) error {
	value = strings.TrimSpace(value)
	if value == "" && allowEmpty {
		return nil
	}
	if !quotaPattern.MatchString(value) {
		return fmt.Errorf("quota must be like 500M, 1G, 5G, or 0B")
	}
	return nil
}

func (r DockerRunner) RunSetup(ctx context.Context, args ...string) (string, error) {
	if !allowedSetupArgs(args) {
		return "", fmt.Errorf("command is not allowed")
	}
	cmdArgs := append([]string{"exec", r.Container, "setup"}, args...)
	cmd := exec.CommandContext(ctx, "docker", cmdArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), errors.New(strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func (r DockerRunner) MailserverStatus(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "docker", "inspect", "-f", "{{.State.Running}}", r.Container)
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

func allowedSetupArgs(args []string) bool {
	if len(args) == 2 && args[0] == "email" && args[1] == "list" {
		return true
	}
	if len(args) == 4 && args[0] == "email" && (args[1] == "add" || args[1] == "update") {
		return true
	}
	if len(args) == 3 && args[0] == "email" && args[1] == "del" {
		return true
	}
	if len(args) == 4 && args[0] == "quota" && args[1] == "set" {
		return true
	}
	if len(args) == 3 && args[0] == "quota" && args[1] == "del" {
		return true
	}
	return false
}

func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: map[string]Session{}}
}

func (s *SessionStore) Create(email string) (string, Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := randomHex(32)
	session := Session{
		Email:     email,
		CSRFToken: randomHex(32),
		ExpiresAt: time.Now().Add(12 * time.Hour),
	}
	s.sessions[id] = session
	return id, session
}

func (s *SessionStore) Get(id string) (Session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[id]
	if !ok || time.Now().After(session.ExpiresAt) {
		delete(s.sessions, id)
		return Session{}, false
	}
	return session, true
}

func (s *SessionStore) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
}

type sessionKey struct{}

func (s *Server) requireAuth(next http.HandlerFunc, requireCSRF bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("mail_admin_session")
		if err != nil {
			writeError(w, http.StatusUnauthorized, "login required")
			return
		}
		session, ok := s.sessions.Get(cookie.Value)
		if !ok {
			writeError(w, http.StatusUnauthorized, "login required")
			return
		}
		if requireCSRF && r.Header.Get("X-CSRF-Token") != session.CSRFToken {
			writeError(w, http.StatusForbidden, "invalid CSRF token")
			return
		}
		ctx := context.WithValue(r.Context(), sessionKey{}, session)
		next(w, r.WithContext(ctx))
	}
}

func (s *Server) setSessionCookie(w http.ResponseWriter, id string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     "mail_admin_session",
		Value:    id,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.cfg.SessionSecure,
	})
}

func splitUserAction(path string) (string, string, bool) {
	path = strings.TrimPrefix(path, "/api/users/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		return "", "", false
	}
	email, err := url.PathUnescape(parts[0])
	if err != nil {
		return "", "", false
	}
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}
	return email, action, true
}

func readJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	return json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(target)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func randomHex(size int) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf)
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func stripANSI(value string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(value, "")
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}
