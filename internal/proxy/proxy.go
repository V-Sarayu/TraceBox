package proxy

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/V-Sarayu/tracebox/internal/storage"
	"github.com/redis/go-redis/v9"
)

const redisChannel = "tracebox:requests"

type Proxy struct {
	store    storage.RequestStore
	redis    *redis.Client // may be nil if Redis isn't configured — publish is best-effort
	handlers map[string]*httputil.ReverseProxy
}

func New(store storage.RequestStore, redisClient *redis.Client, targets map[string]string) (*Proxy, error) {
	p := &Proxy{
		store:    store,
		redis:    redisClient,
		handlers: make(map[string]*httputil.ReverseProxy),
	}
	for prefix, target := range targets {
		u, err := url.Parse(target)
		if err != nil {
			return nil, fmt.Errorf("parsing target %q: %w", target, err)
		}
		p.handlers[prefix] = httputil.NewSingleHostReverseProxy(u)
	}
	return p, nil
}

type responseRecorder struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	prefix, target, handler := p.match(r.URL.Path)
	if handler == nil {
		http.NotFound(w, r)
		return
	}

	traceID, spanID, parentSpanID := extractOrCreateTraceContext(r)
	r.Header.Set("traceparent", buildTraceparent(traceID, spanID))

	var reqBody bytes.Buffer
	if r.Body != nil {
		r.Body = io.NopCloser(io.TeeReader(r.Body, &reqBody))
	}
	reqHeaders := cloneHeaders(r.Header)

	rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}

	start := time.Now()
	handler.ServeHTTP(rec, r)
	duration := time.Since(start)

	go p.record(context.Background(), r, prefix, target, reqHeaders, reqBody.Bytes(),
		rec, duration, traceID, spanID, parentSpanID)
}

func (p *Proxy) match(path string) (prefix, target string, handler *httputil.ReverseProxy) {
	for pre, h := range p.handlers {
		if len(path) >= len(pre) && path[:len(pre)] == pre {
			return pre, pre, h
		}
	}
	return "", "", nil
}

func (p *Proxy) record(ctx context.Context, r *http.Request, prefix, target string,
	reqHeaders map[string][]string, reqBody []byte,
	rec *responseRecorder, duration time.Duration,
	traceID, spanID, parentSpanID string) {

	record := &storage.RecordedRequest{
		Method:          r.Method,
		Path:            r.URL.Path,
		TargetService:   prefix,
		RequestHeaders:  reqHeaders,
		RequestBody:     reqBody,
		ResponseStatus:  rec.status,
		ResponseHeaders: cloneHeaders(rec.Header()),
		ResponseBody:    rec.body.Bytes(),
		DurationMs:      int(duration.Milliseconds()),
		TraceID:         traceID,
		SpanID:          spanID,
		ParentSpanID:    parentSpanID,
	}

	if err := p.store.SaveRequest(ctx, record); err != nil {
		log.Printf("failed to record request: %v", err)
		return
	}

	p.publish(ctx, record)
}

// publish notifies any live dashboard clients that a new request was recorded.
// Best-effort: if Redis is unavailable, we log and move on — recording to
// Postgres (the source of truth) already succeeded, so we never fail the
// request over a pub/sub hiccup.
func (p *Proxy) publish(ctx context.Context, record *storage.RecordedRequest) {
	if p.redis == nil {
		return
	}
	payload, err := json.Marshal(record)
	if err != nil {
		log.Printf("failed to marshal record for publish: %v", err)
		return
	}
	if err := p.redis.Publish(ctx, redisChannel, payload).Err(); err != nil {
		log.Printf("failed to publish to redis: %v", err)
	}
}

func cloneHeaders(h http.Header) map[string][]string {
	out := make(map[string][]string, len(h))
	for k, v := range h {
		out[k] = v
	}
	return out
}

func newID(bytesLen int) string {
	b := make([]byte, bytesLen)
	rand.Read(b)
	return hex.EncodeToString(b)
}
