// Command devserver runs the whole notas contract locally (HTTP + WebSocket) with in-memory storage.
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"flag"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/controllers"
	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/repositories"
	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/services"
)

var allowedOrigins = map[string]bool{
	"http://localhost:5173":          true,
	"http://127.0.0.1:5173":          true,
	"https://notas.lauraesteves.com": true,
}

// hub is an in-memory Broadcaster backed by live WebSocket connections.
type hub struct {
	mu    sync.RWMutex
	conns map[string]*websocket.Conn
}

func newHub() *hub { return &hub{conns: map[string]*websocket.Conn{}} }

func (h *hub) add(id string, c *websocket.Conn) {
	h.mu.Lock()
	h.conns[id] = c
	h.mu.Unlock()
}

func (h *hub) remove(id string) {
	h.mu.Lock()
	delete(h.conns, id)
	h.mu.Unlock()
}

func (h *hub) Post(ctx context.Context, id string, payload []byte) error {
	h.mu.RLock()
	c, ok := h.conns[id]
	h.mu.RUnlock()
	if !ok {
		return services.ErrGone
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return c.Write(ctx, websocket.MessageText, payload)
}

func newConnectionID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}

func adapt(h controllers.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20))
		if err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		query := map[string]string{}
		for k, v := range r.URL.Query() {
			if len(v) > 0 {
				query[k] = v[0]
			}
		}
		headers := map[string]string{}
		for k := range r.Header {
			headers[strings.ToLower(k)] = r.Header.Get(k)
		}
		resp := h(r.Context(), controllers.Request{
			Method:  r.Method,
			Path:    r.PathValue("path"),
			Query:   query,
			Headers: headers,
			Body:    string(body),
		})
		for k, v := range resp.Headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(resp.Status)
		_, _ = io.WriteString(w, resp.Body)
	}
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); allowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Connection-Id")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func wsHandler(h *hub, conns services.ConnectionsService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := newConnectionID()
		ctx := context.WithoutCancel(r.Context())
		if err := conns.Connect(ctx, id, r.URL.Query().Get("path")); err != nil {
			status := http.StatusInternalServerError
			switch {
			case errors.Is(err, services.ErrInvalidPath):
				status = http.StatusBadRequest
			case errors.Is(err, services.ErrTooManyConnections):
				status = http.StatusTooManyRequests
			}
			http.Error(w, err.Error(), status)
			return
		}
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			OriginPatterns: []string{"localhost:5173", "127.0.0.1:5173", "notas.lauraesteves.com"},
		})
		if err != nil {
			_ = conns.Disconnect(ctx, id)
			return
		}
		h.add(id, c)
		slog.Info("ws connected", "connectionId", id, "path", r.URL.Query().Get("path"))
		defer func() {
			h.remove(id)
			_ = conns.Disconnect(ctx, id)
			_ = c.CloseNow()
			slog.Info("ws disconnected", "connectionId", id)
		}()
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			if err := conns.HandleMessage(ctx, id, string(data)); err != nil {
				slog.Warn("ws message", "error", err)
			}
		}
	}
}

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()

	notesRepo := repositories.NewMemoryNotesRepository()
	connsRepo := repositories.NewMemoryConnectionsRepository()
	h := newHub()
	broadcast := services.NewBroadcastService(connsRepo, h, nil)
	notes := services.NewNotesService(notesRepo, broadcast, nil)
	conns := services.NewConnectionsService(connsRepo, broadcast, nil)
	ctrl := controllers.NewNotesController(notes)

	mux := http.NewServeMux()
	mux.Handle("GET /notes/{path...}", adapt(ctrl.GetNote))
	mux.Handle("PUT /notes/{path...}", adapt(ctrl.SaveNote))
	mux.Handle("GET /children/{path...}", adapt(ctrl.GetChildren))
	mux.Handle("GET /raw/{path...}", adapt(ctrl.GetRaw))
	mux.Handle("PUT /raw/{path...}", adapt(ctrl.SaveRaw))
	mux.Handle("GET /ws", wsHandler(h, conns))
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, "notas devserver: /notes, /children, /raw, /ws\n")
	})

	slog.Info("notas devserver listening", "addr", *addr)
	if err := http.ListenAndServe(*addr, cors(mux)); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
