package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	httpServer := &http.Server{
		Addr:    *addr,
		Handler: mux,
	}

	// Graceful shutdown on SIGINT/SIGTERM
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Printf("HookRelay shutting down...")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			log.Printf("shutdown error: %v", err)
		}
	}()

	log.Printf("HookRelay listening on %s", *addr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}

	log.Printf("HookRelay stopped")
}
