package model

import (
	"context"
	"math"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// The dashboard's existing data export records consumption only. Apply pricing
// changes to those same records; no new requests, tokens or refund rows appear.
// Refunds remain in the net usage and statement views.
type billingDashboardCorrection struct {
	Source        QuotaData
	Quota         int64
	Group         string
	Tokens        int
	OriginalQuota int64
}

type billingDashboardSourceKey struct {
	UserID, TokenID, ChannelID int
	Username, ModelName, Group string
	Hour                       int64
}

type billingDashboardTaskKey struct {
	UserID int
	TaskID string
}

type billingDashboardQuotaKey struct {
	UserID              int
	Username, ModelName string
	Hour                int64
}

type billingFlowKey struct {
	UserID, TokenID, ChannelID           int
	Username, NodeName, Group, ModelName string
}

func billingDashboardCorrections(userID int, username string, start, end int64) ([]billingDashboardCorrection, error) {
	last := end
	if last <= math.MaxInt64-3599 {
		last += 3599
	}
	adjustments, err := billingEffectiveLogs(context.Background(), CostAccountingFilter{UserID: userID, Username: username, StartTimestamp: start, EndTimestamp: last, LogType: LogTypeConsume})
	if err != nil || len(adjustments) == 0 {
		return nil, err
	}
	conditions, args := make([]string, 0), make([]any, 0)
	for _, adjustment := range adjustments {
		log := adjustment.Original
		at := log.CreatedAt / 3600 * 3600
		if at < start || at > end {
			continue
		}
		conditions = append(conditions, "(user_id = ? AND username = ? AND model_name = ? AND created_at = ? AND use_group = ? AND token_id = ? AND channel_id = ?)")
		args = append(args, log.UserId, log.Username, log.ModelName, at, log.Group, log.TokenId, log.ChannelId)
	}
	if len(conditions) == 0 {
		return nil, nil
	}
	var exported []QuotaData
	seen := make(map[int]bool)
	for offset := 0; offset < len(conditions); offset += 100 {
		end := min(offset+100, len(conditions))
		var page []QuotaData
		if err := DB.Where(strings.Join(conditions[offset:end], " OR "), args[offset*7:end*7]...).Find(&page).Error; err != nil {
			return nil, err
		}
		for _, row := range page {
			if !seen[row.Id] {
				seen[row.Id] = true
				exported = append(exported, row)
			}
		}
	}
	bySource := make(map[billingDashboardSourceKey][]QuotaData)
	for _, row := range exported {
		key := billingDashboardSourceKey{UserID: row.UserID, Username: row.Username, ModelName: row.ModelName, Hour: row.CreatedAt, Group: row.UseGroup, TokenID: row.TokenID, ChannelID: row.ChannelID}
		bySource[key] = append(bySource[key], row)
	}
	taskIDs := make([]string, 0)
	uniqueTasks := make(map[string]bool)
	for _, adjustment := range adjustments {
		if adjustment.TaskID != "" && !uniqueTasks[adjustment.TaskID] {
			uniqueTasks[adjustment.TaskID] = true
			taskIDs = append(taskIDs, adjustment.TaskID)
		}
	}
	var tasks []Task
	for offset := 0; offset < len(taskIDs); offset += 500 {
		var page []Task
		if err := DB.Select("user_id", "task_id", "private_data").Where("task_id IN ?", taskIDs[offset:min(offset+500, len(taskIDs))]).Find(&page).Error; err != nil {
			return nil, err
		}
		tasks = append(tasks, page...)
	}
	nodes := make(map[billingDashboardTaskKey]string)
	for _, task := range tasks {
		key := billingDashboardTaskKey{UserID: task.UserId, TaskID: task.TaskID}
		if _, found := nodes[key]; found {
			return nil, ErrBillingEvidenceIntegrity
		}
		nodes[key] = task.PrivateData.NodeName
	}
	result := make([]billingDashboardCorrection, 0, len(adjustments))
	for _, adjustment := range adjustments {
		log := adjustment.Original
		at := log.CreatedAt / 3600 * 3600
		if at < start || at > end {
			continue
		}
		rows := bySource[billingDashboardSourceKey{UserID: log.UserId, Username: log.Username, ModelName: log.ModelName, Hour: at, Group: log.Group, TokenID: log.TokenId, ChannelID: log.ChannelId}]
		if len(rows) == 0 {
			// Export may have been disabled for this historical request.
			continue
		}
		var source *QuotaData
		node := nodes[billingDashboardTaskKey{UserID: log.UserId, TaskID: adjustment.TaskID}]
		for i := range rows {
			if len(rows) > 1 && (node == "" || rows[i].NodeName != node) {
				continue
			}
			if source != nil {
				return nil, ErrBillingEvidenceIntegrity
			}
			source = &rows[i]
		}
		if source == nil || source.Count <= 0 || source.Quota < log.Quota {
			return nil, ErrBillingEvidenceIntegrity
		}
		result = append(result, billingDashboardCorrection{Source: *source, Quota: adjustment.Quota, Group: adjustment.Group, Tokens: log.PromptTokens + log.CompletionTokens, OriginalQuota: int64(log.Quota)})
	}
	return result, nil
}

func projectBillingQuotaData(rows []*QuotaData, corrections []billingDashboardCorrection, dimensions string) error {
	byKey := make(map[billingDashboardQuotaKey]*QuotaData, len(rows))
	for _, row := range rows {
		key := billingDashboardQuotaKey{ModelName: row.ModelName, Hour: row.CreatedAt}
		if dimensions == "user" {
			key = billingDashboardQuotaKey{Username: row.Username, Hour: row.CreatedAt}
		} else if dimensions == "user_model" {
			key = billingDashboardQuotaKey{UserID: row.UserID, Username: row.Username, ModelName: row.ModelName, Hour: row.CreatedAt}
		}
		byKey[key] = row
	}
	for _, correction := range corrections {
		source := correction.Source
		key := billingDashboardQuotaKey{ModelName: source.ModelName, Hour: source.CreatedAt}
		if dimensions == "user" {
			key = billingDashboardQuotaKey{Username: source.Username, Hour: source.CreatedAt}
		} else if dimensions == "user_model" {
			key = billingDashboardQuotaKey{UserID: source.UserID, Username: source.Username, ModelName: source.ModelName, Hour: source.CreatedAt}
		}
		row, found := byKey[key]
		if !found {
			return ErrBillingEvidenceIntegrity
		}
		// A source aggregate can contain several requests. Only this source's
		// original amount is replaced; counts and token volume are untouched.
		value := int64(row.Quota) + correction.Quota - correction.OriginalQuota
		if value < 0 || value > int64(common.MaxWalletQuota) {
			return ErrBillingEvidenceIntegrity
		}
		row.Quota = int(value)
	}
	return nil
}

func projectBillingFlowData(rows []*FlowQuotaData, corrections []billingDashboardCorrection, role int) ([]*FlowQuotaData, error) {
	keyFor := func(row *FlowQuotaData) billingFlowKey {
		if role >= common.RoleRootUser {
			return billingFlowKey{UserID: row.UserID, Username: row.Username, NodeName: row.NodeName, TokenID: row.TokenID, Group: row.UseGroup, ModelName: row.ModelName, ChannelID: row.ChannelID}
		}
		if role >= common.RoleAdminUser {
			return billingFlowKey{UserID: row.UserID, Username: row.Username, Group: row.UseGroup, ModelName: row.ModelName, ChannelID: row.ChannelID}
		}
		return billingFlowKey{TokenID: row.TokenID, Group: row.UseGroup, ModelName: row.ModelName}
	}
	byKey := make(map[billingFlowKey]*FlowQuotaData, len(rows))
	for _, row := range rows {
		byKey[keyFor(row)] = row
	}
	for _, correction := range corrections {
		source := correction.Source
		if source.UseGroup == "" {
			continue
		}
		original := &FlowQuotaData{UserID: source.UserID, Username: source.Username, NodeName: source.NodeName, TokenID: source.TokenID, UseGroup: source.UseGroup, ModelName: source.ModelName, ChannelID: source.ChannelID}
		previous, found := byKey[keyFor(original)]
		if !found {
			return nil, ErrBillingEvidenceIntegrity
		}
		if correction.Group == source.UseGroup {
			value := int64(previous.Quota) + correction.Quota - correction.OriginalQuota
			if value < 0 || value > int64(common.MaxWalletQuota) {
				return nil, ErrBillingEvidenceIntegrity
			}
			previous.Quota = int(value)
			continue
		}
		previous.Quota -= int(correction.OriginalQuota)
		previous.Count--
		previous.TokenUsed -= correction.Tokens
		original.UseGroup = correction.Group
		current, found := byKey[keyFor(original)]
		if !found {
			current = original
			byKey[keyFor(current)] = current
			rows = append(rows, current)
		}
		if int64(current.Quota)+correction.Quota > int64(common.MaxWalletQuota) || previous.Quota < 0 || previous.Count < 0 || previous.TokenUsed < 0 {
			return nil, ErrBillingEvidenceIntegrity
		}
		current.Quota += int(correction.Quota)
		current.Count++
		current.TokenUsed += correction.Tokens
	}
	result := make([]*FlowQuotaData, 0, len(rows))
	for _, row := range rows {
		if row.Count > 0 || row.Quota != 0 || row.TokenUsed != 0 {
			result = append(result, row)
		}
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].Quota > result[j].Quota })
	return result, nil
}
