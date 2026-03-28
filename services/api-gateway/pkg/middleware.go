package pkg

import (
	"net/http"

	"realtime-chat-system/services/api-gateway/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

func AuthRequired(svc *service.GatewayService) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" {
			if q := c.Query("access_token"); q != "" {
				auth = "Bearer " + q
				c.Request.Header.Set("Authorization", auth)
			}
		}
		if auth == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		claims, err := svc.ValidateClaims(auth)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Request.Header.Set("X-User-Id", claims.Sub)
		rid := c.GetHeader("X-Request-Id")
		if rid == "" {
			rid = uuid.NewString()
		}
		c.Request.Header.Set("X-Request-Id", rid)
		c.Writer.Header().Set("X-Request-Id", rid)
		c.Next()
	}
}

func RateLimit(r rate.Limit, b int) gin.HandlerFunc {
	limiter := rate.NewLimiter(r, b)
	return func(c *gin.Context) {
		if !limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}

