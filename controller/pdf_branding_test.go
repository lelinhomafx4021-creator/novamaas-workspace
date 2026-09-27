package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestGetPDFOperatingLogoReturnsNotFoundWhenUnconfigured(t *testing.T) {
	saved := common.OperatingEntityLogo
	common.OperatingEntityLogo = ""
	t.Cleanup(func() { common.OperatingEntityLogo = saved })

	response := httptest.NewRecorder()
	router := gin.New()
	router.GET("/api/pdf-branding/operating-logo", GetPDFOperatingLogo)
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/pdf-branding/operating-logo", nil))

	assert.Equal(t, http.StatusNotFound, response.Code)
}
