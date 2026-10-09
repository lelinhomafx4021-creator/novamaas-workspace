package model

import (
	"context"
	"strconv"
)

type BillingAccountOption struct {
	ID                int    `json:"id"`
	Username          string `json:"username"`
	CompanyTitle      string `json:"company_title"`
	AccountingStartAt int64  `json:"accounting_start_at"`
}

// SearchBillingAccounts returns only the identity needed by the account picker.
func SearchBillingAccounts(ctx context.Context, keyword string, offset, limit int) ([]BillingAccountOption, int64, error) {
	query := DB.WithContext(ctx).Unscoped().Model(&User{}).
		Joins("LEFT JOIN billing_accounts ON billing_accounts.user_id = users.id")
	pattern := "%" + keyword + "%"
	condition := "users.username LIKE ? OR users.email LIKE ? OR users.phone LIKE ? OR users.display_name LIKE ? OR billing_accounts.company_title LIKE ?"
	args := []interface{}{pattern, pattern, pattern, pattern, pattern}
	if id, err := strconv.Atoi(keyword); err == nil {
		condition += " OR users.id = ?"
		args = append(args, id)
	}
	query = query.Where("("+condition+")", args...)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]BillingAccountOption, 0)
	err := query.Select("users.id, users.username, COALESCE(billing_accounts.company_title, '') AS company_title, COALESCE(billing_accounts.accounting_start_at, 0) AS accounting_start_at").
		Order("users.id DESC").Offset(offset).Limit(limit).Scan(&items).Error
	return items, total, err
}
