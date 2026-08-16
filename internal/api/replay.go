package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/V-Sarayu/tracebox/internal/storage"
)

type ReplayHandler struct {
	store     storage.RequestStore
	proxyBase string // e.g. "http://localhost:8080"
}

func NewReplayHandler(store storage.RequestStore, proxyBase string) *ReplayHandler {
	return &ReplayHandler{store: store, proxyBase: proxyBase}
}

func (h *ReplayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id query param", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	original, err := h.store.GetRequest(ctx, id)
	if err != nil {
		http.Error(w, fmt.Sprintf("request not found: %v", err), http.StatusNotFound)
		return
	}

	url := h.proxyBase + original.Path
	req, err := http.NewRequest(original.Method, url, bytes.NewReader(original.RequestBody))
	if err != nil {
		http.Error(w, "failed to build replay request", http.StatusInternalServerError)
		return
	}
	for k, values := range original.RequestHeaders {
		for _, v := range values {
			req.Header.Add(k, v)
		}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("replay failed: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"original_id":     original.ID,
		"replay_status":   resp.StatusCode,
		"original_status": original.ResponseStatus,
	})
}
