package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// healthHandler returns 200 OK — used as a Kubernetes liveness probe.
func (s *Server) healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// readyHandler pings all three backends and reports their status.
// Returns 200 only when all dependencies are reachable (readiness probe).
func (s *Server) readyHandler(c *gin.Context) {
	ctx := c.Request.Context()
	components := map[string]string{}
	allOK := true

	if err := s.pg.Ping(ctx); err != nil {
		components["postgres"] = "unhealthy: " + err.Error()
		allOK = false
	} else {
		components["postgres"] = "ok"
	}

	if err := s.mongo.Ping(ctx); err != nil {
		components["mongodb"] = "unhealthy: " + err.Error()
		allOK = false
	} else {
		components["mongodb"] = "ok"
	}

	if err := s.store.Ping(ctx); err != nil {
		components["minio"] = "unhealthy: " + err.Error()
		allOK = false
	} else {
		components["minio"] = "ok"
	}

	status := "ok"
	httpCode := http.StatusOK
	if !allOK {
		status = "degraded"
		httpCode = http.StatusServiceUnavailable
	}

	c.JSON(httpCode, gin.H{
		"status":     status,
		"components": components,
	})
}
