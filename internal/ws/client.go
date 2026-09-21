package ws

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
	maxMsgSize = 1024
)

// PublicHost is the hostname the UI is served on (MAILTUB_DOMAIN). Set once at
// startup; the Origin check accepts it in addition to the request's own Host.
var PublicHost string

func hostOnly(h string) string {
	if i := strings.LastIndex(h, ":"); i > 0 && !strings.Contains(h[i:], "]") {
		return h[:i]
	}
	return h
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Same-host only: Origin must match the request Host (port-insensitive),
	// the proxy's X-Forwarded-Host, or the configured public hostname.
	CheckOrigin: func(r *http.Request) bool {
		o := r.Header.Get("Origin")
		if o == "" {
			return true
		}
		u, err := url.Parse(o)
		if err != nil {
			return false
		}
		oh := u.Hostname()
		for _, h := range []string{hostOnly(r.Host), hostOnly(r.Header.Get("X-Forwarded-Host")), PublicHost} {
			if h != "" && strings.EqualFold(oh, h) {
				return true
			}
		}
		slog.Warn("ws: origin rejected", "origin", o, "host", r.Host, "forwarded_host", r.Header.Get("X-Forwarded-Host"))
		return false
	},
}

// Client is a single WebSocket connection managed by the Hub.
type Client struct {
	hub   *Hub
	conn  *websocket.Conn
	send  chan ServerMessage
	allow func(address string) bool
}

// ServeWS upgrades an HTTP request to a WebSocket and begins pumping messages.
// allow decides per subscribe request whether this connection may watch the
// mailbox address (ownership check); nil allows everything.
func ServeWS(hub *Hub, w http.ResponseWriter, r *http.Request, allow func(address string) bool) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("ws: upgrade failed", "error", err)
		return
	}
	c := &Client{hub: hub, conn: conn, send: make(chan ServerMessage, 64), allow: allow}
	hub.Register(c)
	go c.writePump()
	go c.readPump()
}

// readPump listens for messages from the browser (subscription requests).
func (c *Client) readPump() {
	defer func() {
		c.hub.Unregister(c)
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMsgSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		var msg ClientMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		switch msg.Type {
		case "subscribe":
			if msg.Mailbox != "" && (c.allow == nil || c.allow(msg.Mailbox)) {
				c.hub.Subscribe(c, msg.Mailbox)
				c.send <- ServerMessage{Type: EventSubscribed, Mailbox: msg.Mailbox}
			}
		case "ping":
			c.send <- ServerMessage{Type: EventHeartbeat}
		}
	}
}

// writePump forwards queued server messages over the WebSocket.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			data, err := json.Marshal(msg)
			if err != nil {
				slog.Error("ws: marshal error", "error", err)
				continue
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
