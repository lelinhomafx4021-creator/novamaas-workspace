package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/seedancepricing"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"gorm.io/gorm"
)

const (
	BillingCorrectionGroupRate    = "group_rate"
	BillingCorrectionModelPricing = "model_pricing"
)

// A preview freezes both the correction definition and its original evidence.
// Corrections append new ledger entries; source entries and tasks stay intact.
type BillingCorrection struct {
	ID                    string                 `json:"id" gorm:"type:varchar(64);primaryKey"`
	UserID                int                    `json:"user_id" gorm:"index"`
	UserGroup             string                 `json:"user_group" gorm:"type:varchar(64)"`
	CreatedBy             int                    `json:"created_by"`
	CreatedAt             int64                  `json:"created_at" gorm:"bigint"`
	ExpiresAt             int64                  `json:"expires_at" gorm:"bigint"`
	StartAt               int64                  `json:"start_at" gorm:"bigint"`
	EndAt                 int64                  `json:"end_at" gorm:"bigint"`
	Models                string                 `json:"models" gorm:"type:text"`
	Mode                  string                 `json:"mode,omitempty" gorm:"type:varchar(32)"`
	TargetGroup           string                 `json:"target_group" gorm:"type:varchar(50)"`
	TargetRate            string                 `json:"target_rate" gorm:"type:varchar(64)"`
	Reason                string                 `json:"reason" gorm:"type:varchar(1000)"`
	Status                string                 `json:"status" gorm:"type:varchar(16)"`
	SourceSequence        int64                  `json:"source_sequence" gorm:"bigint"`
	CanApply              bool                   `json:"can_apply"`
	ChargeDelta           int64                  `json:"charge_delta" gorm:"bigint"`
	RefundDelta           int64                  `json:"refund_delta" gorm:"bigint"`
	NetDelta              int64                  `json:"net_delta" gorm:"bigint"`
	CurrentCostQuota      *int64                 `json:"current_cost_quota,omitempty" gorm:"bigint"`
	CorrectedCostQuota    *int64                 `json:"corrected_cost_quota,omitempty" gorm:"bigint"`
	CostDelta             int64                  `json:"cost_delta,omitempty" gorm:"bigint"`
	SHA256                string                 `json:"sha256" gorm:"type:char(64)"`
	AppliedAt             int64                  `json:"applied_at" gorm:"bigint"`
	ReversedAt            int64                  `json:"reversed_at" gorm:"bigint"`
	ReversalReason        string                 `json:"reversal_reason" gorm:"type:varchar(1000)"`
	AuditLoggedAt         int64                  `json:"audit_logged_at" gorm:"bigint"`
	ReversalAuditLoggedAt int64                  `json:"reversal_audit_logged_at" gorm:"bigint"`
	Rows                  []BillingCorrectionRow `json:"rows" gorm:"-"`
}

type BillingCorrectionRow struct {
	ID                 int64  `json:"-" gorm:"primaryKey"`
	BatchID            string `json:"batch_id" gorm:"type:varchar(64);index"`
	SourceEntryID      int64  `json:"source_entry_id" gorm:"bigint;index"`
	ModelName          string `json:"model_name" gorm:"type:varchar(255)"`
	PostedAt           int64  `json:"posted_at" gorm:"bigint"`
	TaskID             string `json:"task_id" gorm:"type:varchar(191)"`
	TokenID            int    `json:"token_id"`
	ChannelID          int    `json:"channel_id"`
	OriginalGroup      string `json:"original_group" gorm:"type:varchar(50)"`
	OriginalRate       string `json:"original_rate" gorm:"type:varchar(64)"`
	OriginalQuota      int64  `json:"original_quota" gorm:"bigint"`
	CorrectedQuota     int64  `json:"corrected_quota" gorm:"bigint"`
	Delta              int64  `json:"delta" gorm:"bigint"`
	SourceSHA256       string `json:"source_sha256" gorm:"type:char(64)"`
	Blocked            string `json:"blocked" gorm:"type:varchar(64)"`
	EffectiveQuota     int64  `json:"effective_quota,omitempty" gorm:"bigint"`
	PreviousBatchID    string `json:"previous_batch_id,omitempty" gorm:"type:varchar(64)"`
	TargetGroup        string `json:"target_group,omitempty" gorm:"type:varchar(50)"`
	TargetRate         string `json:"target_rate,omitempty" gorm:"type:varchar(64)"`
	TargetPricing      string `json:"target_pricing,omitempty" gorm:"type:text"`
	EffectiveGroup     string `json:"effective_group,omitempty" gorm:"type:varchar(50)"`
	EffectiveRate      string `json:"effective_rate,omitempty" gorm:"type:varchar(64)"`
	TargetCost         string `json:"target_cost,omitempty" gorm:"type:text"`
	CurrentCostQuota   *int64 `json:"current_cost_quota,omitempty" gorm:"type:bigint"`
	CorrectedCostQuota *int64 `json:"corrected_cost_quota,omitempty" gorm:"type:bigint"`
	CostDelta          int64  `json:"cost_delta,omitempty" gorm:"type:bigint"`
	CostDiscount       string `json:"cost_discount,omitempty" gorm:"type:varchar(16)"`
	CostBlocked        string `json:"cost_blocked,omitempty" gorm:"type:varchar(64)"`
	CostChanged        bool   `json:"cost_changed,omitempty"`
}

// This per-source claim serializes overlapping batches. Reversal restores the
// previous active pointer without deleting any correction or audit history.
type BillingCorrectionClaim struct {
	SourceEntryID int64  `gorm:"primaryKey;autoIncrement:false"`
	BatchID       string `gorm:"type:varchar(64)"`
}

type BillingCorrectionInput struct {
	UserID      int      `json:"user_id"`
	StartAt     int64    `json:"start_at"`
	EndAt       int64    `json:"end_at"`
	Models      []string `json:"models"`
	Mode        string   `json:"mode"`
	TargetGroup string   `json:"target_group"`
	Reason      string   `json:"reason"`
}

var ErrBillingCorrectionBlocked = errors.New("billing correction cannot be safely applied")
var ErrBillingCorrectionDependency = errors.New("reverse later corrections before reversing this batch")

// These parameters are frozen with every preview row. A configuration change
// during review changes the evidence digest and requires a new preview.
type billingCorrectionPricing struct {
	PricingMode         string             `json:"pricing_mode"`
	ModelPrice          float64            `json:"model_price"`
	ModelRatio          float64            `json:"model_ratio"`
	OtherRatios         map[string]float64 `json:"other_ratios"`
	PerCallBilling      bool               `json:"per_call_billing"`
	TotalTokens         int64              `json:"total_tokens,omitempty"`
	Resolution          string             `json:"resolution,omitempty"`
	HasVideo            *bool              `json:"has_video,omitempty"`
	RequestedResolution string             `json:"requested_resolution,omitempty"`
	ResolutionSource    string             `json:"resolution_source,omitempty"`
}

func billingCorrectionPricingQuota(pricing billingCorrectionPricing, groupRatio float64, settled bool) (int64, error) {
	if math.IsNaN(groupRatio) || math.IsInf(groupRatio, 0) || groupRatio < 0 || groupRatio > 10 {
		return 0, ErrBillingCorrectionBlocked
	}
	keys := make([]string, 0, len(pricing.OtherRatios))
	for key, multiplier := range pricing.OtherRatios {
		if multiplier <= 0 || math.IsNaN(multiplier) || math.IsInf(multiplier, 0) {
			return 0, ErrBillingCorrectionBlocked
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	multiplier := 1.0
	for _, key := range keys {
		multiplier *= pricing.OtherRatios[key]
	}
	if settled && !pricing.PerCallBilling && pricing.ModelRatio > 0 {
		if pricing.TotalTokens <= 0 || pricing.TotalTokens > int64(common.MaxQuota) || pricing.ModelRatio < 0 || math.IsNaN(pricing.ModelRatio) || math.IsInf(pricing.ModelRatio, 0) {
			return 0, ErrBillingCorrectionBlocked
		}
		quota, err := common.QuotaFromFloatStrict(float64(pricing.TotalTokens) * pricing.ModelRatio * groupRatio * multiplier)
		return int64(quota), err
	}
	price := pricing.ModelPrice
	if pricing.PricingMode == "tokens" {
		price = pricing.ModelRatio / 2
	}
	if price < 0 || math.IsNaN(price) || math.IsInf(price, 0) {
		return 0, ErrBillingCorrectionBlocked
	}
	// Submission truncates the base fee before applying duration/video ratios;
	// token settlement truncates once. Scaling a rounded historical amount loses
	// that distinction and produces incorrect charges.
	base, err := common.QuotaFromFloatStrict(price * common.QuotaPerUnit * groupRatio)
	if err != nil {
		return 0, err
	}
	quota, err := common.QuotaFromFloatStrict(float64(base) * multiplier)
	return int64(quota), err
}

type billingCorrectionVideoRequest struct {
	Resolution string
	HasVideo   bool
	Valid      bool
}

func billingCorrectionVideoRequestEvidence(body []byte) billingCorrectionVideoRequest {
	type contentItem struct {
		Type     string          `json:"type"`
		VideoURL json.RawMessage `json:"video_url"`
	}
	type videoRequest struct {
		Resolution json.RawMessage `json:"resolution"`
		Content    *[]contentItem  `json:"content"`
	}
	var request struct {
		videoRequest
		Metadata *videoRequest `json:"metadata"`
	}
	if len(body) == 0 || common.Unmarshal(body, &request) != nil {
		return billingCorrectionVideoRequest{}
	}
	data := request.videoRequest
	// Generic video requests put provider options under metadata. Native and
	// materialized upstream requests put them directly at the top level.
	if data.Content == nil && request.Metadata != nil {
		data = *request.Metadata
	}
	if data.Content == nil || len(*data.Content) == 0 {
		return billingCorrectionVideoRequest{}
	}
	evidence := billingCorrectionVideoRequest{Valid: true}
	if len(data.Resolution) > 0 {
		if common.GetJsonType(data.Resolution) != "string" || common.Unmarshal(data.Resolution, &evidence.Resolution) != nil {
			return billingCorrectionVideoRequest{}
		}
		evidence.Resolution = strings.ToLower(strings.TrimSpace(evidence.Resolution))
		if evidence.Resolution == "" {
			return billingCorrectionVideoRequest{}
		}
	}
	for _, item := range *data.Content {
		if item.Type == "video_url" || len(item.VideoURL) > 0 {
			evidence.HasVideo = true
			continue
		}
		if item.Type != "text" && item.Type != "image_url" && item.Type != "audio_url" {
			return billingCorrectionVideoRequest{}
		}
	}
	return evidence
}

func billingCorrectionTaskRequests(scope *gorm.DB, tasks []Task) (map[string]billingCorrectionVideoRequest, error) {
	result := make(map[string]billingCorrectionVideoRequest)
	references := make([]string, 0, len(tasks)*2)
	originalRefs, upstreamRefs := make(map[string]string), make(map[string]string)
	for _, task := range tasks {
		if bc := task.PrivateData.BillingContext; bc == nil {
			continue
		} else if _, known := seedancepricing.BasePriceCNY(bc.OriginModelName); !known {
			continue
		}
		result[task.TaskID] = billingCorrectionVideoRequestEvidence(task.Properties.RequestBody)
		original, err := taskRequestBodyReference(task.TaskID, "")
		if err != nil {
			return nil, err
		}
		upstream, err := upstreamRequestBodyReference(task.TaskID, "")
		if err != nil {
			return nil, err
		}
		originalRefs[original], upstreamRefs[upstream] = task.TaskID, task.TaskID
		references = append(references, original, upstream)
	}
	if len(references) == 0 {
		return result, nil
	}
	rows, err := scope.Session(&gorm.Session{NewDB: true}).Model(&TaskRequestBody{}).Select("reference_id", "body").Where("reference_id IN ?", references).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	originals, upstreams := make(map[string]billingCorrectionVideoRequest), make(map[string]billingCorrectionVideoRequest)
	// Decode one payload at a time so up to 1,000 tasks do not retain several
	// gigabytes of archived media request JSON during a correction transaction.
	for rows.Next() {
		var record TaskRequestBody
		if err := scope.ScanRows(rows, &record); err != nil {
			return nil, err
		}
		evidence := billingCorrectionVideoRequestEvidence(record.Body)
		if id, ok := upstreamRefs[record.ReferenceID]; ok {
			upstreams[id] = evidence
		} else if id, ok := originalRefs[record.ReferenceID]; ok {
			originals[id] = evidence
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for id, evidence := range originals {
		result[id] = evidence
	}
	for id, evidence := range upstreams {
		result[id] = evidence
	}
	return result, nil
}

func billingCorrectionTaskPricing(task *Task, current bool, request billingCorrectionVideoRequest) (billingCorrectionPricing, error) {
	bc := task.PrivateData.BillingContext
	if bc == nil {
		return billingCorrectionPricing{}, ErrBillingCorrectionBlocked
	}
	pricing := billingCorrectionPricing{PricingMode: "per_call", ModelPrice: bc.ModelPrice, ModelRatio: bc.ModelRatio, PerCallBilling: bc.PerCallBilling, OtherRatios: make(map[string]float64, len(bc.OtherRatios))}
	if bc.ModelPrice < 0 && bc.ModelRatio >= 0 {
		pricing.PricingMode = "tokens"
	}
	for key, value := range bc.OtherRatios {
		pricing.OtherRatios[key] = value
	}
	var result struct {
		// Only known Seedance repricing interprets this field. Other providers
		// may return numeric/object resolution metadata unrelated to billing.
		Resolution json.RawMessage `json:"resolution"`
		Usage      struct {
			TotalTokens int64 `json:"total_tokens"`
		} `json:"usage"`
	}
	if len(task.Data) > 0 && common.Unmarshal(task.Data, &result) != nil {
		return pricing, ErrBillingCorrectionBlocked
	}
	pricing.TotalTokens = result.Usage.TotalTokens
	if !current {
		return pricing, nil
	}
	if billing_setting.GetBillingMode(bc.OriginModelName) == billing_setting.BillingModeTieredExpr {
		return pricing, ErrBillingCorrectionBlocked
	}
	price, hasPrice := ratio_setting.GetModelPrice(bc.OriginModelName, false)
	if !hasPrice {
		price, hasPrice = ratio_setting.GetDefaultModelPriceMap()[bc.OriginModelName]
	}
	ratio, hasRatio, _ := ratio_setting.GetModelRatio(bc.OriginModelName)
	if hasPrice {
		pricing.PricingMode, pricing.ModelPrice, pricing.ModelRatio = "per_call", price, 0
		if hasRatio {
			pricing.ModelRatio = ratio
		}
	} else if hasRatio {
		pricing.PricingMode, pricing.ModelPrice, pricing.ModelRatio = "tokens", -1, ratio
	} else {
		return pricing, ErrBillingCorrectionBlocked
	}
	// Billing mode follows today's submission policy. A legacy fixed-price
	// PerCallBilling flag must not suppress token settlement after a price-to-
	// ratio change, and switching to a fixed price must suppress settlement.
	pricing.PerCallBilling = hasPrice || common.StringsContains(constant.TaskPricePatches, bc.OriginModelName)
	if !pricing.PerCallBilling && pricing.ModelRatio > 0 && task.Status == TaskStatusSuccess && (pricing.TotalTokens <= 0 || pricing.TotalTokens > int64(common.MaxQuota)) {
		return pricing, ErrBillingCorrectionBlocked
	}
	if _, known := seedancepricing.BasePriceCNY(bc.OriginModelName); known {
		if !request.Valid {
			return pricing, ErrBillingCorrectionBlocked
		}
		// Keep complete archived input evidence for video/no-video pricing.
		// A returned resolution describes the generated output and can prove
		// an omitted request option; it must never conceal an invalid request.
		if request.Resolution != "" {
			if _, ok := seedancepricing.Lookup(bc.OriginModelName, request.Resolution, request.HasVideo); !ok {
				return pricing, ErrBillingCorrectionBlocked
			}
		}
		resolution, source := request.Resolution, "archived_request"
		if len(result.Resolution) > 0 {
			if common.GetJsonType(result.Resolution) != "string" || common.Unmarshal(result.Resolution, &resolution) != nil {
				return pricing, ErrBillingCorrectionBlocked
			}
			resolution = strings.ToLower(strings.TrimSpace(resolution))
			source = "upstream_response"
		}
		quoted, ok := seedancepricing.Lookup(bc.OriginModelName, resolution, request.HasVideo)
		if !ok {
			return pricing, ErrBillingCorrectionBlocked
		}
		pricing.Resolution, pricing.HasVideo = resolution, &request.HasVideo
		pricing.RequestedResolution, pricing.ResolutionSource = request.Resolution, source
		delete(pricing.OtherRatios, "video_input")
		if quoted.Ratio != 1 {
			pricing.OtherRatios["video_input"] = quoted.Ratio
		}
	}
	return pricing, nil
}

func billingCorrectionEvidence(scope *gorm.DB, batch *BillingCorrection, startSequence int64) ([]BillingCorrectionRow, error) {
	var models []string
	if err := common.UnmarshalJsonStr(batch.Models, &models); err != nil {
		return nil, err
	}
	var entries []BillingEntry
	err := scope.Session(&gorm.Session{NewDB: true}).Where("user_id = ? AND posted_at >= ? AND posted_at < ? AND model_name IN ? AND kind IN ?", batch.UserID, batch.StartAt, batch.EndAt, models,
		[]string{"usage", "refund", "task_adjustment", "history_usage", "history_refund"}).Where("sequence >= ? AND sequence <= ?", startSequence, batch.SourceSequence).Order("sequence asc").Limit(1001).Find(&entries).Error
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 || len(entries) > 1000 {
		return nil, ErrBillingCorrectionBlocked
	}
	rates, err := billingEntryHistoricalRates(scope, batch.UserID, entries)
	if err != nil {
		return nil, err
	}
	ids, entryIDs := make([]string, 0, len(entries)), make([]int64, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, rates[entry.Sequence].TaskID)
		entryIDs = append(entryIDs, entry.ID)
	}
	var tasks []Task
	if err := scope.Session(&gorm.Session{NewDB: true}).Where("user_id = ? AND task_id IN ?", batch.UserID, ids).Find(&tasks).Error; err != nil {
		return nil, err
	}
	requests := make(map[string]billingCorrectionVideoRequest)
	if batch.Mode == BillingCorrectionModelPricing {
		requests, err = billingCorrectionTaskRequests(scope, tasks)
		if err != nil {
			return nil, err
		}
	}
	byTask := make(map[string][]Task)
	for _, task := range tasks {
		byTask[task.TaskID] = append(byTask[task.TaskID], task)
	}
	var claims []BillingCorrectionClaim
	if err := scope.Session(&gorm.Session{NewDB: true}).Where("source_entry_id IN ?", entryIDs).Find(&claims).Error; err != nil {
		return nil, err
	}
	claimed := make(map[int64]string)
	previousIDs := make([]string, 0, len(claims))
	for _, claim := range claims {
		claimed[claim.SourceEntryID] = claim.BatchID
		if claim.BatchID != "" {
			previousIDs = append(previousIDs, claim.BatchID)
		}
	}
	var previousRows []BillingCorrectionRow
	if len(previousIDs) > 0 {
		if err := scope.Session(&gorm.Session{NewDB: true}).Where("source_entry_id IN ? AND batch_id IN ?", entryIDs, previousIDs).Find(&previousRows).Error; err != nil {
			return nil, err
		}
	}
	previousPricing := make(map[int64]string)
	for _, previous := range previousRows {
		if previous.BatchID == claimed[previous.SourceEntryID] {
			previousPricing[previous.SourceEntryID] = previous.TargetPricing
		}
	}
	var corrections []BillingEntry
	if err := scope.Session(&gorm.Session{NewDB: true}).Where("user_id = ? AND kind = ? AND source_entry_id IN ? AND sequence <= ?", batch.UserID, "rate_correction", entryIDs, batch.SourceSequence).Order("sequence asc").Find(&corrections).Error; err != nil {
		return nil, err
	}
	bySource := make(map[int64][]BillingEntry)
	for _, correction := range corrections {
		bySource[correction.SourceEntryID] = append(bySource[correction.SourceEntryID], correction)
	}
	var target float64
	if batch.Mode != BillingCorrectionModelPricing && common.UnmarshalJsonStr(batch.TargetRate, &target) != nil {
		return nil, ErrBillingCorrectionBlocked
	}
	taskEntries := make(map[string][]BillingEntry)
	for _, entry := range entries {
		taskEntries[rates[entry.Sequence].TaskID] = append(taskEntries[rates[entry.Sequence].TaskID], entry)
	}
	rows := make([]BillingCorrectionRow, 0, len(entries))
	for _, entry := range entries {
		rate := rates[entry.Sequence]
		row := BillingCorrectionRow{BatchID: batch.ID, SourceEntryID: entry.ID, ModelName: entry.ModelName, PostedAt: entry.PostedAt, TaskID: rate.TaskID, TokenID: entry.TokenID,
			OriginalGroup: rate.Group, OriginalRate: rate.Ratio, OriginalQuota: entry.Quota, EffectiveQuota: entry.Quota, CorrectedQuota: entry.Quota, PreviousBatchID: claimed[entry.ID], TargetGroup: batch.TargetGroup, TargetRate: batch.TargetRate}
		effectiveGroup, effectiveRate := rate.Group, rate.Ratio
		for _, correction := range bySource[entry.ID] {
			if correction.Sequence <= entry.Sequence || correction.ModelName != entry.ModelName || correction.TokenID != entry.TokenID || correction.CorrectionID == "" ||
				correction.Quota > common.MaxQuota || correction.Quota < -common.MaxQuota || row.EffectiveQuota > int64(common.MaxQuota)-correction.Quota || row.EffectiveQuota < -int64(common.MaxQuota)-correction.Quota {
				return nil, ErrBillingEvidenceIntegrity
			}
			row.EffectiveQuota += correction.Quota
			effectiveGroup, effectiveRate = correction.BillingGroup, correction.BillingRate
		}
		row.CorrectedQuota = row.EffectiveQuota
		row.EffectiveGroup, row.EffectiveRate = effectiveGroup, effectiveRate
		if (entry.Quota >= 0 && row.EffectiveQuota < 0) || (entry.Quota < 0 && row.EffectiveQuota > 0) || entry.Quota > common.MaxQuota || entry.Quota < -common.MaxQuota {
			return nil, ErrBillingEvidenceIntegrity
		}
		if batch.Mode == BillingCorrectionModelPricing {
			row.TargetGroup, row.TargetRate = effectiveGroup, effectiveRate
		}
		matches := byTask[rate.TaskID]
		if len(matches) != 1 {
			row.Blocked = "missing_task_evidence"
			rows = append(rows, row)
			continue
		}
		task := &matches[0]
		bc := task.PrivateData.BillingContext
		row.ChannelID = task.ChannelId
		if bc == nil || bc.OriginModelName != entry.ModelName || task.Group != rate.Group || task.PrivateData.BillingSource == "subscription" || (task.Status != TaskStatusSuccess && task.Status != TaskStatusFailure) {
			row.Blocked = "unsupported_or_unfinished_task"
		}
		oldPricing, oldErr := billingCorrectionTaskPricing(task, false, requests[task.TaskID])
		newPricing, newErr := oldPricing, oldErr
		if batch.Mode != BillingCorrectionModelPricing && previousPricing[entry.ID] != "" {
			// Decode the effective price into its own multiplier map. Reusing
			// the historical map would alter the original submission evidence.
			newPricing.OtherRatios = nil
			newErr = common.UnmarshalJsonStr(previousPricing[entry.ID], &newPricing)
		}
		if batch.Mode == BillingCorrectionModelPricing {
			newPricing, newErr = billingCorrectionTaskPricing(task, true, requests[task.TaskID])
			if newErr != nil {
				row.Blocked = "missing_current_pricing_evidence"
			}
			if common.UnmarshalJsonStr(row.TargetRate, &target) != nil {
				row.Blocked = "original_price_mismatch"
			}
		}
		if bc != nil {
			encodedRate, _ := common.Marshal(bc.GroupRatio)
			if string(encodedRate) != rate.Ratio || task.PrivateData.TokenId != entry.TokenID {
				row.Blocked = "original_price_mismatch"
			}
			oldSubmission, oldSubmitErr := billingCorrectionPricingQuota(oldPricing, bc.GroupRatio, false)
			newSubmission, newSubmitErr := billingCorrectionPricingQuota(newPricing, target, false)
			oldFinal := oldSubmission
			if oldPricing.ModelRatio > 0 && !oldPricing.PerCallBilling && oldPricing.TotalTokens > 0 {
				oldFinal, oldErr = billingCorrectionPricingQuota(oldPricing, bc.GroupRatio, true)
			}
			// Polling publishes SUCCESS before it commits token settlement. A lone
			// token submission does not prove settlement finished, even when the
			// saved rate reproduces its precharge: live polling reads today's rate.
			// Without another durable entry both paths could post the same delta.
			if task.Status == TaskStatusSuccess && !oldPricing.PerCallBilling && oldPricing.PricingMode == "tokens" &&
				len(taskEntries[task.TaskID]) == 1 && int64(task.Quota) == oldSubmission {
				row.Blocked = "incomplete_task_range"
			}
			if (batch.Mode == BillingCorrectionModelPricing || previousPricing[entry.ID] != "") && task.Status == TaskStatusSuccess {
				// Older completion code could read a configured token ratio without
				// persisting it in the submission context. The immutable terminal
				// task quota and the complete ledger still prove the original amount.
				oldFinal = int64(task.Quota)
			}
			newFinal := newSubmission
			if newPricing.ModelRatio > 0 && !newPricing.PerCallBilling && task.Status == TaskStatusSuccess {
				newFinal, newErr = billingCorrectionPricingQuota(newPricing, target, true)
			}
			lifecycle := taskEntries[task.TaskID]
			var oldCumulative, correctedBefore, correctedAfter int64
			for i, source := range lifecycle {
				previous := oldCumulative
				oldCumulative += source.Quota
				if oldCumulative < 0 || oldCumulative > common.MaxQuota {
					row.Blocked = "original_price_mismatch"
				}
				correctedState := int64(0)
				switch {
				case oldCumulative == 0:
				case oldCumulative == oldSubmission:
					correctedState = newSubmission
				case oldCumulative == oldFinal:
					correctedState = newFinal
				default:
					row.Blocked = "original_price_mismatch"
				}
				if i == len(lifecycle)-1 && task.Status == TaskStatusSuccess {
					correctedState = newFinal
				}
				if i == 0 && (previous != 0 || source.Quota != oldSubmission) {
					row.Blocked = "original_price_mismatch"
				}
				if source.ID == entry.ID {
					correctedAfter = correctedState
					break
				}
				correctedBefore = correctedState
			}
			var originalFinal int64
			for _, source := range lifecycle {
				originalFinal += source.Quota
			}
			if originalFinal != int64(task.Quota) {
				row.Blocked = "incomplete_task_range"
			}
			if row.Blocked == "" && ((task.Status == TaskStatusFailure && originalFinal != 0) || (task.Status == TaskStatusSuccess && originalFinal != oldFinal) || oldErr != nil || newErr != nil || oldSubmitErr != nil || newSubmitErr != nil) {
				row.Blocked = "original_price_mismatch"
			}
			corrected := correctedAfter - correctedBefore
			if row.Blocked == "" && ((entry.Quota >= 0 && corrected < 0) || (entry.Quota < 0 && corrected > 0)) {
				row.Blocked = "adjustment_direction_changed"
			}
			if row.Blocked == "" {
				row.CorrectedQuota, row.Delta = corrected, corrected-row.EffectiveQuota
			}
		}
		pricingJSON, err := common.Marshal(newPricing)
		if err != nil {
			return nil, err
		}
		row.TargetPricing = string(pricingJSON)
		// Request bodies, secrets and response URLs never enter downloaded
		// previews. Only the billing parameters extracted above are hashed.
		evidence, err := common.Marshal(struct {
			Entry              BillingEntry
			Rate               billingHistoricalRate
			Context            *TaskBillingContext
			Pricing            billingCorrectionPricing
			Quota              int
			Status             TaskStatus
			ChannelID, TokenID int
			Corrections        []BillingEntry
		}{entry, rate, bc, newPricing, task.Quota, task.Status, task.ChannelId, task.PrivateData.TokenId, bySource[entry.ID]})
		if err != nil {
			return nil, err
		}
		hash := sha256.Sum256(evidence)
		row.SourceSHA256 = hex.EncodeToString(hash[:])
		rows = append(rows, row)
	}
	// Rows are persisted/read in source-ID order; the evidence digest must use
	// the same order even if imported historical sequences differ from IDs.
	sort.Slice(rows, func(i, j int) bool { return rows[i].SourceEntryID < rows[j].SourceEntryID })
	return billingCorrectionCostEvidence(scope, batch, rows)
}

func billingCorrectionDigest(batch *BillingCorrection, rows []BillingCorrectionRow) (string, error) {
	body, err := common.Marshal(struct {
		ID                                             string
		UserID, CreatedBy                              int
		UserGroup                                      string
		CreatedAt, ExpiresAt, StartAt, EndAt, Sequence int64
		Models, Group, Rate, Reason                    string
		Mode                                           string `json:",omitempty"`
		CanApply                                       bool
		ChargeDelta, RefundDelta, NetDelta             int64
		CurrentCostQuota, CorrectedCostQuota           *int64 `json:",omitempty"`
		CostDelta                                      int64  `json:",omitempty"`
		Rows                                           []BillingCorrectionRow
	}{
		ID: batch.ID, UserID: batch.UserID, CreatedBy: batch.CreatedBy, UserGroup: batch.UserGroup,
		CreatedAt: batch.CreatedAt, ExpiresAt: batch.ExpiresAt, StartAt: batch.StartAt, EndAt: batch.EndAt, Sequence: batch.SourceSequence,
		Models: batch.Models, Group: batch.TargetGroup, Rate: batch.TargetRate, Reason: batch.Reason,
		Mode:     batch.Mode,
		CanApply: batch.CanApply, ChargeDelta: batch.ChargeDelta, RefundDelta: batch.RefundDelta, NetDelta: batch.NetDelta, Rows: rows,
		CurrentCostQuota: batch.CurrentCostQuota, CorrectedCostQuota: batch.CorrectedCostQuota, CostDelta: batch.CostDelta,
	})
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(body)
	return hex.EncodeToString(hash[:]), nil
}

func PreviewBillingCorrection(input BillingCorrectionInput, actorID int, rate string, userGroup string) (*BillingCorrection, error) {
	input.Reason, input.TargetGroup = strings.TrimSpace(input.Reason), strings.TrimSpace(input.TargetGroup)
	if input.Mode == "" {
		input.Mode = BillingCorrectionGroupRate
	}
	if input.UserID <= 0 || actorID <= 0 || input.StartAt <= 0 || input.EndAt <= input.StartAt || input.EndAt > common.GetTimestamp() || input.EndAt-input.StartAt > 93*86400 ||
		len(input.Models) == 0 || len(input.Models) > 20 || len(input.Reason) < 4 || len([]rune(input.Reason)) > 1000 || len(input.TargetGroup) > 50 ||
		(input.Mode != BillingCorrectionGroupRate && input.Mode != BillingCorrectionModelPricing) || (input.Mode == BillingCorrectionGroupRate && input.TargetGroup == "") {
		return nil, ErrBillingCorrectionBlocked
	}
	if input.Mode == BillingCorrectionModelPricing {
		input.TargetGroup, rate = "", ""
	}
	for i := range input.Models {
		input.Models[i] = strings.TrimSpace(input.Models[i])
		if input.Models[i] == "" || len(input.Models[i]) > 255 {
			return nil, ErrBillingCorrectionBlocked
		}
	}
	sort.Strings(input.Models)
	models, err := common.Marshal(input.Models)
	if err != nil {
		return nil, err
	}
	now := common.GetTimestamp()
	batch := &BillingCorrection{ID: common.GetUUID(), UserID: input.UserID, UserGroup: userGroup, CreatedBy: actorID, CreatedAt: now, ExpiresAt: now + 900, StartAt: input.StartAt, EndAt: input.EndAt,
		Models: string(models), Mode: input.Mode, TargetGroup: input.TargetGroup, TargetRate: rate, Reason: input.Reason, Status: "preview", CanApply: true}
	err = DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).First(&user, input.UserID).Error; err != nil {
			return err
		}
		if user.Group != userGroup {
			return ErrBillingConflict
		}
		account, err := lockBillingAccount(tx, input.UserID)
		if err != nil {
			return err
		}
		if account.AccountingStartAt <= 0 || input.StartAt < account.AccountingStartAt {
			return ErrBillingNotConfigured
		}
		batch.SourceSequence = account.Sequence
		batch.Rows, err = billingCorrectionEvidence(tx, batch, account.StartSequence)
		if err != nil {
			return err
		}
		changed := false
		for _, row := range batch.Rows {
			if row.Delta != 0 || row.CostChanged {
				changed = true
			}
			if row.CurrentCostQuota != nil && row.CorrectedCostQuota != nil {
				if batch.CurrentCostQuota == nil {
					batch.CurrentCostQuota, batch.CorrectedCostQuota = new(int64), new(int64)
				}
				*batch.CurrentCostQuota, err = billingProjectionQuotaTotal(*batch.CurrentCostQuota, *row.CurrentCostQuota)
				if err != nil {
					return err
				}
				*batch.CorrectedCostQuota, err = billingProjectionQuotaTotal(*batch.CorrectedCostQuota, *row.CorrectedCostQuota)
				if err != nil {
					return err
				}
				batch.CostDelta, err = billingProjectionQuotaTotal(batch.CostDelta, row.CostDelta)
				if err != nil {
					return err
				}
			}
			if row.Blocked != "" {
				batch.CanApply = false
			}
			if row.OriginalQuota < 0 {
				batch.RefundDelta -= row.Delta
			} else {
				batch.ChargeDelta += row.Delta
			}
		}
		batch.NetDelta = batch.ChargeDelta - batch.RefundDelta
		// Refund lifecycles and cost-only corrections can leave the wallet
		// unchanged. Any changed sales or cost source makes the batch actionable.
		if !changed {
			batch.CanApply = false
		}
		batch.SHA256, err = billingCorrectionDigest(batch, batch.Rows)
		if err != nil {
			return err
		}
		if err := tx.Create(batch).Error; err != nil {
			return err
		}
		return tx.Create(&batch.Rows).Error
	})
	return batch, err
}

func GetBillingCorrection(id string) (*BillingCorrection, error) {
	var batch BillingCorrection
	if err := DB.First(&batch, "id = ?", id).Error; err != nil {
		return nil, err
	}
	if err := DB.Where("batch_id = ?", id).Order("source_entry_id asc").Find(&batch.Rows).Error; err != nil {
		return nil, err
	}
	return &batch, nil
}
