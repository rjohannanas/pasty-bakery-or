package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"
)

// APIKeyAuth exige el header X-API-Key en cada request del grupo protegido.
// No hay usuarios ni roles: un solo secreto compartido entre el back y el front,
// pensado solo para que la API no quede abierta a cualquiera que llegue al dominio.
func APIKeyAuth(expectedKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		provided := c.GetHeader("X-API-Key")
		if subtle.ConstantTimeCompare([]byte(provided), []byte(expectedKey)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "API key inválida o ausente"})
			return
		}
		c.Next()
	}
}

// APIKeyAuthWS es la misma validación pero vía query param, porque el WebSocket
// del navegador no permite mandar headers custom en el handshake.
func APIKeyAuthWS(expectedKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		provided := c.Query("api_key")
		if subtle.ConstantTimeCompare([]byte(provided), []byte(expectedKey)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "API key inválida o ausente"})
			return
		}
		c.Next()
	}
}
