package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

type RecordedRequest struct {
	ID              string
	Method          string
	Path            string
	TargetService   string
	RequestHeaders  map[string][]string
	RequestBody     []byte
	ResponseStatus  int
	ResponseHeaders map[string][]string
	ResponseBody    []byte
	DurationMs      int
	TraceID         string
	SpanID          string
	ParentSpanID    string
	CreatedAt       time.Time
}

// RequestStore is the interface the proxy and API depend on — not *sql.DB directly.
// This is what makes storage swappable/mockable in tests.
type RequestStore interface {
	SaveRequest(ctx context.Context, r *RecordedRequest) error
	GetRequest(ctx context.Context, id string) (*RecordedRequest, error)
	ListRequests(ctx context.Context, limit int) ([]*RecordedRequest, error)
	GetRequestsByTraceID(ctx context.Context, traceID string) ([]*RecordedRequest, error)
}

type PostgresRequestStore struct {
	db *sql.DB
}

func NewPostgresRequestStore(db *sql.DB) *PostgresRequestStore {
	return &PostgresRequestStore{db: db}
}

func (s *PostgresRequestStore) SaveRequest(ctx context.Context, r *RecordedRequest) error {
	reqHeaders, _ := json.Marshal(r.RequestHeaders)
	resHeaders, _ := json.Marshal(r.ResponseHeaders)

	query := `
		INSERT INTO requests
			(method, path, target_service, request_headers, request_body,
			 response_status, response_headers, response_body, duration_ms,
			 trace_id, span_id, parent_span_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING id, created_at`

	return s.db.QueryRowContext(ctx, query,
		r.Method, r.Path, r.TargetService, reqHeaders, r.RequestBody,
		r.ResponseStatus, resHeaders, r.ResponseBody, r.DurationMs,
		r.TraceID, r.SpanID, r.ParentSpanID,
	).Scan(&r.ID, &r.CreatedAt)
}

func (s *PostgresRequestStore) GetRequest(ctx context.Context, id string) (*RecordedRequest, error) {
	query := `
		SELECT id, method, path, target_service, request_headers, request_body,
		       response_status, response_headers, response_body, duration_ms,
		       trace_id, span_id, parent_span_id, created_at
		FROM requests WHERE id = $1`

	return scanRequest(s.db.QueryRowContext(ctx, query, id))
}

func (s *PostgresRequestStore) ListRequests(ctx context.Context, limit int) ([]*RecordedRequest, error) {
	query := `
		SELECT id, method, path, target_service, request_headers, request_body,
		       response_status, response_headers, response_body, duration_ms,
		       trace_id, span_id, parent_span_id, created_at
		FROM requests ORDER BY created_at DESC LIMIT $1`

	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*RecordedRequest
	for rows.Next() {
		r, err := scanRequestRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *PostgresRequestStore) GetRequestsByTraceID(ctx context.Context, traceID string) ([]*RecordedRequest, error) {
	query := `
		SELECT id, method, path, target_service, request_headers, request_body,
		       response_status, response_headers, response_body, duration_ms,
		       trace_id, span_id, parent_span_id, created_at
		FROM requests WHERE trace_id = $1 ORDER BY created_at ASC`

	rows, err := s.db.QueryContext(ctx, query, traceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*RecordedRequest
	for rows.Next() {
		r, err := scanRequestRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// scanner shared between single-row (QueryRow) and multi-row (rows.Next) paths
type rowScanner interface {
	Scan(dest ...any) error
}

func scanRequest(row rowScanner) (*RecordedRequest, error) {
	return scanRequestRow(row)
}

func scanRequestRow(row rowScanner) (*RecordedRequest, error) {
	var r RecordedRequest
	var reqHeaders, resHeaders []byte

	err := row.Scan(
		&r.ID, &r.Method, &r.Path, &r.TargetService, &reqHeaders, &r.RequestBody,
		&r.ResponseStatus, &resHeaders, &r.ResponseBody, &r.DurationMs,
		&r.TraceID, &r.SpanID, &r.ParentSpanID, &r.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(reqHeaders, &r.RequestHeaders)
	json.Unmarshal(resHeaders, &r.ResponseHeaders)
	return &r, nil
}
