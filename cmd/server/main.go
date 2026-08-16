package main

import (
	"log"
	"net/http"

	"github.com/V-Sarayu/tracebox/internal/config"
	"github.com/V-Sarayu/tracebox/internal/storage"
	"github.com/joho/godotenv"
)

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, relying on real environment variables")
	}

	cfg := config.Load()

	db, err := storage.NewPostgresDB(cfg.PostgresDSN())
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()
	log.Println("connected to postgres")

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)

	addr := ":8080"
	log.Printf("TraceBox server listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
