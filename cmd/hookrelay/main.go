package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/relentlessworks/hookrelay/internal/api"
	"github.com/relentlessworks/hookrelay/internal/auth"
	"github.com/relentlessworks/hookrelay/internal/store"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	dbPath := flag.String("db", "hookrelay.json", "path to data file")
	tokenSecret := flag.String("secret", "", "secret for signing tokens (defaults to random)")
	flag.Parse()

	// Layered config: defaults < env < flags
	// Flags take priority if explicitly set; otherwise env overrides defaults
	if v := os.Getenv("HOOKRELAY_ADDR"); v != "" && *addr == ":8080" {
		*addr = v
	}
	if v := os.Getenv("HOOKRELAY_DB"); v != "" && *dbPath == "hookrelay.json" {
		*dbPath = v
	}
	if v := os.Getenv("HOOKRELAY_SECRET"); v != "" && *tokenSecret == "" {
		*tokenSecret = v
	}

	// Initialize store
	db, err := store.New(*dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Initialize auth
	authSvc := auth.New(*tokenSecret)

	// Initialize API server
	srv := api.NewServer(db, authSvc)

	mux := http.NewServeMux()
	mux.HandleFunc("/", srv.Router)

	log.Printf("HookRelay listening on %s", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
