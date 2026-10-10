package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"message": "GoSource-Insight API is running",
	})
}

type Readiness interface{ Ready(context.Context) error }

// Liveness has no external dependencies; readiness checks only required API
// state. Optional AI connectivity is checked when index/ask is requested.
func (h *SourceHandler) Ready(c *gin.Context) {
	ready, ok := h.Service.(Readiness)
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	if err := ready.Ready(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}
