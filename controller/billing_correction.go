package controller

import (
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func billingCorrectionRoot(c *gin.Context) bool {
	if c.GetInt("role") != common.RoleRootUser {
		c.AbortWithStatus(http.StatusForbidden)
		return false
	}
	return true
}

func BillingCorrectionGroups(c *gin.Context) {
	if !billingCorrectionRoot(c) {
		return
	}
	id, ok := billingUserID(c)
	if !ok {
		return
	}
	groups, err := service.BillingCorrectionGroups(id)
	if err != nil {
		billingError(c, err)
		return
	}
	common.ApiSuccess(c, groups)
}

func PreviewBillingCorrection(c *gin.Context) {
	if !billingCorrectionRoot(c) {
		return
	}
	if _, ok := middleware.GetSessionAuthIdentity(c); !ok {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	var input model.BillingCorrectionInput
	if err := common.DecodeJson(http.MaxBytesReader(c.Writer, c.Request.Body, 16384), &input); err != nil {
		billingError(c, errors.New("invalid billing correction"))
		return
	}
	batch, err := service.PreviewBillingCorrection(input, c.GetInt("id"))
	if err != nil {
		billingError(c, err)
		return
	}
	common.ApiSuccess(c, batch)
}

func ListBillingCorrections(c *gin.Context) {
	if !billingCorrectionRoot(c) {
		return
	}
	id, ok := billingUserID(c)
	if !ok {
		return
	}
	var batches []model.BillingCorrection
	if err := model.DB.Where("user_id = ?", id).Order("created_at desc, id desc").Limit(30).Find(&batches).Error; err != nil {
		billingError(c, err)
		return
	}
	common.ApiSuccess(c, batches)
}

func GetBillingCorrection(c *gin.Context) {
	if !billingCorrectionRoot(c) {
		return
	}
	batch, err := model.GetBillingCorrection(c.Param("correction_id"))
	if err != nil {
		billingError(c, err)
		return
	}
	common.ApiSuccess(c, batch)
}

func ActOnBillingCorrection(c *gin.Context) {
	if !billingCorrectionRoot(c) {
		return
	}
	if !middleware.RequireSecurityProof(c, "billing.correct", []string{"2fa", "passkey"}) {
		return
	}
	var input struct {
		Action          string `json:"action"`
		SHA256          string `json:"sha256"`
		Reason          string `json:"reason"`
		ConfirmUserID   int    `json:"confirm_user_id"`
		ConfirmNetDelta *int64 `json:"confirm_net_delta"`
	}
	if err := common.DecodeJson(http.MaxBytesReader(c.Writer, c.Request.Body, 4096), &input); err != nil || (input.Action != "apply" && input.Action != "reverse") {
		billingError(c, errors.New("invalid correction action"))
		return
	}
	existing, err := model.GetBillingCorrection(c.Param("correction_id"))
	if err != nil {
		billingError(c, err)
		return
	}
	expected := existing.NetDelta
	if input.Action == "reverse" {
		expected = -expected
	}
	if input.ConfirmUserID != existing.UserID || input.ConfirmNetDelta == nil || *input.ConfirmNetDelta != expected {
		billingError(c, model.ErrBillingConflict)
		return
	}
	batch, err := service.ApplyBillingCorrection(c.Param("correction_id"), input.SHA256, c.GetInt("id"), input.Action == "reverse", input.Reason)
	if err != nil {
		billingError(c, err)
		return
	}
	common.ApiSuccess(c, batch)
}
