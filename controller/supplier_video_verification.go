package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/suppliertest"
	"github.com/gin-gonic/gin"
)

func VerifySupplierVideo(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32<<10)
	var input suppliertest.VideoVerificationRequest
	if err := common.DecodeJson(c.Request.Body, &input); err != nil {
		common.ApiError(c, err)
		return
	}
	exchange, err := suppliertest.VerifyVideo(c.Request.Context(), service.GetSSRFProtectedHTTPClient(), input)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, exchange)
}
