package main

import (
	"encoding/json"
	"io"
	"log"
	"math/rand"
	"net/http"
	"time"
)

type orderResponse struct {
	OrderID       string `json:"order_id"`
	InventoryData string `json:"inventory_data"`
}

func ordersHandler(w http.ResponseWriter, r *http.Request) {
	time.Sleep(time.Duration(rand.Intn(80)) * time.Millisecond)

	// call inventory THROUGH the proxy, not directly — this is what makes it a real hop
	req, err := http.NewRequest("GET", "http://localhost:8080/inventory/check", nil)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	// propagate the incoming trace context to the downstream call
	if tp := r.Header.Get("traceparent"); tp != "" {
		req.Header.Set("traceparent", tp)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "inventory call failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(orderResponse{
		OrderID:       "order-abc",
		InventoryData: string(body),
	})
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/orders/create", ordersHandler)

	log.Println("orders service listening on :9001")
	log.Fatal(http.ListenAndServe(":9001", mux))
}
