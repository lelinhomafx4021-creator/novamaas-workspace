package model

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/samber/hot"
	"golang.org/x/sync/singleflight"
)

// Usage-log statistics are a reference view, never a source of formal evidence.
// The existing (user_id, created_at, id) index bounds each scan to one customer
// and at most one month. Cache raw quota, not currency-converted amounts.
type BillingUsageHour struct {
	Hour      int64 `gorm:"column:hour_bucket"`
	Charge    int64
	Refund    int64
	Count     int64 `gorm:"column:records"`
	Invalid   int64
	LastLogAt int64
}

type BillingUsageWindow struct {
	Hours     []BillingUsageHour
	CheckedAt int64
}

var billingUsageCache = hot.NewHotCache[string, BillingUsageWindow](hot.LRU, 128).WithTTL(15 * time.Second).Build()
var billingUsageQueries singleflight.Group

// This uncached, index-bounded existence check also covers a partial first hour.
// An empty formal ledger must not certify existing legacy usage as zero.
func HasBillingUsageRecords(ctx context.Context, userID int, start, end int64) (bool, error) {
	if userID <= 0 || start < 0 || end <= start || end-start > 31*86400 {
		return false, errors.New("invalid usage billing period")
	}
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var log Log
	result := LOG_DB.WithContext(queryCtx).Select("id").
		Where("user_id = ? AND created_at >= ? AND created_at < ? AND type IN ?", userID, start, end, []int{LogTypeConsume, LogTypeRefund}).Limit(1).Find(&log)
	return result.RowsAffected > 0, result.Error
}

func GetBillingUsageHours(ctx context.Context, userID int, start, end int64) (BillingUsageWindow, error) {
	if userID <= 0 || start < 0 || end <= start || end-start > 31*86400 {
		return BillingUsageWindow{}, errors.New("invalid usage billing period")
	}
	var correctionVersion int64
	if err := DB.WithContext(ctx).Model(&BillingEntry{}).Where("user_id = ? AND kind = ?", userID, "rate_correction").Select("COALESCE(MAX(id), 0)").Scan(&correctionVersion).Error; err != nil {
		return BillingUsageWindow{}, err
	}
	key := fmt.Sprintf("%p:%p:%d:%d:%d:%d", DB, LOG_DB, userID, start, end, correctionVersion)
	if cached, found, _ := billingUsageCache.Get(key); found {
		return cached, nil
	}
	result := billingUsageQueries.DoChan(key, func() (any, error) {
		if cached, found, _ := billingUsageCache.Get(key); found {
			return cached, nil
		}
		// One disconnected waiter must not cancel a shared query for all users.
		queryCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		bucket := "CAST(created_at / 3600 AS BIGINT) * 3600"
		if common.UsingLogDatabase(common.DatabaseTypeMySQL) {
			bucket = "(created_at DIV 3600) * 3600"
		} else if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
			bucket = "intDiv(toInt64(created_at), 3600) * 3600"
		}
		window := BillingUsageWindow{Hours: make([]BillingUsageHour, 0), CheckedAt: common.GetTimestamp()}
		err := LOG_DB.WithContext(queryCtx).Model(&Log{}).
			Where("user_id = ? AND created_at >= ? AND created_at < ? AND created_at <= ?", userID, start, end, window.CheckedAt).
			Where("type IN ?", []int{LogTypeConsume, LogTypeRefund}).
			Select(bucket + " AS hour_bucket, SUM(CASE WHEN type = 2 AND quota >= 0 THEN quota ELSE 0 END) AS charge, SUM(CASE WHEN type = 6 AND quota >= 0 THEN quota ELSE 0 END) AS refund, COUNT(*) AS records, SUM(CASE WHEN quota < 0 THEN 1 ELSE 0 END) AS invalid, MAX(created_at) AS last_log_at").
			Group("hour_bucket").Order("hour_bucket asc").Limit(744).Scan(&window.Hours).Error
		if err != nil {
			return BillingUsageWindow{}, err
		}
		adjustments, err := billingEffectiveLogs(queryCtx, CostAccountingFilter{UserID: userID, StartTimestamp: start, EndTimestamp: end - 1})
		if err != nil {
			return BillingUsageWindow{}, err
		}
		byHour := make(map[int64]int, len(window.Hours))
		for i := range window.Hours {
			byHour[window.Hours[i].Hour] = i
		}
		for _, adjustment := range adjustments {
			hour := adjustment.Original.CreatedAt / 3600 * 3600
			index, found := byHour[hour]
			if !found {
				return BillingUsageWindow{}, ErrBillingEvidenceIntegrity
			}
			delta := adjustment.Quota
			if adjustment.Original.Type == LogTypeRefund {
				delta = -delta
				window.Hours[index].Refund += delta - int64(adjustment.Original.Quota)
			} else {
				window.Hours[index].Charge += delta - int64(adjustment.Original.Quota)
			}
		}
		for _, hour := range window.Hours {
			if hour.Invalid != 0 || hour.Charge < 0 || hour.Refund < 0 || hour.Charge > int64(common.MaxWalletQuota) || hour.Refund > int64(common.MaxWalletQuota) {
				return BillingUsageWindow{}, errors.New("usage billing data contains invalid quota")
			}
		}
		billingUsageCache.Set(key, window)
		return window, nil
	})
	select {
	case <-ctx.Done():
		return BillingUsageWindow{}, ctx.Err()
	case value := <-result:
		if value.Err != nil {
			return BillingUsageWindow{}, value.Err
		}
		return value.Val.(BillingUsageWindow), nil
	}
}

type BillingUsageCursor struct {
	At        int64  `json:"at"`
	ID        int    `json:"id"`
	RequestID string `json:"request_id"`
}

// Deliberately select a safe subset: no request bodies, IPs, token names,
// provider credentials, channel details, or administrator-only log fields.
func GetBillingUsageRecords(ctx context.Context, userID int, start, end int64, before *BillingUsageCursor) ([]Log, error) {
	if userID <= 0 || start < 0 || end-start != 3600 {
		return nil, errors.New("invalid usage billing hour")
	}
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	query := LOG_DB.WithContext(queryCtx).Model(&Log{}).
		Select("id", "user_id", "created_at", "type", "model_name", "quota", "request_id", "token_id", "channel_id").
		Where("user_id = ? AND created_at >= ? AND created_at < ? AND created_at <= ?", userID, start, end, common.GetTimestamp()).
		Where("type IN ?", []int{LogTypeConsume, LogTypeRefund})
	if before != nil {
		query = query.Where("created_at < ? OR (created_at = ? AND id < ?) OR (created_at = ? AND id = ? AND request_id < ?)", before.At, before.At, before.ID, before.At, before.ID, before.RequestID)
	}
	records := make([]Log, 0)
	err := query.Order("created_at desc").Order("id desc").Order("request_id desc").Limit(101).Find(&records).Error
	if err != nil {
		return nil, err
	}
	adjustments, err := billingEffectiveLogs(queryCtx, CostAccountingFilter{UserID: userID, StartTimestamp: start, EndTimestamp: end - 1})
	if err != nil {
		return nil, err
	}
	pointers := make([]*Log, 0, len(records))
	for i := range records {
		pointers = append(pointers, &records[i])
	}
	err = projectBillingEffectiveLogs(pointers, adjustments)
	// These records remain a safe subset even though metadata was needed
	// internally to project the correction.
	for i := range records {
		records[i].Other = ""
		records[i].UserId, records[i].TokenId, records[i].ChannelId = 0, 0, 0
		records[i].Group = ""
	}
	return records, err
}
