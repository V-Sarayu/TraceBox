package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"time"
)

type item struct {
	SKU   string `json:"sku"`
	Stock int    `json:"stock"`
}

func inventoryHandler(w http.ResponseWriter, r *http.Request) {
	// simulate variable latency so the dependency graph has something to flag
	time.Sleep(time.Duration(rand.Intn(150)) * time.Millisecond)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(item{SKU: "sku-123", Stock: rand.Intn(50)})
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/inventory/check", inventoryHandler)

	log.Println("inventory service listening on :9002")
	log.Fatal(http.ListenAndServe(":9002", mux))
}
