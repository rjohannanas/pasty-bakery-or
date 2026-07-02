package ws

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

// Client representa una conexión WebSocket activa.
type Client struct {
	conn *websocket.Conn
	send chan []byte
}

// Hub gestiona el conjunto de clientes y el broadcast de mensajes.
type Hub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	mu         sync.Mutex
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte),
	}
}

// Run inicia el bucle de eventos del Hub.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
		case message := <-h.broadcast:
			h.mu.Lock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.Unlock()
		}
	}
}

// Broadcast envía un mensaje a todos los clientes conectados de manera no bloqueante.
func (h *Hub) Broadcast(message []byte) {
	select {
	case h.broadcast <- message:
	default:
		// Mensaje descartado si el Hub está saturado
	}
}

// Funciones para que el Handler pueda registrar clientes
func (h *Hub) RegisterClient(conn *websocket.Conn) {
	client := &Client{conn: conn, send: make(chan []byte, 256)}
	h.register <- client

	go client.writePump(h)
	go client.readPump(h)
}

// writePump escribe mensajes salientes y manda pings periódicos para
// detectar conexiones muertas (el navegador responde el pong solo).
func (c *Client) writePump(h *Hub) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		h.unregister <- c
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readPump solo existe para detectar cierre/timeout del cliente; el front
// no manda comandos que el backend necesite interpretar.
func (c *Client) readPump(h *Hub) {
	defer func() {
		h.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}
