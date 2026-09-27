package controller

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func GetPDFOperatingLogo(c *gin.Context) {
	logo, err := service.FetchPDFOperatingLogo(c.Request.Context())
	if errors.Is(err, service.ErrBillingOperatingLogoUnconfigured) {
		c.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("PDF operating entity logo fetch failed: %v", err))
		c.Status(http.StatusBadGateway)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "image/png", logo)
}
