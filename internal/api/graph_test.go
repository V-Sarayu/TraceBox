package api

import (
	"testing"

	"github.com/V-Sarayu/tracebox/internal/storage"
)

func TestBuildGraph_SingleHop_ClientToService(t *testing.T) {
	requests := []*storage.RecordedRequest{
		{SpanID: "span1", ParentSpanID: "", TargetService: "/orders", DurationMs: 50},
	}

	result := buildGraph(requests)

	if len(result.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(result.Edges))
	}
	edge := result.Edges[0]
	if edge.Source != "client" || edge.Target != "/orders" {
		t.Errorf("expected client -> /orders, got %s -> %s", edge.Source, edge.Target)
	}
	if edge.CallCount != 1 {
		t.Errorf("expected call_count=1, got %d", edge.CallCount)
	}
}

func TestBuildGraph_MultiHop_ParentChildLinked(t *testing.T) {
	// orders (root span) calls inventory (child span, parent = orders' span id)
	requests := []*storage.RecordedRequest{
		{SpanID: "span-orders", ParentSpanID: "", TargetService: "/orders", DurationMs: 80},
		{SpanID: "span-inventory", ParentSpanID: "span-orders", TargetService: "/inventory", DurationMs: 40},
	}

	result := buildGraph(requests)

	if len(result.Edges) != 2 {
		t.Fatalf("expected 2 edges (client->orders, orders->inventory), got %d", len(result.Edges))
	}

	var foundClientToOrders, foundOrdersToInventory bool
	for _, e := range result.Edges {
		if e.Source == "client" && e.Target == "/orders" {
			foundClientToOrders = true
		}
		if e.Source == "/orders" && e.Target == "/inventory" {
			foundOrdersToInventory = true
		}
	}
	if !foundClientToOrders {
		t.Errorf("expected an edge client -> /orders")
	}
	if !foundOrdersToInventory {
		t.Errorf("expected an edge /orders -> /inventory, parent/child linkage failed")
	}
}

func TestBuildGraph_RepeatedCalls_AggregatesLatencyCorrectly(t *testing.T) {
	requests := []*storage.RecordedRequest{
		{SpanID: "s1", ParentSpanID: "", TargetService: "/orders", DurationMs: 100},
		{SpanID: "s2", ParentSpanID: "", TargetService: "/orders", DurationMs: 200},
	}

	result := buildGraph(requests)

	if len(result.Edges) != 1 {
		t.Fatalf("expected calls to the same target to aggregate into 1 edge, got %d", len(result.Edges))
	}
	edge := result.Edges[0]
	if edge.CallCount != 2 {
		t.Errorf("expected call_count=2, got %d", edge.CallCount)
	}
	if edge.AvgLatency != 150 {
		t.Errorf("expected avg_latency=150 (avg of 100,200), got %f", edge.AvgLatency)
	}
	if edge.MaxLatency != 200 {
		t.Errorf("expected max_latency=200, got %d", edge.MaxLatency)
	}
}

func TestBuildGraph_SlowEdgeFlagging(t *testing.T) {
	requests := []*storage.RecordedRequest{
		{SpanID: "fast", ParentSpanID: "", TargetService: "/inventory", DurationMs: 20},
		{SpanID: "slow", ParentSpanID: "", TargetService: "/orders", DurationMs: 500},
	}

	result := buildGraph(requests)

	for _, e := range result.Edges {
		if e.Target == "/orders" && !e.IsSlow {
			t.Errorf("expected /orders edge (500ms avg) to be flagged slow")
		}
		if e.Target == "/inventory" && e.IsSlow {
			t.Errorf("expected /inventory edge (20ms avg) to NOT be flagged slow")
		}
	}
}

func TestBuildGraph_EmptyInput_ReturnsEmptyGraph(t *testing.T) {
	result := buildGraph([]*storage.RecordedRequest{})

	if len(result.Nodes) != 0 {
		t.Errorf("expected 0 nodes for empty input, got %d", len(result.Nodes))
	}
	if len(result.Edges) != 0 {
		t.Errorf("expected 0 edges for empty input, got %d", len(result.Edges))
	}
}
