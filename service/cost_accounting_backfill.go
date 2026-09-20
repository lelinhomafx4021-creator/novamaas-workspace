package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/shopspring/decimal"
)

type CostAccountingBackfillInput struct {
	StartTimestamp int64
	EndTimestamp   int64
	ChannelID      int
	CostDiscount   string
	Offset         int
	Limit          int
	Apply          bool
	BatchID        string
}

type CostAccountingBackfillResult struct {
	BatchID      string `json:"batch_id"`
	Scanned      int    `json:"scanned"`
	Ready        int    `json:"ready"`
	Existing     int    `json:"existing"`
	Unresolved   int    `json:"unresolved"`
	Applied      int    `json:"applied"`
	NextOffset   int    `json:"next_offset"`
	HasMore      bool   `json:"has_more"`
	CostQuota    int64  `json:"cost_quota"`
	RevenueQuota int64  `json:"revenue_quota"`
}

type CostAccountingRepriceInput struct {
	StartTimestamp int64
	EndTimestamp   int64
	ChannelID      int
	CostDiscount   string
	Offset         int
	Limit          int
	Apply          bool
	BatchID        string
	Reason         string
	ActorID        int
}

type CostAccountingRepriceResult struct {
	BatchID          string `json:"batch_id"`
	Scanned          int    `json:"scanned"`
	Changed          int    `json:"changed"`
	Unchanged        int    `json:"unchanged"`
	Applied          int    `json:"applied"`
	NextOffset       int    `json:"next_offset"`
	HasMore          bool   `json:"has_more"`
	CurrentCostQuota int64  `json:"current_cost_quota"`
	NewCostQuota     int64  `json:"new_cost_quota"`
	DeltaCostQuota   int64  `json:"delta_cost_quota"`
}

func BackfillCostAccounting(input CostAccountingBackfillInput) (*CostAccountingBackfillResult, error) {
	discount, err := model.NormalizeCostDiscount(input.CostDiscount)
	if err != nil {
		return nil, err
	}
	if input.Limit == 0 {
		input.Limit = 1000
	}
	input.BatchID = strings.TrimSpace(input.BatchID)
	if input.ChannelID < 0 || len(input.BatchID) > 64 {
		return nil, errors.New("invalid cost accounting backfill request")
	}
	logs, err := model.ListLogsForCostBackfill(input.StartTimestamp, input.EndTimestamp, input.ChannelID, input.Offset, input.Limit)
	if err != nil {
		return nil, err
	}
	batchID := input.BatchID
	if batchID == "" {
		batchID = common.GetUUID()
	}
	result := &CostAccountingBackfillResult{
		BatchID:    batchID,
		Scanned:    len(logs),
		NextOffset: input.Offset + len(logs),
		HasMore:    len(logs) == input.Limit,
	}
	eventKeys := make([]string, 0, len(logs))
	for i := range logs {
		eventKeys = append(eventKeys, model.CostSnapshotEventKey(&logs[i]))
	}
	existing, err := model.ExistingCostSnapshotEventKeys(eventKeys)
	if err != nil {
		return nil, err
	}
	var discountValue decimal.Decimal
	if discount != "" {
		discountValue = decimal.RequireFromString(discount)
	}
	snapshots := make([]model.CostAccountingSnapshot, 0, len(logs))
	for i := range logs {
		log := &logs[i]
		if _, ok := existing[model.CostSnapshotEventKey(log)]; ok {
			result.Existing++
			continue
		}
		if log.Quota < 0 || log.Quota > common.MaxQuota {
			result.Unresolved++
			continue
		}
		var costBasis decimal.Decimal
		var basisErr error
		if discount == "" {
			costBasis, basisErr = signedSaleQuota(log.Type, log.Quota)
		} else {
			costBasis, basisErr = historicalCostBasis(log)
		}
		if basisErr != nil {
			result.Unresolved++
			continue
		}
		costValue := costBasis
		if discount != "" {
			costValue = costBasis.Mul(discountValue)
		}
		costQuota, clamp := common.QuotaFromDecimalChecked(costValue)
		if clamp != nil {
			return nil, clamp
		}
		snapshot, buildErr := model.BuildCostAccountingSnapshot(log, &model.CostAccountingInput{
			CostBasisQuota: costBasis.StringFixed(6),
			CostDiscount:   discount,
			CostQuota:      int64(costQuota),
			Source:         model.CostSnapshotSourceBackfill,
			BatchID:        batchID,
		})
		if buildErr != nil {
			return nil, buildErr
		}
		if snapshot == nil {
			result.Unresolved++
			continue
		}
		result.Ready++
		result.CostQuota += snapshot.CostQuota
		result.RevenueQuota += snapshot.RevenueQuota
		snapshots = append(snapshots, *snapshot)
	}
	if input.Apply {
		applied, err := model.InsertCostAccountingSnapshots(snapshots)
		if err != nil {
			return nil, err
		}
		result.Applied = int(applied)
	}
	return result, nil
}

func RepriceCostAccounting(input CostAccountingRepriceInput) (*CostAccountingRepriceResult, error) {
	if err := ValidateAccountingRange(input.StartTimestamp, input.EndTimestamp); err != nil {
		return nil, err
	}
	discount, err := model.NormalizeCostDiscount(input.CostDiscount)
	if err != nil {
		return nil, err
	}
	if input.Limit == 0 {
		input.Limit = 500
	}
	input.BatchID = strings.TrimSpace(input.BatchID)
	input.Reason = strings.TrimSpace(input.Reason)
	if input.ChannelID < 0 || len(input.BatchID) > 64 || input.Reason == "" || len(input.Reason) > 500 {
		return nil, errors.New("invalid cost accounting reprice request")
	}
	views, total, err := model.ListCostAccountingSnapshots(model.CostAccountingFilter{
		StartTimestamp: input.StartTimestamp,
		EndTimestamp:   input.EndTimestamp,
		ChannelID:      input.ChannelID,
	}, input.Offset, input.Limit)
	if err != nil {
		return nil, err
	}
	batchID := input.BatchID
	if batchID == "" {
		batchID = common.GetUUID()
	}
	result := &CostAccountingRepriceResult{
		BatchID:    batchID,
		Scanned:    len(views),
		NextOffset: input.Offset + len(views),
		HasMore:    int64(input.Offset+len(views)) < total,
	}
	var discountValue decimal.Decimal
	if discount != "" {
		discountValue = decimal.RequireFromString(discount)
	}
	legacySnapshotLogIDs := make([]int64, 0)
	for _, view := range views {
		if discount != "" && costBasisNeedsSourceLog(view) && view.SourceLogID > 0 {
			legacySnapshotLogIDs = append(legacySnapshotLogIDs, view.SourceLogID)
		}
	}
	legacySnapshotLogs, err := model.GetLogsForCostAccounting(legacySnapshotLogIDs)
	if err != nil {
		return nil, err
	}
	targets := make([]model.CostAccountingAdjustmentTarget, 0, len(views))
	for _, view := range views {
		costBasis, parseErr := decimal.NewFromString(view.CostBasisQuota)
		if parseErr != nil {
			return nil, errors.New("invalid stored cost accounting basis")
		}
		if discount == "" {
			costBasis = decimal.NewFromInt(view.RevenueQuota)
		} else if costBasisNeedsSourceLog(view) {
			log, ok := legacySnapshotLogs[view.SourceLogID]
			if !ok || log.Type != view.LogType || log.Quota < 0 || log.Quota > common.MaxQuota {
				return nil, errors.New("legacy cost snapshot is missing its source log")
			}
			costBasis, parseErr = historicalCostBasis(&log)
			if parseErr != nil {
				return nil, parseErr
			}
		}
		if (view.LogType == model.LogTypeConsume && costBasis.IsNegative()) ||
			(view.LogType == model.LogTypeRefund && costBasis.IsPositive()) {
			return nil, errors.New("stored cost accounting basis has an invalid sign")
		}
		costValue := costBasis
		if discount != "" {
			costValue = costBasis.Mul(discountValue)
		}
		newCost, clamp := common.QuotaFromDecimalChecked(costValue)
		if clamp != nil {
			return nil, clamp
		}
		result.CurrentCostQuota += view.EffectiveCostQuota
		result.NewCostQuota += int64(newCost)
		if int64(newCost) == view.EffectiveCostQuota {
			result.Unchanged++
			continue
		}
		result.Changed++
		targets = append(targets, model.CostAccountingAdjustmentTarget{
			SnapshotID:   view.ID,
			NewCostQuota: int64(newCost),
			CostDiscount: discount,
		})
	}
	result.DeltaCostQuota = result.NewCostQuota - result.CurrentCostQuota
	if input.Apply {
		adjustments, applyErr := model.AdjustCostAccountingSnapshots(targets, input.Reason, input.ActorID, batchID)
		if applyErr != nil {
			return nil, applyErr
		}
		result.Applied = len(adjustments)
	}
	return result, nil
}

func costBasisNeedsSourceLog(view model.CostAccountingSnapshotView) bool {
	return strings.TrimSpace(view.CostDiscount) == "" ||
		view.SnapshotVersion < model.CostAccountingSignedRefundVersion ||
		view.SnapshotVersion == model.CostAccountingLegacyTurnoverBasisVersion
}

func historicalCostBasis(log *model.Log) (decimal.Decimal, error) {
	if log == nil || log.Quota < 0 || log.Quota > common.MaxQuota {
		return decimal.Zero, errors.New("cost accounting log quota is invalid")
	}
	var other struct {
		GroupRatio interface{} `json:"group_ratio"`
	}
	if err := common.UnmarshalJsonStr(log.Other, &other); err != nil {
		return decimal.Zero, errors.New("cost accounting log has invalid billing metadata")
	}
	if other.GroupRatio == nil {
		return decimal.Zero, errors.New("cost accounting log is missing its billing discount")
	}
	billingDiscount, err := decimal.NewFromString(fmt.Sprint(other.GroupRatio))
	if err != nil || billingDiscount.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, errors.New("cost accounting log has an invalid billing discount")
	}
	costBasis, err := originalPriceCostBasis(log.Type, log.Quota, billingDiscount)
	if err != nil {
		return decimal.Zero, err
	}
	return costBasis, nil
}

func ValidateAccountingRange(startTimestamp, endTimestamp int64) error {
	if startTimestamp <= 0 || endTimestamp < startTimestamp {
		return errors.New("invalid accounting time range")
	}
	return nil
}
