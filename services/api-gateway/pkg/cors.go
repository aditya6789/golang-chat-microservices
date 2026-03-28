package pkg

import (
	"github.com/gin-gonic/gin"
)

// CORSMiddleware allows browser-based test clients (e.g. frontend/) to call the gateway.
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-Id, X-User-Id")
		h.Set("Access-Control-Expose-Headers", "X-Request-Id")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
