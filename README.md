# TraceBox

**API request recording, replay, diffing, and live service dependency visualization.**

TraceBox sits between services as a reverse proxy, transparently recording every request and response that passes through it. Any recorded request can be replayed on demand, and two recorded responses can be diffed side by side to see exactly what changed — same endpoint, different point in time. Using W3C `traceparent` propagation, it reconstructs multi-hop call chains into a live dependency graph, flagging slow hops automatically.

It combines Postman-style request history with a lightweight slice of what a distributed tracing system (Jaeger, Datadog APM) provides — purpose-built around one question: **what changed, and where did the time go?**

---

## Architecture

```
                    ┌─────────────┐
   Client  ────────▶│   Proxy /   │──── records every hop ────▶  PostgreSQL
                    │   API (Go)  │                                (requests,
                    └──────┬──────┘                                 spans)
                           │
              ┌────────────┼────────────┐
              ▼                         ▼
        demo-orders                demo-inventory
        (calls inventory
         THROUGH the proxy,
         propagating traceparent)

   Proxy also publishes each recorded request to Redis (pub/sub) ──▶
   WebSocket handler ──▶ React dashboard (live table + dependency graph)
```

**Request flow:** every request to a demo service is routed through the TraceBox proxy rather than hitting the service directly. The proxy forwards the request using Go's `httputil.ReverseProxy`, capturing a copy of the request and response bodies via `io.TeeReader` and a wrapped `http.ResponseWriter`, without disrupting the actual proxying. Recording to Postgres happens asynchronously in a goroutine, so persistence never adds latency to the proxied response — which matters, since that response time is the exact thing being measured and displayed.

**Tracing:** each request carries (or receives, if absent) a W3C `traceparent` header (`00-{trace-id}-{span-id}-01`). When `demo-orders` calls `demo-inventory`, it does so *through the proxy* and forwards its own `traceparent`, so the downstream call's `parent_span_id` links back to the originating span. The dependency graph is reconstructed purely from this parent/child linkage already present in stored request records — no separate tracing infrastructure required.

**Live updates:** the proxy publishes a message to a Redis channel after every successful recording. A Go WebSocket handler holds a single Redis subscription and fans each message out to every connected dashboard client, so multiple open browser tabs share one subscription rather than each opening their own. If Redis is unavailable, recording still works — publishing is deliberately best-effort, since Postgres is the actual source of truth.

---

## Why no Kubernetes

This is a two-service backend, two small demo services, Postgres, and Redis — five containers, one host, one deploy target. Kubernetes solves problems this system doesn't have at this scale: multi-node scheduling, rolling updates across replicas, service mesh, autoscaling under real traffic variance. Introducing it here would mean writing and maintaining a meaningful amount of orchestration configuration to solve problems that don't exist yet.

Docker Compose expresses the system's actual requirements — multiple services, one shared network, one command to bring it all up — without paying for orchestration complexity the project's scale doesn't call for. If TraceBox needed to run across multiple nodes with rolling deploys and autoscaling, that would be a legitimate reason to introduce Kubernetes as a deliberate next step, not a default.

---

## Tech stack

| Layer | Choice | Why |
|---|---|---|
| Proxy & API | Go, `net/http` | Goroutines make async recording trivial; strong fit for a proxy/middleware workload |
| Storage | PostgreSQL | Relational fit for structured request/response/span records; `JSONB` for flexible header storage |
| Live updates | Redis pub/sub + WebSockets | Decouples "a request happened" from "who's listening" |
| Dashboard | React + React Flow | Purpose-built for interactive node/edge graph rendering |
| Local dev | Docker Compose | Multi-service orchestration without Kubernetes-scale complexity |
| Deploy | Fly.io | Simple multi-service deploys with a custom domain, no cluster to manage |

---

## Running it locally

**Requirements:** Docker Desktop.

```bash
git clone https://github.com/V-Sarayu/tracebox.git
cd tracebox
cp .env.example .env
docker compose up -d --build
```

This starts five containers: `postgres`, `redis`, `server` (proxy + API), `orders`, `inventory`.

In a separate terminal, start the dashboard:
```bash
cd web
npm install
npm run dev
```

Open `http://localhost:5173`.

Generate traffic to see it in action:
```bash
curl http://localhost:8080/orders/create
```

The dashboard updates live — no refresh needed.

---

## API

| Endpoint | Description |
|---|---|
| `GET /api/requests` | List recently recorded requests |
| `GET /api/replay?id={id}` | Re-issue a previously recorded request |
| `GET /api/diff?a={id}&b={id}` | Structural diff between two recorded responses (status, headers, body) |
| `GET /api/graph` | Reconstructed dependency graph: nodes, edges, call counts, average/max latency, slow-hop flags |
| `GET /ws` | WebSocket stream — pushes a message whenever a new request is recorded |

---

## Project structure

```
cmd/server/          entry point — wires config, storage, proxy, and API together
internal/
  config/            environment-variable driven configuration
  storage/           RequestStore interface + Postgres implementation (swappable/testable)
  proxy/             reverse proxy: recording, trace context propagation
  api/                HTTP handlers: list, replay, diff, graph, websocket
demo-services/
  orders/            calls inventory through the proxy, to generate real multi-hop traces
  inventory/
web/                 React dashboard (Vite + React Flow)
```

`storage.RequestStore` is defined as an interface, with `PostgresRequestStore` as its only current implementation — the proxy and API depend on the interface, not the concrete type, so storage is swappable and mockable in tests without touching a real database.

---

## Testing

```bash
go test ./... -v
```

Tests focus on the two pieces of core logic in the system — response diffing and dependency graph reconstruction from span parent/child linkage — rather than integration-testing the database or HTTP layers, which are thinner and lower-risk by comparison.

---

## Roadmap

- Persist and diff full request *headers*, not just response headers, for a more complete comparison
- Span-level view: click a trace ID to see its full multi-hop timeline, not just the aggregated graph
- Configurable slow-hop threshold per service, rather than one global constant
- Authentication on the dashboard/API ahead of any deployment beyond a demo environment