package proxy

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/V-Sarayu/tracebox/internal/storage"
)

type Proxy struct {
	store    storage.RequestStore
	handlers map[string]*httputil.ReverseProxy // path prefix -> target
}

func New(store storage.RequestStore, targets map[string]string) (*Proxy, error) {
	p := &Proxy{
		store:    store,
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

// responseRecorder captures status + body while still writing through to the real client.
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
	r.body.Write(b)                  // copy for recording
	return r.ResponseWriter.Write(b) // still send to the real client
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
