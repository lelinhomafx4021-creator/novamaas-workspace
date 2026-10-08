package model

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// Wallet posting time remains the actual execution time. Statement amounts
// attribute correction deltas to each source consumption/refund's period.
const BillingSourcePeriodCorrections = "source_period_corrections_v1"

var ErrBillingStatementCorrectionsPending = errors.New("statement does not include current corrections; void it and create a new draft before issuing or confirming")

type BillingStatementCorrection struct {
	EntryID      int64  `json:"entry_id"`
	Sequence     int64  `json:"sequence"`
	PostedAt     int64  `json:"posted_at"`
	Quota        int64  `json:"quota"`
	BatchID      string `json:"batch_id"`
	BillingGroup string `json:"billing_group"`
	BillingRate  string `json:"billing_rate"`
}

func (entry BillingEntry) StatementAmountQuota() int64 {
	if entry.StatementQuota != nil {
		return *entry.StatementQuota
	}
	return entry.Quota
}

func (statement *BillingStatement) UsesSourcePeriodCorrections() (bool, error) {
	var snapshot struct {
		AccountingBasis string `json:"accounting_basis"`
	}
	if err := common.UnmarshalJsonStr(statement.Snapshot, &snapshot); err != nil {
		return false, err
	}
	if snapshot.AccountingBasis != "" && snapshot.AccountingBasis != BillingSourcePeriodCorrections {
		return false, ErrBillingEvidenceIntegrity
	}
	return snapshot.AccountingBasis == BillingSourcePeriodCorrections, nil
}

func billingStatementEntryPage(scope *gorm.DB, userID int, from, to, start, end, after int64, limit int, corrected bool) ([]BillingEntry, error) {
	entries := make([]BillingEntry, 0)
	query := scope.Session(&gorm.Session{NewDB: true}).Model(&BillingEntry{}).
		Where("user_id = ? AND sequence >= ? AND sequence > ? AND sequence <= ? AND posted_at >= ? AND posted_at < ? AND kind <> ?", userID, from, after, to, start, end, "funding")
	if corrected {
		query = query.Where("kind <> ?", "rate_correction")
	}
	if err := query.Order("sequence asc").Limit(limit).Find(&entries).Error; err != nil {
		return nil, err
	}
	if !corrected || len(entries) == 0 {
		return entries, nil
	}
	ids := make([]int64, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, entry.ID)
	}
	var corrections []BillingEntry
	if err := scope.Session(&gorm.Session{NewDB: true}).Where("user_id = ? AND kind = ? AND source_entry_id IN ? AND sequence <= ?", userID, "rate_correction", ids, to).
		Order("sequence asc").Find(&corrections).Error; err != nil {
		return nil, err
	}
	bySource := make(map[int64][]BillingEntry)
	for _, correction := range corrections {
		bySource[correction.SourceEntryID] = append(bySource[correction.SourceEntryID], correction)
	}
	for i := range entries {
		entry := &entries[i]
		adjustments := bySource[entry.ID]
		if len(adjustments) == 0 {
			continue
		}
		quota := entry.Quota
		if quota > int64(common.MaxWalletQuota) || quota < -int64(common.MaxWalletQuota) {
			return nil, ErrBillingEvidenceIntegrity
		}
		for _, correction := range adjustments {
			if correction.Sequence <= entry.Sequence || correction.ModelName != entry.ModelName || correction.TokenID != entry.TokenID || correction.CorrectionID == "" ||
				correction.Quota > int64(common.MaxWalletQuota) || correction.Quota < -int64(common.MaxWalletQuota) ||
				quota > int64(common.MaxWalletQuota)-correction.Quota || quota < -int64(common.MaxWalletQuota)-correction.Quota {
				return nil, ErrBillingEvidenceIntegrity
			}
			quota += correction.Quota
			if (entry.Quota >= 0 && quota < 0) || (entry.Quota < 0 && quota > 0) {
				return nil, ErrBillingEvidenceIntegrity
			}
			entry.StatementCorrections = append(entry.StatementCorrections, BillingStatementCorrection{EntryID: correction.ID, Sequence: correction.Sequence, PostedAt: correction.PostedAt,
				Quota: correction.Quota, BatchID: correction.CorrectionID, BillingGroup: correction.BillingGroup, BillingRate: correction.BillingRate})
		}
		entry.StatementQuota = &quota
	}
	return entries, nil
}

// Freeze source rows and all correction events through the account sequence,
// independently of the mutable batch status/claim. Reversal is another event.
func billingCorrectedPeriodTotals(scope *gorm.DB, userID int, from, to, start, end int64) ([]BillingHour, []BillingModelTotal, error) {
	type modelKey struct{ Model, Group, Rate string }
	byModel := make(map[modelKey]BillingModelTotal)
	byHour := make(map[int64]BillingHour)
	days := make(map[modelKey]map[int64]bool)
	cursor := from - 1
	for {
		entries, err := billingStatementEntryPage(scope, userID, from, to, start, end, cursor, 1000, true)
		if err != nil {
			return nil, nil, err
		}
		rates, err := billingEntryHistoricalRates(scope, userID, entries)
		if err != nil {
			return nil, nil, err
		}
		for _, entry := range entries {
			quota := entry.StatementAmountQuota()
			charge, refund := quota, int64(0)
			if entry.Quota < 0 {
				refund, charge = -quota, 0
			}
			rate := rates[entry.Sequence]
			if len(entry.StatementCorrections) > 0 {
				last := entry.StatementCorrections[len(entry.StatementCorrections)-1]
				rate.Group, rate.Ratio = last.BillingGroup, last.BillingRate
			}
			key := modelKey{entry.ModelName, rate.Group, rate.Ratio}
			total := byModel[key]
			at := entry.PostedAt / 3600 * 3600
			hour := byHour[at]
			if charge < 0 || refund < 0 || charge > int64(common.MaxWalletQuota)-total.Charge || refund > int64(common.MaxWalletQuota)-total.Refund ||
				charge > int64(common.MaxWalletQuota)-hour.Charge || refund > int64(common.MaxWalletQuota)-hour.Refund {
				return nil, nil, errors.New("corrected billing totals exceed exact quota limit")
			}
			total.ModelName, total.BillingGroup, total.BillingRate = key.Model, key.Group, key.Rate
			total.Charge += charge
			total.Refund += refund
			total.Count++
			if entry.Quota >= 0 {
				total.ChargeCount++
			} else {
				total.RefundCount++
			}
			if total.Count == 1 || entry.PostedAt < total.FirstPosted {
				total.FirstPosted = entry.PostedAt
			}
			if entry.PostedAt > total.LastPosted {
				total.LastPosted = entry.PostedAt
			}
			if days[key] == nil {
				days[key] = make(map[int64]bool)
			}
			days[key][(entry.PostedAt+28800)/86400] = true
			total.ActiveDays = int64(len(days[key]))
			byModel[key] = total
			hour.UserID, hour.Hour = userID, at
			hour.Charge += charge
			hour.Refund += refund
			hour.Count++
			if hour.FirstSequence == 0 {
				hour.FirstSequence = entry.Sequence
			}
			hour.LastSequence = entry.Sequence
			byHour[at] = hour
		}
		if len(entries) < 1000 {
			break
		}
		cursor = entries[len(entries)-1].Sequence
	}
	hours := make([]BillingHour, 0, len(byHour))
	totals := make([]BillingModelTotal, 0, len(byModel))
	for _, hour := range byHour {
		hours = append(hours, hour)
	}
	for _, total := range byModel {
		totals = append(totals, total)
	}
	sort.Slice(hours, func(i, j int) bool { return hours[i].Hour < hours[j].Hour })
	sort.Slice(totals, func(i, j int) bool {
		if totals[i].ModelName != totals[j].ModelName {
			return totals[i].ModelName < totals[j].ModelName
		}
		if totals[i].BillingGroup != totals[j].BillingGroup {
			return totals[i].BillingGroup < totals[j].BillingGroup
		}
		return totals[i].BillingRate < totals[j].BillingRate
	})
	return hours, totals, nil
}

// A legacy active bill may already include a correction at its wallet posting
// date. Require that period's statement to be voided first, so two current bills
// cannot claim the same delta. Historical superseded files remain available.
func validateBillingCorrectionPeriodStatements(scope *gorm.DB, statement *BillingStatement) error {
	var active []BillingStatement
	if err := effectiveBillingStatements(scope).Where("user_id = ? AND month <> ?", statement.UserID, statement.Month).Find(&active).Error; err != nil {
		return err
	}
	for _, other := range active {
		var corrections []BillingEntry
		if err := scope.Session(&gorm.Session{NewDB: true}).Where("user_id = ? AND kind = ? AND sequence >= ? AND sequence <= ? AND posted_at >= ? AND posted_at < ?", statement.UserID, "rate_correction", other.FromSequence, other.ToSequence, other.StartAt, other.EndAt).Find(&corrections).Error; err != nil {
			return err
		}
		if len(corrections) == 0 {
			continue
		}
		corrected, err := other.UsesSourcePeriodCorrections()
		if err != nil {
			return err
		}
		if corrected {
			continue
		}
		ids := make([]int64, 0, len(corrections))
		for _, entry := range corrections {
			ids = append(ids, entry.SourceEntryID)
		}
		var source BillingEntry
		if err := scope.Session(&gorm.Session{NewDB: true}).Select("id").Where("user_id = ? AND id IN ? AND posted_at >= ? AND posted_at < ? AND sequence >= ?", statement.UserID, ids, statement.StartAt, statement.EndAt, statement.FromSequence).Limit(1).Find(&source).Error; err != nil {
			return err
		}
		if source.ID != 0 {
			return fmt.Errorf("void statement %s (%s) before creating a replacement to avoid billing the same adjustment twice", other.ID, other.Month)
		}
	}
	return nil
}

func BillingStatementCorrectionsPending(ctx context.Context, statement *BillingStatement) (bool, error) {
	return billingStatementCorrectionsPending(DB.WithContext(ctx), statement)
}

func billingStatementCorrectionsPending(scope *gorm.DB, statement *BillingStatement) (bool, error) {
	corrected, err := statement.UsesSourcePeriodCorrections()
	if err != nil {
		return false, err
	}
	cutoff := int64(0)
	if corrected {
		cutoff = statement.ToSequence
	}
	var account BillingAccount
	if err := scope.Session(&gorm.Session{NewDB: true}).Where("user_id = ?", statement.UserID).Limit(1).Find(&account).Error; err != nil {
		return false, err
	}
	if !corrected {
		// A legacy posting-period bill must also be voided and replaced to remove deltas
		// whose sources belong to an earlier period.
		var postedCorrection BillingEntry
		if err := scope.Session(&gorm.Session{NewDB: true}).Select("id").Where("user_id = ? AND kind = ? AND posted_at >= ? AND posted_at < ? AND sequence >= ? AND sequence <= ?", statement.UserID, "rate_correction", statement.StartAt, statement.EndAt, account.StartSequence, account.Sequence).Limit(1).Find(&postedCorrection).Error; err != nil {
			return false, err
		}
		if postedCorrection.ID != 0 {
			return true, nil
		}
	}
	sources := scope.Session(&gorm.Session{NewDB: true}).Model(&BillingEntry{}).Select("id").
		Where("user_id = ? AND posted_at >= ? AND posted_at < ? AND sequence >= ? AND kind NOT IN ?", statement.UserID, statement.StartAt, statement.EndAt, account.StartSequence, []string{"funding", "rate_correction"})
	var correction BillingEntry
	err = scope.Session(&gorm.Session{NewDB: true}).Select("id").Where("user_id = ? AND kind = ? AND source_entry_id IN (?) AND sequence > ? AND sequence <= ?", statement.UserID, "rate_correction", sources, cutoff, account.Sequence).Limit(1).Find(&correction).Error
	return correction.ID != 0, err
}
