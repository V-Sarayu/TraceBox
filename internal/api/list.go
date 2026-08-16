package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/V-Sarayu/tracebox/internal/storage"
)

type ListHandler struct {
	store storage.RequestStore
}

func NewListHandler(store storage.RequestStore) *ListHandler {
	return &ListHandler{store: store}
}

func (h *ListHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	requests, err := h.store.ListRequests(ctx, 50)
	if err != nil {
		http.Error(w, "failed to list requests", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(requests)
}
