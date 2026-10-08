package model

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func billingCorrectionRequestID(eventKey string) string {
	hash := sha256.Sum256([]byte(eventKey))
	return hex.EncodeToString(hash[:])
}

// ApplyBillingCorrection atomically moves money, appends accounting entries,
// updates usage counters, and claims each source exactly once. A reverse action
// appends the opposite entries; it never edits the original financial evidence.
func ApplyBillingCorrection(id, digest, rate, userGroup string, actorID int, reverse bool, reason string) (*BillingCorrection, error) {
	batch, err := GetBillingCorrection(id)
	if err != nil {
		return nil, err
	}
	if batch.CreatedBy != actorID || digest == "" || digest != batch.SHA256 {
		return nil, ErrBillingConflict
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).First(&user, batch.UserID).Error; err != nil {
			return err
		}
		account, err := lockBillingAccount(tx, batch.UserID)
		if err != nil {
			return err
		}
		if err := lockForUpdate(tx).First(batch, "id = ?", id).Error; err != nil {
			return err
		}
		if err := tx.Where("batch_id = ?", id).Order("source_entry_id asc").Find(&batch.Rows).Error; err != nil {
			return err
		}
		actualDigest, err := billingCorrectionDigest(batch, batch.Rows)
		if err != nil {
			return err
		}
		if actualDigest != digest || batch.CreatedBy != actorID {
			return ErrBillingConflict
		}
		if (!reverse && batch.Status == "applied") || (reverse && batch.Status == "reversed") {
			return nil
		}
		if !batch.CanApply {
			return ErrBillingCorrectionBlocked
		}
		var pending int64
		if err := tx.Model(&BillingOperation{}).Where("user_id = ? AND state = ?", user.Id, "reserved").Count(&pending).Error; err != nil {
			return err
		}
		if pending > 0 {
			return ErrBillingCorrectionBlocked
		}
		if reverse {
			reason = strings.TrimSpace(reason)
			if batch.Status != "applied" || len(reason) < 4 || len([]rune(reason)) > 1000 {
				return ErrBillingConflict
			}
		} else {
			if batch.Status != "preview" || batch.ExpiresAt < common.GetTimestamp() || batch.TargetRate != rate || batch.UserGroup != userGroup || user.Group != userGroup || batch.SourceSequence != account.Sequence {
				return ErrBillingConflict
			}
			rows, err := billingCorrectionEvidence(tx, batch, account.StartSequence)
			if err != nil {
				return err
			}
			currentDigest, err := billingCorrectionDigest(batch, rows)
			if err != nil {
				return err
			}
			if currentDigest != digest {
				return ErrBillingConflict
			}
		}
		net := batch.NetDelta
		if reverse {
			net = -net
		}
		if net > int64(user.Quota) {
			return ErrBillingInsufficientQuota
		}
		if user.UsedQuota < 0 || int64(user.UsedQuota) > int64(common.MaxWalletQuota) {
			return ErrWalletQuotaLimitExceeded
		}
		if int64(user.UsedQuota)+net < 0 || int64(user.UsedQuota)+net > int64(common.MaxWalletQuota) {
			return ErrWalletQuotaLimitExceeded
		}
		// Credits first prevent a temporarily negative wallet while applying a
		// batch with both additional consumption and additional refunds.
		rows := append([]BillingCorrectionRow(nil), batch.Rows...)
		sort.SliceStable(rows, func(i, j int) bool {
			if reverse {
				return rows[i].Delta > rows[j].Delta
			}
			return rows[i].Delta < rows[j].Delta
		})
		tokenDeltas, channelDeltas := make(map[int]int64), make(map[int]int64)
		for _, row := range rows {
			if row.Blocked != "" {
				return ErrBillingCorrectionBlocked
			}
			claim := BillingCorrectionClaim{SourceEntryID: row.SourceEntryID}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&claim).Error; err != nil {
				return err
			}
			if err := lockForUpdate(tx).First(&claim, "source_entry_id = ?", row.SourceEntryID).Error; err != nil {
				return err
			}
			if (!reverse && claim.BatchID != "") || (reverse && claim.BatchID != batch.ID) {
				return ErrBillingConflict
			}
			active := batch.ID
			if reverse {
				active = ""
			}
			if err := tx.Model(&claim).Update("batch_id", active).Error; err != nil {
				return err
			}
			if row.Delta == 0 {
				continue
			}
			delta := row.Delta
			group, rowRate := batch.TargetGroup, batch.TargetRate
			action := "apply"
			if reverse {
				delta = -delta
				group, rowRate = row.OriginalGroup, row.OriginalRate
				action = "reverse"
			}
			if err := changeBillingWallet(tx, &user, -delta); err != nil {
				return err
			}
			entry := BillingEntry{EventKey: "correction:" + action + ":" + batch.ID + ":" + strconv.FormatInt(row.SourceEntryID, 10),
				Kind: "rate_correction", Quota: delta, WalletDelta: -delta, ActorID: actorID, CorrectionID: batch.ID, SourceEntryID: row.SourceEntryID,
				ModelName: row.ModelName, TokenID: row.TokenID, BillingGroup: group, BillingRate: rowRate}
			entry.RequestID = billingCorrectionRequestID(entry.EventKey)
			if err := appendBillingEntry(tx, &user, &entry); err != nil {
				return err
			}
			// A pricing correction changes customer revenue, with no new upstream
			// request or cost. Keep the original cost snapshots untouched.
			var sourceToken Token
			if row.TokenID > 0 {
				if err := tx.Select("id", "user_id", "name").First(&sourceToken, row.TokenID).Error; err != nil {
					return err
				}
				if sourceToken.UserId != user.Id {
					return ErrBillingConflict
				}
			}
			cost := CostAccountingSnapshot{EventKey: entry.EventKey, RequestID: entry.RequestID, UserID: user.Id, Username: user.Username,
				TokenName: sourceToken.Name, ModelName: row.ModelName, ChannelID: row.ChannelID, GroupName: group, LogType: LogTypeBillingCorrection, OccurredAt: entry.PostedAt,
				RevenueQuota: delta, CostBasisQuota: "0", CostDiscount: "0", SnapshotVersion: 4, Source: "correction", BatchID: batch.ID, CreatedAt: entry.PostedAt}
			if err := tx.Create(&cost).Error; err != nil {
				return err
			}
			tokenDeltas[row.TokenID] += delta
			channelDeltas[row.ChannelID] += delta
		}
		if err := tx.Model(&user).Update("used_quota", int64(user.UsedQuota)+net).Error; err != nil {
			return err
		}
		tokenIDs := make([]int, 0, len(tokenDeltas))
		for id := range tokenDeltas {
			if id > 0 {
				tokenIDs = append(tokenIDs, id)
			}
		}
		sort.Ints(tokenIDs)
		for _, id := range tokenIDs {
			var token Token
			if err := lockForUpdate(tx).First(&token, id).Error; err != nil {
				return err
			}
			delta := tokenDeltas[id]
			if token.UsedQuota < 0 || int64(token.UsedQuota) > int64(common.MaxWalletQuota) || int64(token.RemainQuota) > int64(common.MaxWalletQuota) || int64(token.RemainQuota) < -int64(common.MaxWalletQuota) {
				return ErrWalletQuotaLimitExceeded
			}
			used, remaining := int64(token.UsedQuota)+delta, int64(token.RemainQuota)-delta
			if used < 0 || used > int64(common.MaxWalletQuota) || remaining > int64(common.MaxWalletQuota) || remaining < -int64(common.MaxWalletQuota) || (!token.UnlimitedQuota && remaining < 0) {
				return ErrBillingInsufficientQuota
			}
			if err := tx.Model(&token).Updates(map[string]any{"used_quota": used, "remain_quota": remaining}).Error; err != nil {
				return err
			}
		}
		channelIDs := make([]int, 0, len(channelDeltas))
		for id := range channelDeltas {
			if id > 0 {
				channelIDs = append(channelIDs, id)
			}
		}
		sort.Ints(channelIDs)
		for _, id := range channelIDs {
			var channel Channel
			if err := lockForUpdate(tx).First(&channel, id).Error; err != nil {
				return err
			}
			if channel.UsedQuota < 0 || channel.UsedQuota > int64(common.MaxWalletQuota) {
				return ErrWalletQuotaLimitExceeded
			}
			used := channel.UsedQuota + channelDeltas[id]
			if used < 0 || used > int64(common.MaxWalletQuota) {
				return ErrWalletQuotaLimitExceeded
			}
			if err := tx.Model(&channel).Update("used_quota", used).Error; err != nil {
				return err
			}
		}
		now := common.GetTimestamp()
		if reverse {
			batch.Status = "reversed"
			batch.ReversedAt = now
			batch.ReversalReason = reason
		}
		if !reverse {
			batch.Status = "applied"
			batch.AppliedAt = now
		}
		return tx.Model(batch).Updates(map[string]any{"status": batch.Status, "applied_at": batch.AppliedAt, "reversed_at": batch.ReversedAt, "reversal_reason": batch.ReversalReason}).Error
	})
	if err == nil {
		_ = invalidateUserCache(batch.UserID)
		_ = InvalidateUserTokensCache(batch.UserID)
	}
	return batch, err
}
