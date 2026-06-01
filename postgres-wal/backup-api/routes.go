package main

import "net/http"

func (s *server) routes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /swagger", s.swaggerUI)
	mux.HandleFunc("GET /openapi.json", s.openapi)

	mux.Handle("GET /v1/status", s.auth(http.HandlerFunc(s.status)))
	mux.Handle("GET /v1/database/status", s.auth(http.HandlerFunc(s.databaseStatus)))

	mux.Handle("GET /v1/backups", s.auth(http.HandlerFunc(s.listBackups)))
	mux.Handle("POST /v1/backups/run", s.auth(http.HandlerFunc(s.runBackupHTTP)))

	mux.Handle("GET /v1/restores/options", s.auth(http.HandlerFunc(s.restoreOptions)))
	mux.Handle("POST /v1/restores/run", s.auth(http.HandlerFunc(s.restoreRunHTTP)))

	mux.Handle("GET /v1/s3/status", s.auth(http.HandlerFunc(s.s3Status)))
	mux.Handle("GET /v1/pitr/chains", s.auth(http.HandlerFunc(s.pitrChainsHTTP)))
	mux.Handle("POST /v1/pitr/download", s.auth(http.HandlerFunc(s.pitrDownloadHTTP)))
	mux.Handle("GET /v1/wal/status", s.auth(http.HandlerFunc(s.walStatus)))
	mux.Handle("POST /v1/wal/upload", s.auth(http.HandlerFunc(s.uploadWALHTTP)))

	return mux
}
