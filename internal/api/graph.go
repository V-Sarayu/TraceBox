package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/V-Sarayu/tracebox/internal/storage"
)

type GraphNode struct {
	ID string `json:"id"`
}

type GraphEdge struct {
	Source     string  `json:"source"`
	Target     string  `json:"target"`
	CallCount  int     `json:"call_count"`
	AvgLatency float64 `json:"avg_latency_ms"`
	MaxLatency int     `json:"max_latency_ms"`
	IsSlow     bool    `json:"is_slow"`
}

type GraphResult struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

const slowThresholdMs = 100

type GraphHandler struct {
	store storage.RequestStore
}

func NewGraphHandler(store storage.RequestStore) *GraphHandler {
	return &GraphHandler{store: store}
}

func (h *GraphHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	requests, err := h.store.ListRequests(ctx, 500)
	if err != nil {
		http.Error(w, "failed to load requests", http.StatusInternalServerError)
		return
	}

	result := buildGraph(requests)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func buildGraph(requests []*storage.RecordedRequest) GraphResult {
	bySpanID := make(map[string]*storage.RecordedRequest)
	for _, r := range requests {
		if r.SpanID != "" {
			bySpanID[r.SpanID] = r
		}
	}

	type edgeKey struct{ source, target string }
	edgeStats := make(map[edgeKey]*GraphEdge)
	nodeSet := make(map[string]bool)

	for _, r := range requests {
		target := r.TargetService
		nodeSet[target] = true

		source := "client"
		if r.ParentSpanID != "" {
			if parent, ok := bySpanID[r.ParentSpanID]; ok {
				source = parent.TargetService
			}
		}
		nodeSet[source] = true

		key := edgeKey{source, target}
		e, exists := edgeStats[key]
		if !exists {
			e = &GraphEdge{Source: source, Target: target}
			edgeStats[key] = e
		}
		e.CallCount++
		if r.DurationMs > e.MaxLatency {
			e.MaxLatency = r.DurationMs
		}
		e.AvgLatency = ((e.AvgLatency * float64(e.CallCount-1)) + float64(r.DurationMs)) / float64(e.CallCount)
	}

	result := GraphResult{}
	for node := range nodeSet {
		result.Nodes = append(result.Nodes, GraphNode{ID: node})
	}
	for _, e := range edgeStats {
		e.IsSlow = e.AvgLatency > slowThresholdMs
		result.Edges = append(result.Edges, *e)
	}
	return result
}
