package model

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
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
	TargetGroup           string                 `json:"target_group" gorm:"type:varchar(50)"`
	TargetRate            string                 `json:"target_rate" gorm:"type:varchar(64)"`
	Reason                string                 `json:"reason" gorm:"type:varchar(1000)"`
	Status                string                 `json:"status" gorm:"type:varchar(16)"`
	SourceSequence        int64                  `json:"source_sequence" gorm:"bigint"`
	CanApply              bool                   `json:"can_apply"`
	ChargeDelta           int64                  `json:"charge_delta" gorm:"bigint"`
	RefundDelta           int64                  `json:"refund_delta" gorm:"bigint"`
	NetDelta              int64                  `json:"net_delta" gorm:"bigint"`
	SHA256                string                 `json:"sha256" gorm:"type:char(64)"`
	AppliedAt             int64                  `json:"applied_at" gorm:"bigint"`
	ReversedAt            int64                  `json:"reversed_at" gorm:"bigint"`
	ReversalReason        string                 `json:"reversal_reason" gorm:"type:varchar(1000)"`
	AuditLoggedAt         int64                  `json:"audit_logged_at" gorm:"bigint"`
	ReversalAuditLoggedAt int64                  `json:"reversal_audit_logged_at" gorm:"bigint"`
	Rows                  []BillingCorrectionRow `json:"rows" gorm:"-"`
}

type BillingCorrectionRow struct {
	ID             int64  `json:"-" gorm:"primaryKey"`
	BatchID        string `json:"batch_id" gorm:"type:varchar(64);index"`
	SourceEntryID  int64  `json:"source_entry_id" gorm:"bigint;index"`
	ModelName      string `json:"model_name" gorm:"type:varchar(255)"`
	PostedAt       int64  `json:"posted_at" gorm:"bigint"`
	TaskID         string `json:"task_id" gorm:"type:varchar(191)"`
	TokenID        int    `json:"token_id"`
	ChannelID      int    `json:"channel_id"`
	OriginalGroup  string `json:"original_group" gorm:"type:varchar(50)"`
	OriginalRate   string `json:"original_rate" gorm:"type:varchar(64)"`
	OriginalQuota  int64  `json:"original_quota" gorm:"bigint"`
	CorrectedQuota int64  `json:"corrected_quota" gorm:"bigint"`
	Delta          int64  `json:"delta" gorm:"bigint"`
	SourceSHA256   string `json:"source_sha256" gorm:"type:char(64)"`
	Blocked        string `json:"blocked" gorm:"type:varchar(64)"`
}

// This per-source claim serializes overlapping batches. Reversal releases its
// active pointer without deleting any correction or audit history.
type BillingCorrectionClaim struct {
	SourceEntryID int64  `gorm:"primaryKey;autoIncrement:false"`
	BatchID       string `gorm:"type:varchar(64)"`
}

type BillingCorrectionInput struct {
	UserID      int      `json:"user_id"`
	StartAt     int64    `json:"start_at"`
	EndAt       int64    `json:"end_at"`
	Models      []string `json:"models"`
	TargetGroup string   `json:"target_group"`
	Reason      string   `json:"reason"`
}

var ErrBillingCorrectionBlocked = errors.New("billing correction cannot be safely applied")

// Reproduce both original truncation steps, rather than dividing an already
// discounted integer quota by its old discount. Unknown pricing modes fail closed.
func billingCorrectionTaskQuota(task *Task, ratio float64) (int64, error) {
	bc := task.PrivateData.BillingContext
	if bc == nil || math.IsNaN(ratio) || math.IsInf(ratio, 0) || ratio < 0 || ratio > 10 {
		return 0, ErrBillingCorrectionBlocked
	}
	price := bc.ModelPrice
	if price <= 0 || math.IsNaN(price) || math.IsInf(price, 0) {
		return 0, ErrBillingCorrectionBlocked
	}
	base, err := common.QuotaFromFloatStrict(price * common.QuotaPerUnit * ratio)
	if err != nil {
		return 0, err
	}
	// PerCallBilling suppresses later polling adjustments; submission still
	// applies duration/resolution multipliers. It does not mean one base unit.
	// Legacy calculation first multiplies the additional ratios together,
	// then multiplies the integer base. Sort keys to make correction previews
	// and execution deterministic even when a snapshot has multiple ratios.
	keys := make([]string, 0, len(bc.OtherRatios))
	for key, multiplier := range bc.OtherRatios {
		if multiplier <= 0 || math.IsNaN(multiplier) || math.IsInf(multiplier, 0) {
			return 0, ErrBillingCorrectionBlocked
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	multiplier := 1.0
	for _, key := range keys {
		multiplier *= bc.OtherRatios[key]
	}
	quota, err := common.QuotaFromFloatStrict(float64(base) * multiplier)
	return int64(quota), err
}

func billingCorrectionEvidence(scope *gorm.DB, batch *BillingCorrection, startSequence int64) ([]BillingCorrectionRow, error) {
	var models []string
	if err := common.UnmarshalJsonStr(batch.Models, &models); err != nil {
		return nil, err
	}
	var entries []BillingEntry
	err := scope.Session(&gorm.Session{NewDB: true}).Where("user_id = ? AND posted_at >= ? AND posted_at < ? AND model_name IN ? AND kind IN ?", batch.UserID, batch.StartAt, batch.EndAt, models,
		[]string{"usage", "refund", "task_adjustment", "history_usage", "history_refund"}).Where("sequence >= ? AND sequence <= ?", startSequence, batch.SourceSequence).Order("id asc").Limit(1001).Find(&entries).Error
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
	ids := make([]string, 0, len(entries))
	entryIDs := make([]int64, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, rates[entry.Sequence].TaskID)
		entryIDs = append(entryIDs, entry.ID)
	}
	var tasks []Task
	if err := scope.Session(&gorm.Session{NewDB: true}).Where("user_id = ? AND task_id IN ?", batch.UserID, ids).Find(&tasks).Error; err != nil {
		return nil, err
	}
	byTask := make(map[string][]Task)
	for _, task := range tasks {
		byTask[task.TaskID] = append(byTask[task.TaskID], task)
	}
	var claims []BillingCorrectionClaim
	if err := scope.Session(&gorm.Session{NewDB: true}).Where("source_entry_id IN ? AND batch_id <> ?", entryIDs, "").Find(&claims).Error; err != nil {
		return nil, err
	}
	claimed := make(map[int64]bool)
	for _, claim := range claims {
		claimed[claim.SourceEntryID] = true
	}
	var target float64
	if err := common.UnmarshalJsonStr(batch.TargetRate, &target); err != nil {
		return nil, ErrBillingCorrectionBlocked
	}
	selectedModels := make(map[string]bool, len(models))
	for _, name := range models {
		selectedModels[name] = true
	}
	rows := make([]BillingCorrectionRow, 0, len(entries))
	taskTotals := make(map[string]int64)
	for _, entry := range entries {
		rate := rates[entry.Sequence]
		row := BillingCorrectionRow{BatchID: batch.ID, SourceEntryID: entry.ID, ModelName: entry.ModelName, PostedAt: entry.PostedAt, TaskID: rate.TaskID, TokenID: entry.TokenID,
			OriginalGroup: rate.Group, OriginalRate: rate.Ratio, OriginalQuota: entry.Quota, CorrectedQuota: entry.Quota}
		if !selectedModels[entry.ModelName] {
			return nil, ErrBillingCorrectionBlocked
		}
		if entry.Quota > common.MaxQuota || entry.Quota < -common.MaxQuota {
			row.Blocked = "original_price_mismatch"
			rows = append(rows, row)
			continue
		}
		if claimed[entry.ID] {
			row.Blocked = "already_corrected"
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
		if bc == nil || bc.OriginModelName != entry.ModelName || task.Group != rate.Group || task.PrivateData.BillingSource == "subscription" ||
			(task.Status != TaskStatusSuccess && task.Status != TaskStatusFailure) {
			row.Blocked = "unsupported_or_unfinished_task"
		}
		if bc != nil {
			encodedRate, _ := common.Marshal(bc.GroupRatio)
			if string(encodedRate) != rate.Ratio || task.PrivateData.TokenId != entry.TokenID {
				row.Blocked = "original_price_mismatch"
			}
			old, oldErr := billingCorrectionTaskQuota(task, bc.GroupRatio)
			corrected, newErr := billingCorrectionTaskQuota(task, target)
			absolute := entry.Quota
			if absolute < 0 {
				absolute = -absolute
			}
			if oldErr != nil || newErr != nil || absolute != old {
				row.Blocked = "original_price_mismatch"
			}
			if row.Blocked == "" {
				if entry.Quota < 0 {
					corrected = -corrected
				}
				if entry.Quota == 0 && task.Status == TaskStatusFailure {
					corrected = 0
				}
				row.CorrectedQuota, row.Delta = corrected, corrected-entry.Quota
			}
		}
		// Serialize only billing evidence; private token keys and request bodies
		// never enter the preview, its fingerprint or downloadable audit file.
		evidence, err := common.Marshal(struct {
			Entry     BillingEntry
			Rate      billingHistoricalRate
			Context   *TaskBillingContext
			Quota     int
			Status    TaskStatus
			ChannelID int
			TokenID   int
		}{entry, rate, bc, task.Quota, task.Status, task.ChannelId, task.PrivateData.TokenId})
		if err != nil {
			return nil, err
		}
		hash := sha256.Sum256(evidence)
		row.SourceSHA256 = hex.EncodeToString(hash[:])
		taskTotals[task.TaskID] += entry.Quota
		rows = append(rows, row)
	}
	for i := range rows {
		matches := byTask[rows[i].TaskID]
		if len(matches) == 1 && taskTotals[rows[i].TaskID] != int64(matches[0].Quota) {
			rows[i].Blocked = "incomplete_task_range"
			rows[i].CorrectedQuota, rows[i].Delta = rows[i].OriginalQuota, 0
		}
	}
	return rows, nil
}

func billingCorrectionDigest(batch *BillingCorrection, rows []BillingCorrectionRow) (string, error) {
	body, err := common.Marshal(struct {
		ID                                             string
		UserID, CreatedBy                              int
		UserGroup                                      string
		CreatedAt, ExpiresAt, StartAt, EndAt, Sequence int64
		Models, Group, Rate, Reason                    string
		CanApply                                       bool
		ChargeDelta, RefundDelta, NetDelta             int64
		Rows                                           []BillingCorrectionRow
	}{
		ID: batch.ID, UserID: batch.UserID, CreatedBy: batch.CreatedBy, UserGroup: batch.UserGroup,
		CreatedAt: batch.CreatedAt, ExpiresAt: batch.ExpiresAt, StartAt: batch.StartAt, EndAt: batch.EndAt, Sequence: batch.SourceSequence,
		Models: batch.Models, Group: batch.TargetGroup, Rate: batch.TargetRate, Reason: batch.Reason,
		CanApply: batch.CanApply, ChargeDelta: batch.ChargeDelta, RefundDelta: batch.RefundDelta, NetDelta: batch.NetDelta, Rows: rows,
	})
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(body)
	return hex.EncodeToString(hash[:]), nil
}

func PreviewBillingCorrection(input BillingCorrectionInput, actorID int, rate string, userGroup string) (*BillingCorrection, error) {
	input.Reason, input.TargetGroup = strings.TrimSpace(input.Reason), strings.TrimSpace(input.TargetGroup)
	if input.UserID <= 0 || actorID <= 0 || input.StartAt <= 0 || input.EndAt <= input.StartAt || input.EndAt > common.GetTimestamp() || input.EndAt-input.StartAt > 93*86400 ||
		len(input.Models) == 0 || len(input.Models) > 20 || len(input.Reason) < 4 || len([]rune(input.Reason)) > 1000 || input.TargetGroup == "" || len(input.TargetGroup) > 50 {
		return nil, ErrBillingCorrectionBlocked
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
		Models: string(models), TargetGroup: input.TargetGroup, TargetRate: rate, Reason: input.Reason, Status: "preview", CanApply: true}
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
		for _, row := range batch.Rows {
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
		if batch.NetDelta == 0 {
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
