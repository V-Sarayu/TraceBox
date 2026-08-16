package api

import (
	"context"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

var upgrader = websocket.Upgrader{
	// dev-friendly: allow any origin. Fine for a local/portfolio demo;
	// a real production deploy would restrict this to the dashboard's actual origin.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// WSHandler fans out messages from a single Redis subscription to every
// connected browser client, so N dashboard tabs share one Redis subscription
// rather than each opening their own.
type WSHandler struct {
	redis   *redis.Client
	clients map[*websocket.Conn]bool
	mu      sync.Mutex
}

func NewWSHandler(redisClient *redis.Client) *WSHandler {
	h := &WSHandler{
		redis:   redisClient,
		clients: make(map[*websocket.Conn]bool),
	}
	go h.listenRedis()
	return h
}

func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade failed: %v", err)
		return
	}

	h.mu.Lock()
	h.clients[conn] = true
	h.mu.Unlock()

	// block here reading (and discarding) pings/close frames from the client,
	// so we notice disconnects and can clean up
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			h.mu.Lock()
			delete(h.clients, conn)
			h.mu.Unlock()
			conn.Close()
			return
		}
	}
}

func (h *WSHandler) listenRedis() {
	if h.redis == nil {
		log.Println("websocket: no redis client configured, live updates disabled")
		return
	}

	ctx := context.Background()
	sub := h.redis.Subscribe(ctx, "tracebox:requests")
	defer sub.Close()

	ch := sub.Channel()
	for msg := range ch {
		h.broadcast([]byte(msg.Payload))
	}
}

func (h *WSHandler) broadcast(payload []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for conn := range h.clients {
		if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			conn.Close()
			delete(h.clients, conn)
		}
	}
}
