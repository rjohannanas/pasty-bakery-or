package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"lingo-backend/internal/ws"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // En producción, restringir a dominios permitidos
	},
}

// WebSocketHandler maneja el upgrade de HTTP a WS y registra al cliente en el Hub.
func WebSocketHandler(hub *ws.Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		hub.RegisterClient(conn)
	}
}
