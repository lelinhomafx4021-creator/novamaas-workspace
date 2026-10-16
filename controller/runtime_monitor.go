package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/runtimewatch"
	"github.com/gin-gonic/gin"
)

// GetRuntimeMonitor reads only the node-local in-memory snapshot. Authentication
// is enforced by the existing root-only system-info route group.
func GetRuntimeMonitor(c *gin.Context) {
	minutes := 60
	if raw := c.Query("minutes"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 7*24*60 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "minutes must be between 1 and 10080"})
			return
		}
		minutes = parsed
	}
	c.Header("Cache-Control", "private, no-store")
	common.ApiSuccess(c, runtimewatch.Snapshot(minutes))
}
