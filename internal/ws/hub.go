package ws

import (
	"sync"
	"github.com/gorilla/websocket"
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

	// Goroutine para escribir mensajes al cliente
	go func() {
		defer func() {
			h.unregister <- client
			conn.Close()
		}()
		for message := range client.send {
			err := conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				break
			}
		}
	}()
}
