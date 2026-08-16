package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/V-Sarayu/tracebox/internal/api"
	"github.com/V-Sarayu/tracebox/internal/config"
	"github.com/V-Sarayu/tracebox/internal/proxy"
	"github.com/V-Sarayu/tracebox/internal/storage"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
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

	redisClient := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr()})
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		log.Printf("warning: could not connect to redis (%v) — live updates disabled, recording still works", err)
		redisClient = nil
	} else {
		log.Println("connected to redis")
	}

	store := storage.NewPostgresRequestStore(db)

	replayHandler := api.NewReplayHandler(store, "http://localhost:8080")
	diffHandler := api.NewDiffHandler(store)
	listHandler := api.NewListHandler(store)
	graphHandler := api.NewGraphHandler(store)
	wsHandler := api.NewWSHandler(redisClient)

	ordersURL := os.Getenv("ORDERS_URL")
	if ordersURL == "" {
		ordersURL = "http://localhost:9001"
	}
	inventoryURL := os.Getenv("INVENTORY_URL")
	if inventoryURL == "" {
		inventoryURL = "http://localhost:9002"
	}

	p, err := proxy.New(store, redisClient, map[string]string{
		"/orders":    ordersURL,
		"/inventory": inventoryURL,
	})
	if err != nil {
		log.Fatalf("failed to set up proxy: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.Handle("/orders/", p)
	mux.Handle("/inventory/", p)
	mux.Handle("/api/replay", replayHandler)
	mux.Handle("/api/requests", listHandler)
	mux.Handle("/api/diff", diffHandler)
	mux.Handle("/api/graph", graphHandler)
	mux.Handle("/ws", wsHandler)

	addr := ":8080"
	log.Printf("TraceBox proxy listening on %s", addr)
	if err := http.ListenAndServe(addr, api.WithCORS(mux)); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
