package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/StevenBuglione/oas-cli-go/internal/runtime"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8765", "listen address")
	auditPath := flag.String("audit-path", ".cache/audit.log", "audit log path")
	cacheDir := flag.String("cache-dir", ".cache/http", "cache directory for remote discovery and OpenAPI documents")
	flag.Parse()

	server := runtime.NewServer(runtime.Options{
		AuditPath: *auditPath,
		CacheDir:  *cacheDir,
	})
	log.Fatal(http.ListenAndServe(*addr, server.Handler()))
}
