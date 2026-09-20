package controller

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/authz"
	"github.com/gin-gonic/gin"
)

func canViewFinancialAccounting(c *gin.Context) bool {
	return authz.Can(c.GetInt("id"), c.GetInt("role"), authz.FinancialAccountingView)
}

func costAccountingFilterFromQuery(c *gin.Context) model.CostAccountingFilter {
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	userID, _ := strconv.Atoi(c.Query("user_id"))
	channelID, _ := strconv.Atoi(c.Query("channel"))
	logType, _ := strconv.Atoi(c.Query("type"))
	return model.CostAccountingFilter{
		StartTimestamp:    startTimestamp,
		EndTimestamp:      endTimestamp,
		UserID:            userID,
		Username:          c.Query("username"),
		TokenName:         c.Query("token_name"),
		ModelName:         c.Query("model_name"),
		ChannelID:         channelID,
		Group:             c.Query("group"),
		LogType:           logType,
		RequestID:         c.Query("request_id"),
		UpstreamRequestID: c.Query("upstream_request_id"),
	}
}

func GetCostAccountingSummary(c *gin.Context) {
	filter := costAccountingFilterFromQuery(c)
	overview, err := service.GetCostAccountingOverview(time.Now(), filter)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, overview)
}

func BackfillCostAccounting(c *gin.Context) {
	var input struct {
		StartTimestamp int64  `json:"start_timestamp"`
		EndTimestamp   int64  `json:"end_timestamp"`
		ChannelID      int    `json:"channel_id"`
		CostDiscount   string `json:"cost_discount"`
		Offset         int    `json:"offset"`
		Limit          int    `json:"limit"`
		Apply          bool   `json:"apply"`
		BatchID        string `json:"batch_id"`
	}
	if err := common.DecodeJson(http.MaxBytesReader(c.Writer, c.Request.Body, 8192), &input); err != nil {
		common.ApiError(c, errors.New("invalid cost accounting backfill request"))
		return
	}
	result, err := service.BackfillCostAccounting(service.CostAccountingBackfillInput{
		StartTimestamp: input.StartTimestamp,
		EndTimestamp:   input.EndTimestamp,
		ChannelID:      input.ChannelID,
		CostDiscount:   input.CostDiscount,
		Offset:         input.Offset,
		Limit:          input.Limit,
		Apply:          input.Apply,
		BatchID:        input.BatchID,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func RepriceCostAccounting(c *gin.Context) {
	var input struct {
		StartTimestamp int64  `json:"start_timestamp"`
		EndTimestamp   int64  `json:"end_timestamp"`
		ChannelID      int    `json:"channel_id"`
		CostDiscount   string `json:"cost_discount"`
		Offset         int    `json:"offset"`
		Limit          int    `json:"limit"`
		Apply          bool   `json:"apply"`
		BatchID        string `json:"batch_id"`
		Reason         string `json:"reason"`
	}
	if err := common.DecodeJson(http.MaxBytesReader(c.Writer, c.Request.Body, 8192), &input); err != nil {
		common.ApiError(c, errors.New("invalid cost accounting reprice request"))
		return
	}
	result, err := service.RepriceCostAccounting(service.CostAccountingRepriceInput{
		StartTimestamp: input.StartTimestamp,
		EndTimestamp:   input.EndTimestamp,
		ChannelID:      input.ChannelID,
		CostDiscount:   input.CostDiscount,
		Offset:         input.Offset,
		Limit:          input.Limit,
		Apply:          input.Apply,
		BatchID:        input.BatchID,
		Reason:         input.Reason,
		ActorID:        c.GetInt("id"),
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}
