CREATE TABLE IF NOT EXISTS requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    method TEXT NOT NULL,
    path TEXT NOT NULL,
    target_service TEXT NOT NULL,
    request_headers JSONB NOT NULL DEFAULT '{}',
    request_body BYTEA,
    response_status INT,
    response_headers JSONB NOT NULL DEFAULT '{}',
    response_body BYTEA,
    duration_ms INT,
    trace_id TEXT,
    span_id TEXT,
    parent_span_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_requests_trace_id ON requests(trace_id);
CREATE INDEX IF NOT EXISTS idx_requests_created_at ON requests(created_at DESC);