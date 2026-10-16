package model

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

const (
	CacheUsageAuditPageSize   = 2000
	CacheUsageAuditMaxRecords = 200000
)

var ErrCacheUsageAuditTooLarge = errors.New("cache usage audit exceeds 200000 records; narrow the period or filters")

type CacheUsageAuditFilter struct {
	UserID  int
	TokenID int
	StartAt int64 // inclusive unix seconds
	EndAt   int64 // exclusive unix seconds
	Model   string
}

// ScanCacheUsageAuditLogs streams settled text logs page by page so large
// periods never sit in memory at once. Billing corrections are projected onto
// each page, matching the amounts shown in consumption lookups.
func ScanCacheUsageAuditLogs(ctx context.Context, filter CacheUsageAuditFilter, visit func([]*Log) error) error {
	query := LOG_DB.WithContext(ctx).Model(&Log{}).
		Where("type IN ? AND created_at >= ? AND created_at < ?", []int{LogTypeConsume, LogTypeRefund}, filter.StartAt, filter.EndAt)
	if filter.UserID > 0 {
		query = query.Where("user_id = ?", filter.UserID)
	}
	if filter.TokenID > 0 {
		query = query.Where("token_id = ?", filter.TokenID)
	}
	if filter.Model != "" {
		query = query.Where("model_name = ?", filter.Model)
	}
	order := "created_at asc, id asc"
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		order = "created_at asc, request_id asc"
	}
	columns := []string{"id", "user_id", "username", "created_at", "type", "token_id", "token_name", "channel_id",
		"model_name", "group", "request_id", "upstream_request_id", "prompt_tokens", "completion_tokens", "quota", "is_stream", "other"}
	for offset := 0; ; offset += CacheUsageAuditPageSize {
		if offset >= CacheUsageAuditMaxRecords {
			return ErrCacheUsageAuditTooLarge
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		page := make([]*Log, 0, CacheUsageAuditPageSize)
		if err := query.Session(&gorm.Session{}).Select(columns).Order(order).
			Offset(offset).Limit(CacheUsageAuditPageSize).Find(&page).Error; err != nil {
			return err
		}
		if len(page) == 0 {
			return nil
		}
		adjustments, err := billingEffectiveLogsForPage(ctx, page)
		if err != nil {
			return err
		}
		if err := projectBillingEffectiveLogs(page, adjustments); err != nil {
			return err
		}
		if err := visit(page); err != nil {
			return err
		}
		if len(page) < CacheUsageAuditPageSize {
			return nil
		}
	}
}

type CacheUsageAuditKey struct {
	ID      int    `json:"id"`
	UserID  int    `json:"user_id"`
	Name    string `json:"name"`
	Deleted bool   `json:"deleted"`
}

// SearchCacheUsageAuditKeys includes deleted keys because historical audits
// must still resolve them. Key secrets are never selected.
func SearchCacheUsageAuditKeys(ctx context.Context, userID int, keyword string) ([]CacheUsageAuditKey, error) {
	query := DB.WithContext(ctx).Unscoped().Model(&Token{})
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		if id, err := strconv.Atoi(keyword); err == nil && id > 0 {
			query = query.Where("id = ?", id)
		} else {
			pattern, err := sanitizeLikePattern(strings.ReplaceAll(keyword, "%", ""))
			if err != nil {
				return nil, err
			}
			query = query.Where("name LIKE ? ESCAPE '!'", "%"+pattern+"%")
		}
	}
	var rows []struct {
		ID        int
		UserID    int
		Name      string
		DeletedAt gorm.DeletedAt
	}
	if err := query.Select("id", "user_id", "name", "deleted_at").Order("id desc").Limit(50).Find(&rows).Error; err != nil {
		return nil, err
	}
	keys := make([]CacheUsageAuditKey, 0, len(rows))
	for _, row := range rows {
		keys = append(keys, CacheUsageAuditKey{ID: row.ID, UserID: row.UserID, Name: row.Name, Deleted: row.DeletedAt.Valid})
	}
	return keys, nil
}
