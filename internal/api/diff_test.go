package api

import (
	"testing"

	"github.com/V-Sarayu/tracebox/internal/storage"
)

func TestComputeDiff_IdenticalRequests_NoChanges(t *testing.T) {
	a := &storage.RecordedRequest{
		ResponseStatus:  200,
		ResponseHeaders: map[string][]string{"Content-Type": {"application/json"}},
		ResponseBody:    []byte(`{"stock":10}`),
	}
	b := &storage.RecordedRequest{
		ResponseStatus:  200,
		ResponseHeaders: map[string][]string{"Content-Type": {"application/json"}},
		ResponseBody:    []byte(`{"stock":10}`),
	}

	result := computeDiff(a, b)

	if result.StatusChanged {
		t.Errorf("expected StatusChanged=false, got true")
	}
	if result.BodyChanged {
		t.Errorf("expected BodyChanged=false, got true")
	}
	if len(result.HeadersAdded) != 0 || len(result.HeadersRemoved) != 0 || len(result.HeadersChanged) != 0 {
		t.Errorf("expected no header diffs, got added=%v removed=%v changed=%v",
			result.HeadersAdded, result.HeadersRemoved, result.HeadersChanged)
	}
}

func TestComputeDiff_StatusChanged(t *testing.T) {
	a := &storage.RecordedRequest{ResponseStatus: 200, ResponseHeaders: map[string][]string{}}
	b := &storage.RecordedRequest{ResponseStatus: 500, ResponseHeaders: map[string][]string{}}

	result := computeDiff(a, b)

	if !result.StatusChanged {
		t.Errorf("expected StatusChanged=true for 200 -> 500")
	}
	if result.StatusA != 200 || result.StatusB != 500 {
		t.Errorf("expected StatusA=200 StatusB=500, got StatusA=%d StatusB=%d", result.StatusA, result.StatusB)
	}
}

func TestComputeDiff_BodyChanged(t *testing.T) {
	a := &storage.RecordedRequest{ResponseStatus: 200, ResponseHeaders: map[string][]string{}, ResponseBody: []byte(`{"stock":10}`)}
	b := &storage.RecordedRequest{ResponseStatus: 200, ResponseHeaders: map[string][]string{}, ResponseBody: []byte(`{"stock":4}`)}

	result := computeDiff(a, b)

	if !result.BodyChanged {
		t.Errorf("expected BodyChanged=true when bodies differ")
	}
	if result.BodyA != `{"stock":10}` || result.BodyB != `{"stock":4}` {
		t.Errorf("body diff did not capture both original values correctly")
	}
}

func TestComputeDiff_HeaderAddedAndRemoved(t *testing.T) {
	a := &storage.RecordedRequest{
		ResponseStatus:  200,
		ResponseHeaders: map[string][]string{"X-Old": {"gone"}},
	}
	b := &storage.RecordedRequest{
		ResponseStatus:  200,
		ResponseHeaders: map[string][]string{"X-New": {"here"}},
	}

	result := computeDiff(a, b)

	if _, ok := result.HeadersAdded["X-New"]; !ok {
		t.Errorf("expected X-New to be reported as added")
	}
	if _, ok := result.HeadersRemoved["X-Old"]; !ok {
		t.Errorf("expected X-Old to be reported as removed")
	}
}

func TestComputeDiff_HeaderValueChanged(t *testing.T) {
	a := &storage.RecordedRequest{
		ResponseStatus:  200,
		ResponseHeaders: map[string][]string{"X-Version": {"v1"}},
	}
	b := &storage.RecordedRequest{
		ResponseStatus:  200,
		ResponseHeaders: map[string][]string{"X-Version": {"v2"}},
	}

	result := computeDiff(a, b)

	changed, ok := result.HeadersChanged["X-Version"]
	if !ok {
		t.Fatalf("expected X-Version to be reported as changed")
	}
	if changed[0] != "v1" || changed[1] != "v2" {
		t.Errorf("expected [v1 v2], got %v", changed)
	}
}
