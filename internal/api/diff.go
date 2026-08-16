package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/V-Sarayu/tracebox/internal/storage"
)

type DiffResult struct {
	StatusChanged  bool                 `json:"status_changed"`
	StatusA        int                  `json:"status_a"`
	StatusB        int                  `json:"status_b"`
	HeadersAdded   map[string]string    `json:"headers_added,omitempty"`
	HeadersRemoved map[string]string    `json:"headers_removed,omitempty"`
	HeadersChanged map[string][2]string `json:"headers_changed,omitempty"`
	BodyChanged    bool                 `json:"body_changed"`
	BodyA          string               `json:"body_a,omitempty"`
	BodyB          string               `json:"body_b,omitempty"`
}

type DiffHandler struct {
	store storage.RequestStore
}

func NewDiffHandler(store storage.RequestStore) *DiffHandler {
	return &DiffHandler{store: store}
}

func (h *DiffHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	idA := r.URL.Query().Get("a")
	idB := r.URL.Query().Get("b")
	if idA == "" || idB == "" {
		http.Error(w, "missing a or b query param", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reqA, err := h.store.GetRequest(ctx, idA)
	if err != nil {
		http.Error(w, "request a not found", http.StatusNotFound)
		return
	}
	reqB, err := h.store.GetRequest(ctx, idB)
	if err != nil {
		http.Error(w, "request b not found", http.StatusNotFound)
		return
	}

	result := computeDiff(reqA, reqB)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func computeDiff(a, b *storage.RecordedRequest) DiffResult {
	result := DiffResult{
		StatusA:        a.ResponseStatus,
		StatusB:        b.ResponseStatus,
		StatusChanged:  a.ResponseStatus != b.ResponseStatus,
		HeadersAdded:   map[string]string{},
		HeadersRemoved: map[string]string{},
		HeadersChanged: map[string][2]string{},
	}

	for k, v := range b.ResponseHeaders {
		if _, ok := a.ResponseHeaders[k]; !ok {
			result.HeadersAdded[k] = firstOrEmpty(v)
		}
	}
	for k, v := range a.ResponseHeaders {
		bv, ok := b.ResponseHeaders[k]
		if !ok {
			result.HeadersRemoved[k] = firstOrEmpty(v)
			continue
		}
		if firstOrEmpty(v) != firstOrEmpty(bv) {
			result.HeadersChanged[k] = [2]string{firstOrEmpty(v), firstOrEmpty(bv)}
		}
	}

	bodyA := string(a.ResponseBody)
	bodyB := string(b.ResponseBody)
	result.BodyChanged = bodyA != bodyB
	if result.BodyChanged {
		result.BodyA = bodyA
		result.BodyB = bodyB
	}

	return result
}

func firstOrEmpty(v []string) string {
	if len(v) == 0 {
		return ""
	}
	return v[0]
}
