package model

import (
	"fmt"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// AppliedAt/LoggedAt form a durable outbox: a separate log database outage
// must never roll back committed money or make a repeated request charge twice.
func PublishBillingCorrectionAudit(id string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var batch BillingCorrection
		if err := lockForUpdate(tx).First(&batch, "id = ?", id).Error; err != nil {
			return err
		}
		var user User
		if err := tx.Select("id", "username").First(&user, batch.UserID).Error; err != nil {
			return err
		}
		logsDB := LOG_DB
		if LOG_DB == DB {
			logsDB = tx.Session(&gorm.Session{NewDB: true})
		}
		var rows []BillingCorrectionRow
		if err := tx.Where("batch_id = ?", id).Order("source_entry_id asc").Find(&rows).Error; err != nil {
			return err
		}
		for _, action := range []string{"apply", "reverse"} {
			at, logged, reason := batch.AppliedAt, batch.AuditLoggedAt, batch.Reason
			column := "audit_logged_at"
			if action == "reverse" {
				at, logged, reason = batch.ReversedAt, batch.ReversalAuditLoggedAt, batch.ReversalReason
				column = "reversal_audit_logged_at"
			}
			if at == 0 || logged > 0 {
				continue
			}
			for _, row := range rows {
				if row.Delta == 0 {
					continue
				}
				delta := row.Delta
				group := batch.TargetGroup
				if action == "reverse" {
					delta = -delta
					group = row.OriginalGroup
				}
				body, err := common.Marshal(map[string]any{"admin_info": map[string]any{"correction_id": batch.ID, "source_entry_id": row.SourceEntryID, "action": action, "actor_id": batch.CreatedBy, "quota_delta": delta, "sha256": batch.SHA256, "reason": reason}})
				if err != nil {
					return err
				}
				quota, err := common.WalletQuotaFromDecimalStrict(decimal.NewFromInt(delta))
				if err != nil {
					return err
				}
				requestID := billingCorrectionRequestID("correction:" + action + ":" + batch.ID + ":" + strconv.FormatInt(row.SourceEntryID, 10))
				var snapshot CostAccountingSnapshot
				if err := tx.Select("occurred_at", "token_name").First(&snapshot, "request_id = ? AND source = ?", requestID, "correction").Error; err != nil {
					return err
				}
				var existing int64
				if err := logsDB.Model(&Log{}).Where("request_id = ? AND type = ?", requestID, LogTypeBillingCorrection).Count(&existing).Error; err != nil {
					return err
				}
				if existing == 0 {
					log := Log{UserId: batch.UserID, Username: user.Username, CreatedAt: snapshot.OccurredAt, TokenName: snapshot.TokenName, Type: LogTypeBillingCorrection, Quota: quota, Group: group, TokenId: row.TokenID, ChannelId: row.ChannelID, ModelName: row.ModelName,
						RequestId: requestID, Content: fmt.Sprintf("费率调账 %s · 批次 %s · 净额度差额 %d · %s", action, batch.ID, delta, reason), Other: string(body)}
					if err := logsDB.Create(&log).Error; err != nil {
						return err
					}
				}
			}
			if err := tx.Model(&batch).Update(column, common.GetTimestamp()).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func FlushBillingCorrectionAudits() error {
	var batches []BillingCorrection
	if err := DB.Where("(applied_at > 0 AND audit_logged_at = 0) OR (reversed_at > 0 AND reversal_audit_logged_at = 0)").Limit(20).Find(&batches).Error; err != nil {
		return err
	}
	for _, batch := range batches {
		if err := PublishBillingCorrectionAudit(batch.ID); err != nil {
			return err
		}
	}
	return nil
}
