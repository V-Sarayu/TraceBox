package proxy

import (
	"fmt"
	"net/http"
	"strings"
)

func extractOrCreateTraceContext(r *http.Request) (traceID, spanID, parentSpanID string) {
	header := r.Header.Get("traceparent")
	spanID = newID(8) // this hop's own span id, always fresh

	if header == "" {
		// no incoming trace — this request is the root of a new trace
		traceID = newID(16)
		parentSpanID = ""
		return
	}

	parts := strings.Split(header, "-")
	if len(parts) != 4 {
		traceID = newID(16)
		parentSpanID = ""
		return
	}
	traceID = parts[1]
	parentSpanID = parts[2] // the incoming span becomes this hop's parent
	return
}

func buildTraceparent(traceID, spanID string) string {
	return fmt.Sprintf("00-%s-%s-01", traceID, spanID)
}
