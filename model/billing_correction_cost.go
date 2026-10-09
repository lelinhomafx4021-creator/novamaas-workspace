package model

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"sort"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/seedancepricing"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Both the amount and the pricing basis are append-only. Cumulative before and
// after states make a task's submission and settlement telescope to the rounded
// supplier total, including when a resolution multiplier is fractional.
type billingCorrectionCost struct {
	Version             int    `json:"version"`
	SnapshotID          int64  `json:"snapshot_id"`
	ChannelID           int    `json:"channel_id"`
	ChannelDiscount     string `json:"channel_discount"`
	EvidenceSHA256      string `json:"evidence_sha256"`
	LatestAdjustmentID  int64  `json:"latest_adjustment_id"`
	StateAdjustmentID   int64  `json:"state_adjustment_id"`
	CurrentQuota        int64  `json:"current_quota"`
	CurrentBasis        string `json:"current_basis"`
	CurrentBefore       string `json:"current_before"`
	CurrentAfter        string `json:"current_after"`
	CurrentDiscount     string `json:"current_discount"`
	CurrentBasisVersion int    `json:"current_basis_version"`
	TargetQuota         int64  `json:"target_quota"`
	TargetBasis         string `json:"target_basis"`
	TargetBefore        string `json:"target_before"`
	TargetAfter         string `json:"target_after"`
	TargetDiscount      string `json:"target_discount"`
	TargetBasisVersion  int    `json:"target_basis_version"`
}

type costAccountingState struct {
	Quota, LatestID, StateID       int64
	Basis, Before, After, Discount string
	BasisVersion                   int
}

func effectiveCostAccountingState(snapshot CostAccountingSnapshot, adjustments []CostAccountingAdjustment) (costAccountingState, error) {
	state := costAccountingState{Quota: snapshot.CostQuota, Basis: snapshot.CostBasisQuota, Discount: snapshot.CostDiscount, BasisVersion: snapshot.SnapshotVersion}
	if state.Quota > int64(common.MaxQuota) || state.Quota < -int64(common.MaxQuota) {
		return state, ErrBillingEvidenceIntegrity
	}
	for _, adjustment := range adjustments {
		if adjustment.SnapshotID != snapshot.ID || adjustment.ID <= state.LatestID {
			return state, ErrBillingEvidenceIntegrity
		}
		if adjustment.DeltaCostQuota > int64(common.MaxQuota)-state.Quota || adjustment.DeltaCostQuota < -int64(common.MaxQuota)-state.Quota {
			return state, ErrBillingEvidenceIntegrity
		}
		state.Quota += adjustment.DeltaCostQuota
		state.LatestID, state.StateID = adjustment.ID, adjustment.ID
		if adjustment.Action == "reverse" {
			state.StateID = adjustment.PreviousAdjustmentID
		}
		if adjustment.BasisVersion > 0 || adjustment.Action == "reverse" {
			state.Basis, state.Before, state.After = adjustment.CostBasisQuota, adjustment.CostBasisBefore, adjustment.CostBasisAfter
			state.BasisVersion, state.Discount = adjustment.BasisVersion, adjustment.CostDiscount
		} else if adjustment.CostDiscount != "" {
			// Historical discount-only adjustments predate explicit basis fields.
			state.Discount = adjustment.CostDiscount
		}
	}
	return state, nil
}

func attachCostAccountingBasis(scope *gorm.DB, views []CostAccountingSnapshotView) error {
	ids := make([]int64, 0, len(views))
	for _, view := range views {
		ids = append(ids, view.ID)
	}
	if len(ids) == 0 {
		return nil
	}
	var adjustments []CostAccountingAdjustment
	if err := scope.Where("snapshot_id IN ?", ids).Order("id asc").Find(&adjustments).Error; err != nil {
		return err
	}
	bySnapshot := make(map[int64][]CostAccountingAdjustment)
	for _, adjustment := range adjustments {
		bySnapshot[adjustment.SnapshotID] = append(bySnapshot[adjustment.SnapshotID], adjustment)
	}
	for i := range views {
		view := &views[i]
		state, err := effectiveCostAccountingState(view.CostAccountingSnapshot, bySnapshot[view.ID])
		if err != nil {
			return err
		}
		view.EffectiveCostBasisQuota, view.EffectiveCostBasisBefore, view.EffectiveCostBasisAfter = state.Basis, state.Before, state.After
		view.EffectiveCostDiscount, view.EffectiveBasisVersion, view.LatestAdjustmentID = state.Discount, state.BasisVersion, state.LatestID
		view.EffectiveCostQuota, view.AdjustmentQuota = state.Quota, state.Quota-view.CostQuota
	}
	return nil
}

// Reprice callers share the exact cumulative rounding used by model corrections.
func RepriceCostAccountingBasis(view CostAccountingSnapshotView, discount string) (int64, error) {
	return costAccountingBasisQuota(view.EffectiveCostBasisQuota, view.EffectiveCostBasisBefore, view.EffectiveCostBasisAfter, discount)
}

func costAccountingBasisQuota(basis, before, after, discount string) (int64, error) {
	value, err := decimal.NewFromString(basis)
	if err != nil {
		return 0, ErrBillingEvidenceIntegrity
	}
	multiplier := decimal.NewFromInt(1)
	if discount != "" {
		normalized, err := NormalizeCostDiscount(discount)
		if err != nil {
			return 0, err
		}
		multiplier = decimal.RequireFromString(normalized)
	}
	if before != "" || after != "" {
		previous, err := decimal.NewFromString(before)
		if err != nil || previous.IsNegative() {
			return 0, ErrBillingEvidenceIntegrity
		}
		current, err := decimal.NewFromString(after)
		if err != nil || current.IsNegative() || !current.Sub(previous).Equal(value) {
			return 0, ErrBillingEvidenceIntegrity
		}
		previousQuota, clamp := common.QuotaFromDecimalChecked(previous.Mul(multiplier))
		if clamp != nil {
			return 0, clamp
		}
		currentQuota, clamp := common.QuotaFromDecimalChecked(current.Mul(multiplier))
		if clamp != nil {
			return 0, clamp
		}
		return int64(currentQuota) - int64(previousQuota), nil
	}
	quota, clamp := common.QuotaFromDecimalChecked(value.Mul(multiplier))
	if clamp != nil {
		return 0, clamp
	}
	return int64(quota), nil
}

func billingCorrectionCostBasis(pricing billingCorrectionPricing, modelName string, settled bool) (decimal.Decimal, error) {
	if pricing.ModelRatio < 0 || math.IsNaN(pricing.ModelRatio) || math.IsInf(pricing.ModelRatio, 0) {
		return decimal.Zero, ErrBillingCorrectionBlocked
	}
	var value decimal.Decimal
	if settled && !pricing.PerCallBilling && pricing.PricingMode == "tokens" {
		if pricing.TotalTokens <= 0 || pricing.TotalTokens > int64(common.MaxQuota) {
			return decimal.Zero, ErrBillingCorrectionBlocked
		}
		value = decimal.NewFromInt(pricing.TotalTokens).Mul(decimal.NewFromFloat(pricing.ModelRatio))
	} else if pricing.PricingMode == "tokens" {
		value = decimal.NewFromFloat(pricing.ModelRatio).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).Div(decimal.NewFromInt(2))
	} else {
		if pricing.ModelPrice < 0 || math.IsNaN(pricing.ModelPrice) || math.IsInf(pricing.ModelPrice, 0) {
			return decimal.Zero, ErrBillingCorrectionBlocked
		}
		value = decimal.NewFromFloat(pricing.ModelPrice).Mul(decimal.NewFromFloat(common.QuotaPerUnit))
	}
	keys := make([]string, 0, len(pricing.OtherRatios))
	for key := range pricing.OtherRatios {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		ratio := pricing.OtherRatios[key]
		if ratio <= 0 || math.IsNaN(ratio) || math.IsInf(ratio, 0) {
			return decimal.Zero, ErrBillingCorrectionBlocked
		}
		if key == "video_input" && pricing.HasVideo != nil {
			if price, ok := seedancepricing.Lookup(modelName, pricing.Resolution, *pricing.HasVideo); ok {
				// Recover the exact published fractional tier rather than multiplying
				// by a repeating float saved in the customer pricing parameters.
				value = value.Mul(decimal.NewFromFloat(price.CNYPerMillionTokens)).Div(decimal.NewFromFloat(price.BaseCNYPerMillionTokens))
				continue
			}
		}
		value = value.Mul(decimal.NewFromFloat(ratio))
	}
	if value.IsNegative() || value.GreaterThan(decimal.NewFromInt(int64(common.MaxWalletQuota))) {
		return decimal.Zero, ErrBillingCorrectionBlocked
	}
	return value, nil
}

// Cost evidence is queried through the calling transaction. Log evidence may
// live in a separate database; when it shares the main database, reuse scope to
// avoid waiting on the transaction's own single SQLite connection.
func billingCorrectionCostEvidence(scope *gorm.DB, batch *BillingCorrection, rows []BillingCorrectionRow) ([]BillingCorrectionRow, error) {
	entryIDs, taskIDs, channelIDs := make([]int64, 0, len(rows)), make([]string, 0, len(rows)), make([]int, 0, len(rows))
	for _, row := range rows {
		entryIDs = append(entryIDs, row.SourceEntryID)
		taskIDs = append(taskIDs, row.TaskID)
		channelIDs = append(channelIDs, row.ChannelID)
	}
	var entries []BillingEntry
	if err := scope.Session(&gorm.Session{NewDB: true}).Where("id IN ? AND user_id = ?", entryIDs, batch.UserID).Order("sequence asc").Find(&entries).Error; err != nil {
		return nil, err
	}
	var tasks []Task
	if err := scope.Session(&gorm.Session{NewDB: true}).Where("task_id IN ? AND user_id = ?", taskIDs, batch.UserID).Find(&tasks).Error; err != nil {
		return nil, err
	}
	var channels []Channel
	if err := lockForUpdate(scope.Session(&gorm.Session{NewDB: true})).Select("id", "cost_discount").Where("id IN ?", channelIDs).Order("id asc").Find(&channels).Error; err != nil {
		return nil, err
	}
	byChannel := make(map[int]string, len(channels))
	for _, channel := range channels {
		discount, err := NormalizeCostDiscount(channel.CostDiscount)
		if err != nil {
			return nil, err
		}
		byChannel[channel.Id] = discount
	}
	byEntry, byTask := make(map[int64]BillingEntry, len(entries)), make(map[string]Task, len(tasks))
	for _, entry := range entries {
		byEntry[entry.ID] = entry
	}
	for _, task := range tasks {
		byTask[task.TaskID] = task
	}
	logIDs := make(map[int64]int)
	var missing []int
	for i, row := range rows {
		if row.Blocked != "" {
			continue
		}
		if entry, ok := byEntry[row.SourceEntryID]; ok && entry.SourceLogID > 0 {
			logIDs[row.SourceEntryID] = entry.SourceLogID
		} else {
			missing = append(missing, i)
		}
	}
	logsDB := LOG_DB
	if LOG_DB == DB {
		logsDB = scope.Session(&gorm.Session{NewDB: true})
	}
	for offset := 0; offset < len(missing); offset += 100 {
		indices := missing[offset:min(offset+100, len(missing))]
		query := logsDB.Model(&Log{}).Where("user_id = ? AND type IN ?", batch.UserID, []int{LogTypeConsume, LogTypeRefund})
		conditions := scope.Session(&gorm.Session{NewDB: true}).Where("1 = 0")
		for _, i := range indices {
			row := rows[i]
			conditions = conditions.Or("model_name = ? AND channel_id = ? AND token_id = ? AND created_at >= ? AND created_at <= ?", row.ModelName, row.ChannelID, row.TokenID, row.PostedAt-60, row.PostedAt+60)
		}
		var logs []Log
		if err := query.Where(conditions).Select("id", "user_id", "type", "model_name", "channel_id", "token_id", "created_at", "quota", "request_id", "other").Find(&logs).Error; err != nil {
			return nil, err
		}
		for _, i := range indices {
			row := rows[i]
			for _, log := range logs {
				signed := int64(log.Quota)
				if log.Type == LogTypeRefund {
					signed = -signed
				}
				var metadata struct {
					TaskID string `json:"task_id"`
				}
				if log.ModelName != row.ModelName || log.ChannelId != row.ChannelID || log.TokenId != row.TokenID || signed != row.OriginalQuota || log.CreatedAt < row.PostedAt-60 || log.CreatedAt > row.PostedAt+60 || common.UnmarshalJsonStr(log.Other, &metadata) != nil || metadata.TaskID != row.TaskID {
					continue
				}
				if logIDs[row.SourceEntryID] > 0 {
					return nil, ErrBillingEvidenceIntegrity
				}
				logIDs[row.SourceEntryID] = log.Id
			}
		}
	}
	ids := make([]int, 0, len(logIDs))
	for _, id := range logIDs {
		ids = append(ids, id)
	}
	var snapshots []CostAccountingSnapshot
	if len(ids) > 0 {
		if err := lockForUpdate(scope.Session(&gorm.Session{NewDB: true})).Where("user_id = ? AND source_log_id IN ? AND log_type IN ?", batch.UserID, ids, []int{LogTypeConsume, LogTypeRefund}).Order("id asc").Find(&snapshots).Error; err != nil {
			return nil, err
		}
	}
	byLog := make(map[int64]CostAccountingSnapshot, len(snapshots))
	snapshotIDs := make([]int64, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if _, found := byLog[snapshot.SourceLogID]; found {
			continue
		}
		byLog[snapshot.SourceLogID] = snapshot
		snapshotIDs = append(snapshotIDs, snapshot.ID)
	}
	var adjustments []CostAccountingAdjustment
	if len(snapshotIDs) > 0 {
		if err := scope.Session(&gorm.Session{NewDB: true}).Where("snapshot_id IN ?", snapshotIDs).Order("id asc").Find(&adjustments).Error; err != nil {
			return nil, err
		}
	}
	bySnapshot := make(map[int64][]CostAccountingAdjustment)
	for _, adjustment := range adjustments {
		bySnapshot[adjustment.SnapshotID] = append(bySnapshot[adjustment.SnapshotID], adjustment)
	}
	lifecycles := make(map[string][]BillingEntry)
	rowTask := make(map[int64]string, len(rows))
	for _, row := range rows {
		rowTask[row.SourceEntryID] = row.TaskID
	}
	for _, entry := range entries {
		lifecycles[rowTask[entry.ID]] = append(lifecycles[rowTask[entry.ID]], entry)
	}
	for i := range rows {
		row := &rows[i]
		if row.Blocked != "" {
			continue
		}
		snapshot, found := byLog[int64(logIDs[row.SourceEntryID])]
		channelDiscount, channelFound := byChannel[row.ChannelID]
		if !found {
			if channelDiscount != "" || !channelFound {
				row.CostBlocked, row.Blocked = "missing_cost_evidence", "missing_cost_evidence"
			}
			continue
		}
		if snapshot.UserID != batch.UserID || snapshot.ModelName != row.ModelName || snapshot.ChannelID != row.ChannelID || snapshot.RevenueQuota != row.OriginalQuota {
			return nil, ErrBillingEvidenceIntegrity
		}
		state, err := effectiveCostAccountingState(snapshot, bySnapshot[snapshot.ID])
		if err != nil {
			return nil, err
		}
		discount := channelDiscount
		if discount == "" {
			discount = state.Discount
		}
		// A channel with no configured or historical supplier discount retains
		// its existing break-even policy; do not invent a wholesale price.
		if discount == "" {
			continue
		}
		basis, before, after, version := state.Basis, state.Before, state.After, state.BasisVersion
		if batch.Mode == BillingCorrectionModelPricing {
			var pricing billingCorrectionPricing
			task, found := byTask[row.TaskID]
			if !found || task.PrivateData.BillingContext == nil || common.UnmarshalJsonStr(row.TargetPricing, &pricing) != nil {
				row.CostBlocked, row.Blocked = "missing_cost_evidence", "missing_cost_evidence"
				continue
			}
			oldPricing, err := billingCorrectionTaskPricing(&task, false, billingCorrectionVideoRequest{})
			if err != nil {
				return nil, err
			}
			oldSubmission, err := billingCorrectionPricingQuota(oldPricing, task.PrivateData.BillingContext.GroupRatio, false)
			if err != nil {
				return nil, err
			}
			pre, err := billingCorrectionCostBasis(pricing, row.ModelName, false)
			if err != nil {
				return nil, err
			}
			final := decimal.Zero
			if task.Status == TaskStatusSuccess {
				final, err = billingCorrectionCostBasis(pricing, row.ModelName, true)
				if err != nil {
					return nil, err
				}
			}
			previous, current, cumulative := decimal.Zero, decimal.Zero, int64(0)
			for j, entry := range lifecycles[row.TaskID] {
				cumulative += entry.Quota
				switch {
				case cumulative == 0:
					current = decimal.Zero
				case cumulative == oldSubmission:
					current = pre
				case cumulative == int64(task.Quota):
					current = final
				default:
					return nil, ErrBillingEvidenceIntegrity
				}
				if j == len(lifecycles[row.TaskID])-1 && task.Status == TaskStatusSuccess {
					current = final
				}
				if entry.ID == row.SourceEntryID {
					break
				}
				previous = current
			}
			before, after = previous.String(), current.String()
			basis, version = current.Sub(previous).String(), CostAccountingCorrectedBasisVersion
		} else if version < CostAccountingCorrectedBasisVersion && (snapshot.CostDiscount == "" || snapshot.SnapshotVersion < CostAccountingSignedRefundVersion || snapshot.SnapshotVersion == CostAccountingLegacyTurnoverBasisVersion) {
			// Legacy break-even/turnover evidence was not a supplier list-price
			// basis. Recover its original customer discount, never the target group.
			group, err := decimal.NewFromString(row.OriginalRate)
			if err != nil || !group.IsPositive() {
				return nil, ErrBillingEvidenceIntegrity
			}
			basis, before, after, version = decimal.NewFromInt(row.OriginalQuota).Div(group).String(), "", "", CostAccountingOriginalPriceBasisVersion
		}
		target, err := costAccountingBasisQuota(basis, before, after, discount)
		if err != nil {
			return nil, err
		}
		if len(basis) > 64 || len(before) > 64 || len(after) > 64 {
			row.CostBlocked, row.Blocked = "missing_cost_evidence", "missing_cost_evidence"
			continue
		}
		if (snapshot.LogType == LogTypeConsume && target < 0) || (snapshot.LogType == LogTypeRefund && target > 0) {
			row.CostBlocked, row.Blocked = "cost_direction_changed", "cost_direction_changed"
			continue
		}
		body, err := common.Marshal(struct {
			Snapshot    CostAccountingSnapshot
			Adjustments []CostAccountingAdjustment
		}{snapshot, bySnapshot[snapshot.ID]})
		if err != nil {
			return nil, err
		}
		hash := sha256.Sum256(body)
		frozen := billingCorrectionCost{Version: 1, SnapshotID: snapshot.ID, ChannelID: row.ChannelID, ChannelDiscount: channelDiscount, EvidenceSHA256: hex.EncodeToString(hash[:]), LatestAdjustmentID: state.LatestID, StateAdjustmentID: state.StateID,
			CurrentQuota: state.Quota, CurrentBasis: state.Basis, CurrentBefore: state.Before, CurrentAfter: state.After, CurrentDiscount: state.Discount, CurrentBasisVersion: state.BasisVersion,
			TargetQuota: target, TargetBasis: basis, TargetBefore: before, TargetAfter: after, TargetDiscount: discount, TargetBasisVersion: version}
		body, err = common.Marshal(frozen)
		if err != nil {
			return nil, err
		}
		row.TargetCost, row.CostDiscount = string(body), discount
		currentQuota, targetQuota := state.Quota, target
		row.CurrentCostQuota, row.CorrectedCostQuota, row.CostDelta = &currentQuota, &targetQuota, target-state.Quota
		row.CostChanged = row.CostDelta != 0 || basis != state.Basis || before != state.Before || after != state.After || discount != state.Discount || version != state.BasisVersion
	}
	return rows, nil
}

// Called inside the wallet/ledger transaction. Snapshot locks serialize model
// corrections with ordinary cost repricing. Reversals restore the previous
// basis/discount and active dependency pointer rather than deleting evidence.
func billingCorrectionApplyCosts(tx *gorm.DB, batch *BillingCorrection, actorID int, reverse bool, reason string) error {
	type target struct {
		row    BillingCorrectionRow
		frozen billingCorrectionCost
	}
	targets := make([]target, 0, len(batch.Rows))
	for _, row := range batch.Rows {
		if row.TargetCost == "" || !row.CostChanged {
			continue
		}
		var frozen billingCorrectionCost
		if common.UnmarshalJsonStr(row.TargetCost, &frozen) != nil || frozen.Version != 1 || frozen.SnapshotID <= 0 {
			return ErrBillingConflict
		}
		targets = append(targets, target{row, frozen})
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].frozen.SnapshotID < targets[j].frozen.SnapshotID })
	for i, target := range targets {
		if i > 0 && targets[i-1].frozen.SnapshotID == target.frozen.SnapshotID {
			return ErrBillingEvidenceIntegrity
		}
		var snapshot CostAccountingSnapshot
		if err := lockForUpdate(tx).First(&snapshot, target.frozen.SnapshotID).Error; err != nil {
			return err
		}
		if snapshot.UserID != batch.UserID || snapshot.ModelName != target.row.ModelName || snapshot.ChannelID != target.row.ChannelID || snapshot.RevenueQuota != target.row.OriginalQuota {
			return ErrBillingConflict
		}
		var adjustments []CostAccountingAdjustment
		if err := tx.Where("snapshot_id = ?", snapshot.ID).Order("id asc").Find(&adjustments).Error; err != nil {
			return err
		}
		if len(adjustments) == 0 {
			// Match the preview's absent per-snapshot adjustment slice.
			adjustments = nil
		}
		state, err := effectiveCostAccountingState(snapshot, adjustments)
		if err != nil {
			return err
		}
		action, quota := "apply", target.frozen.TargetQuota
		basis, before, after, discount, version := target.frozen.TargetBasis, target.frozen.TargetBefore, target.frozen.TargetAfter, target.frozen.TargetDiscount, target.frozen.TargetBasisVersion
		if reverse {
			action = "reverse"
			var applied *CostAccountingAdjustment
			for j := range adjustments {
				if adjustments[j].CorrectionID == batch.ID && adjustments[j].Action == "apply" {
					applied = &adjustments[j]
				}
			}
			if applied == nil || state.StateID != applied.ID || state.Quota != target.frozen.TargetQuota {
				return ErrBillingCorrectionDependency
			}
			quota, basis, before, after, discount, version = target.frozen.CurrentQuota, target.frozen.CurrentBasis, target.frozen.CurrentBefore, target.frozen.CurrentAfter, target.frozen.CurrentDiscount, target.frozen.CurrentBasisVersion
		} else {
			body, err := common.Marshal(struct {
				Snapshot    CostAccountingSnapshot
				Adjustments []CostAccountingAdjustment
			}{snapshot, adjustments})
			if err != nil {
				return err
			}
			hash := sha256.Sum256(body)
			if hex.EncodeToString(hash[:]) != target.frozen.EvidenceSHA256 || state.LatestID != target.frozen.LatestAdjustmentID {
				return ErrBillingConflict
			}
			var channel Channel
			if err := lockForUpdate(tx).Select("id", "cost_discount").First(&channel, target.row.ChannelID).Error; err != nil {
				return err
			}
			currentDiscount, err := NormalizeCostDiscount(channel.CostDiscount)
			if err != nil || currentDiscount != target.frozen.ChannelDiscount {
				return ErrBillingConflict
			}
		}
		eventKey := "billing-cost:" + action + ":" + batch.ID + ":" + strconv.FormatInt(snapshot.ID, 10)
		costReason := batch.Reason
		if reverse {
			costReason = reason
		}
		// The full explanation remains on the correction batch. The existing
		// cost audit column permits 500 characters on every supported database.
		if len([]rune(costReason)) > 500 {
			costReason = string([]rune(costReason)[:500])
		}
		adjustment := CostAccountingAdjustment{EventKey: &eventKey, SnapshotID: snapshot.ID, DeltaCostQuota: quota - state.Quota, NewCostQuota: quota,
			CostBasisQuota: basis, CostBasisBefore: before, CostBasisAfter: after, CostDiscount: discount, BasisVersion: version,
			CorrectionID: batch.ID, Action: action, PreviousAdjustmentID: target.frozen.StateAdjustmentID, ActorID: actorID, BatchID: batch.ID, CreatedAt: common.GetTimestamp(), Reason: costReason}
		if err := tx.Create(&adjustment).Error; err != nil {
			return err
		}
	}
	return nil
}
