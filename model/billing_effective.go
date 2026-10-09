package model

import (
	"context"
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// Billing evidence remains immutable. These projections replace customer-facing
// values only, attributing every applied/reversed event to the source request.
// In particular, a separate log database never needs a cross-database SQL join.
type billingEffectiveLog struct {
	Original Log
	Quota    int64 // Signed consumption/refund, unlike Log.Quota.
	Group    string
	Rate     string
	Pricing  string
	TaskID   string
}

type billingEffectivePricingRow struct {
	BatchID         string
	SourceEntryID   int64
	TaskID          string
	TargetPricing   string
	PreviousBatchID string
}

type billingEffectiveLogKey struct {
	UserID, ID, Type, TokenID, ChannelID, Quota int
	CreatedAt                                   int64
	RequestID, ModelName                        string
}

func billingLogIdentity(log *Log) billingEffectiveLogKey {
	// ClickHouse IDs are display values, not durable source identifiers.
	if !common.UsingLogDatabase(common.DatabaseTypeClickHouse) && log.Id > 0 {
		return billingEffectiveLogKey{UserID: log.UserId, ID: log.Id}
	}
	return billingEffectiveLogKey{UserID: log.UserId, Type: log.Type, CreatedAt: log.CreatedAt, RequestID: log.RequestId, ModelName: log.ModelName, TokenID: log.TokenId, ChannelID: log.ChannelId, Quota: log.Quota}
}

func billingEffectiveLogs(ctx context.Context, filter CostAccountingFilter) ([]billingEffectiveLog, error) {
	result := make([]billingEffectiveLog, 0)
	if filter.LogType != LogTypeUnknown && filter.LogType != LogTypeConsume && filter.LogType != LogTypeRefund {
		return result, nil
	}
	// Only corrected sources are read. Ordinary logs remain on their existing
	// indexed query path, irrespective of the size of the immutable ledger.
	correctedSources := DB.WithContext(ctx).Model(&BillingEntry{}).Select("source_entry_id").Distinct().Where("source_entry_id > 0 AND kind = ?", "rate_correction")
	if filter.UserID != 0 {
		correctedSources = correctedSources.Where("user_id = ?", filter.UserID)
	} else if len(filter.userIDs) > 0 {
		correctedSources = correctedSources.Where("user_id IN ?", filter.userIDs)
	}
	query := DB.WithContext(ctx).Model(&BillingEntry{}).Where("kind <> ? AND id IN (?)", "rate_correction", correctedSources)
	if filter.UserID != 0 {
		query = query.Where("user_id = ?", filter.UserID)
	} else if len(filter.userIDs) > 0 {
		query = query.Where("user_id IN ?", filter.userIDs)
	}
	if filter.StartTimestamp > 0 {
		query = query.Where("posted_at >= ?", max(int64(0), filter.StartTimestamp-60))
	}
	if filter.EndTimestamp != 0 {
		end := filter.EndTimestamp
		if end <= math.MaxInt64-60 {
			end += 60
		}
		query = query.Where("posted_at <= ?", end)
	}
	if filter.ModelName != "" && !strings.Contains(filter.ModelName, "%") {
		query = query.Where("model_name = ?", filter.ModelName)
	}
	var cursor int64
	used := make(map[billingEffectiveLogKey]bool)
	for {
		var sources []BillingEntry
		if err := query.Session(&gorm.Session{}).Where("id > ?", cursor).Order("id asc").Limit(100).Find(&sources).Error; err != nil {
			return nil, err
		}
		if len(sources) == 0 {
			return result, nil
		}
		ids := make([]int64, 0, len(sources))
		for _, source := range sources {
			ids = append(ids, source.ID)
		}
		var events []BillingEntry
		if err := DB.WithContext(ctx).Where("kind = ? AND source_entry_id IN ?", "rate_correction", ids).Order("sequence asc").Find(&events).Error; err != nil {
			return nil, err
		}
		var rows []billingEffectivePricingRow
		if err := DB.WithContext(ctx).Table("billing_correction_rows").Where("source_entry_id IN ?", ids).Find(&rows).Error; err != nil {
			return nil, err
		}
		bySource := make(map[int64][]BillingEntry)
		byBatch := make(map[string]billingEffectivePricingRow)
		for _, event := range events {
			bySource[event.SourceEntryID] = append(bySource[event.SourceEntryID], event)
		}
		for _, row := range rows {
			key := row.BatchID + ":" + strconv.FormatInt(row.SourceEntryID, 10)
			if _, found := byBatch[key]; found {
				return nil, ErrBillingEvidenceIntegrity
			}
			byBatch[key] = row
		}
		// Stable ownership-qualified identities are used on both relational log
		// databases and ClickHouse. Task refunds can have a new request ID, so
		// their immutable task_id metadata supplies the fallback identity.
		conditions := make([]string, 0, len(sources))
		values := make([]any, 0)
		for _, source := range sources {
			if source.SourceLogID > 0 && !common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
				conditions = append(conditions, "(user_id = ? AND id = ?)")
				values = append(values, source.UserID, source.SourceLogID)
			} else if source.Kind != "task_adjustment" && source.RequestID != "" {
				conditions = append(conditions, "(user_id = ? AND request_id = ?)")
				values = append(values, source.UserID, source.RequestID)
			} else {
				conditions = append(conditions, "(user_id = ? AND model_name = ? AND token_id = ? AND created_at >= ? AND created_at <= ?)")
				values = append(values, source.UserID, source.ModelName, source.TokenID, source.PostedAt-60, source.PostedAt+60)
			}
		}
		var candidates []Log
		if err := LOG_DB.WithContext(ctx).Where("type IN ?", []int{LogTypeConsume, LogTypeRefund}).Where(strings.Join(conditions, " OR "), values...).Find(&candidates).Error; err != nil {
			return nil, err
		}
		// Reuse the database's filtering semantics for names and wildcard
		// escaping. Group is evaluated after projecting the latest event.
		filtered := filter
		filtered.Group = ""
		filtered.IncludeBillingCorrections = false
		matchingQuery, err := applyLogStatisticsFilter(LOG_DB.WithContext(ctx).Model(&Log{}), filtered)
		if err != nil {
			return nil, err
		}
		var matching []Log
		if err := matchingQuery.Where(strings.Join(conditions, " OR "), values...).Find(&matching).Error; err != nil {
			return nil, err
		}
		included := make(map[billingEffectiveLogKey]bool, len(matching))
		for i := range matching {
			included[billingLogIdentity(&matching[i])] = true
		}
		for _, source := range sources {
			adjustments := bySource[source.ID]
			if len(adjustments) == 0 {
				continue
			}
			quota := source.Quota
			if quota > int64(common.MaxWalletQuota) || quota < -int64(common.MaxWalletQuota) {
				return nil, ErrBillingEvidenceIntegrity
			}
			for _, event := range adjustments {
				if event.UserID != source.UserID || event.Sequence <= source.Sequence || event.ModelName != source.ModelName || event.TokenID != source.TokenID || event.CorrectionID == "" ||
					event.Quota > int64(common.MaxWalletQuota) || event.Quota < -int64(common.MaxWalletQuota) || quota > int64(common.MaxWalletQuota)-event.Quota || quota < -int64(common.MaxWalletQuota)-event.Quota {
					return nil, ErrBillingEvidenceIntegrity
				}
				quota += event.Quota
				if (source.Quota >= 0 && quota < 0) || (source.Quota < 0 && quota > 0) {
					return nil, ErrBillingEvidenceIntegrity
				}
			}
			absolute, logType := source.Quota, LogTypeConsume
			if absolute < 0 || source.Kind == "refund" || source.Kind == "history_refund" {
				absolute, logType = -absolute, LogTypeRefund
			}
			var original *Log
			for i := range candidates {
				log := &candidates[i]
				if log.UserId != source.UserID || log.ModelName != source.ModelName || log.TokenId != source.TokenID || log.Type != logType || int64(log.Quota) != absolute {
					continue
				}
				if source.SourceLogID > 0 && !common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
					if log.Id != source.SourceLogID {
						continue
					}
				} else if source.Kind != "task_adjustment" && source.RequestID != "" {
					if log.RequestId != source.RequestID {
						continue
					}
				} else {
					var metadata struct {
						TaskID string `json:"task_id"`
					}
					if source.RequestID == "" || log.CreatedAt < source.PostedAt-60 || log.CreatedAt > source.PostedAt+60 || common.UnmarshalJsonStr(log.Other, &metadata) != nil || metadata.TaskID != source.RequestID {
						continue
					}
				}
				if original != nil {
					return nil, errors.New("corrected usage has ambiguous original log evidence")
				}
				original = log
			}
			if original == nil {
				// Retention can remove old logs without removing permanent ledger
				// evidence. Do not manufacture usage rows that no longer exist.
				continue
			}
			identity := billingLogIdentity(original)
			if used[identity] {
				return nil, errors.New("multiple corrected ledger sources refer to one usage log")
			}
			used[identity] = true
			if !included[identity] {
				continue
			}
			last := adjustments[len(adjustments)-1]
			row := byBatch[last.CorrectionID+":"+strconv.FormatInt(source.ID, 10)]
			pricing := row.TargetPricing
			if strings.HasPrefix(last.EventKey, "correction:reverse:") {
				pricing = byBatch[row.PreviousBatchID+":"+strconv.FormatInt(source.ID, 10)].TargetPricing
			}
			result = append(result, billingEffectiveLog{Original: *original, Quota: quota, Group: last.BillingGroup, Rate: last.BillingRate, Pricing: pricing, TaskID: row.TaskID})
		}
		cursor = sources[len(sources)-1].ID
		if len(sources) < 100 {
			return result, nil
		}
	}
}

func projectBillingEffectiveLogs(logs []*Log, adjustments []billingEffectiveLog) error {
	byIdentity := make(map[billingEffectiveLogKey]billingEffectiveLog, len(adjustments))
	for _, adjustment := range adjustments {
		byIdentity[billingLogIdentity(&adjustment.Original)] = adjustment
	}
	for _, log := range logs {
		adjustment, found := byIdentity[billingLogIdentity(log)]
		if !found {
			continue
		}
		quota := adjustment.Quota
		if log.Type == LogTypeRefund {
			quota = -quota
		}
		if quota < 0 || quota > int64(common.MaxWalletQuota) {
			return ErrBillingEvidenceIntegrity
		}
		log.Quota, log.Group = int(quota), adjustment.Group // exact-wallet bounded above
		if log.RevenueQuota != nil {
			revenue := adjustment.Quota
			log.RevenueQuota = &revenue
			if log.CostQuota != nil {
				profit := revenue - *log.CostQuota
				log.ProfitQuota = &profit
			}
		}
		metadata := make(map[string]any)
		if log.Other != "" && common.UnmarshalJsonStr(log.Other, &metadata) != nil {
			return ErrBillingEvidenceIntegrity
		}
		if metadata == nil {
			metadata = make(map[string]any)
		}
		if adjustment.TaskID != "" {
			metadata["is_task"] = true
			metadata["task_id"] = adjustment.TaskID
		}
		var rate float64
		if common.UnmarshalJsonStr(adjustment.Rate, &rate) != nil || math.IsNaN(rate) || math.IsInf(rate, 0) || rate < 0 || rate > 10 {
			return ErrBillingEvidenceIntegrity
		}
		metadata["group_ratio"] = rate
		if _, found := metadata["group"]; found {
			metadata["group"] = adjustment.Group
		}
		if _, found := metadata["fee_quota"]; found {
			metadata["fee_quota"] = quota
		}
		if _, found := metadata["user_group_ratio"]; found {
			metadata["user_group_ratio"] = rate
		}
		if adjustment.Pricing != "" {
			var pricing map[string]any
			if common.UnmarshalJsonStr(adjustment.Pricing, &pricing) != nil {
				return ErrBillingEvidenceIntegrity
			}
			for _, name := range []string{"model_price", "model_ratio", "other_ratios", "total_tokens", "resolution", "has_video", "per_call_billing"} {
				if value, found := pricing[name]; found {
					metadata[name] = value
				}
			}
			if pricing["pricing_mode"] == "tokens" {
				// A task can retain its submission reservation's model_price even
				// after token settlement. The visible fee explanation must use the
				// effective token price rather than that reservation unit price.
				metadata["model_price"] = 0
			}
		}
		metadata["billing_correction_applied"] = true
		body, err := common.Marshal(metadata)
		if err != nil {
			return err
		}
		log.Other = string(body)
	}
	return nil
}

func billingEffectiveLogsForPage(ctx context.Context, logs []*Log) ([]billingEffectiveLog, error) {
	if len(logs) == 0 {
		return nil, nil
	}
	filter := CostAccountingFilter{UserID: logs[0].UserId, StartTimestamp: logs[0].CreatedAt, EndTimestamp: logs[0].CreatedAt}
	users := make(map[int]bool)
	for _, log := range logs {
		if !users[log.UserId] {
			users[log.UserId] = true
			filter.userIDs = append(filter.userIDs, log.UserId)
		}
		if log.UserId != filter.UserID {
			filter.UserID = 0
		}
		filter.StartTimestamp = min(filter.StartTimestamp, log.CreatedAt)
		filter.EndTimestamp = max(filter.EndTimestamp, log.CreatedAt)
	}
	return billingEffectiveLogs(ctx, filter)
}

// Group changes must participate in filtering before counts and pagination.
// The identity predicates remain valid when relational log IDs are unavailable.
func applyBillingEffectiveGroup(query *gorm.DB, group string, adjustments []billingEffectiveLog) *gorm.DB {
	if group == "" {
		return query
	}
	includeIDs, excludeIDs := make([]int, 0), make([]int, 0)
	includeKeys, excludeKeys := make([][]any, 0), make([][]any, 0)
	for _, adjustment := range adjustments {
		log := adjustment.Original
		if !common.UsingLogDatabase(common.DatabaseTypeClickHouse) && log.Id > 0 {
			if adjustment.Group == group && log.Group != group {
				includeIDs = append(includeIDs, log.Id)
			} else if adjustment.Group != group && log.Group == group {
				excludeIDs = append(excludeIDs, log.Id)
			}
			continue
		}
		identity := []any{log.UserId, log.RequestId, log.Type, log.CreatedAt, log.ModelName, log.Quota, log.TokenId, log.ChannelId}
		if adjustment.Group == group && log.Group != group {
			includeKeys = append(includeKeys, identity)
		} else if adjustment.Group != group && log.Group == group {
			excludeKeys = append(excludeKeys, identity)
		}
	}
	include, exclude := make([]string, 0), make([]string, 0)
	includeArgs, excludeArgs := make([]any, 0), make([]any, 0)
	if len(includeIDs) > 0 {
		include = append(include, "logs.id IN ?")
		includeArgs = append(includeArgs, includeIDs)
	}
	if len(excludeIDs) > 0 {
		exclude = append(exclude, "logs.id IN ?")
		excludeArgs = append(excludeArgs, excludeIDs)
	}
	columns := "(logs.user_id, logs.request_id, logs.type, logs.created_at, logs.model_name, logs.quota, logs.token_id, logs.channel_id) IN ?"
	if len(includeKeys) > 0 {
		include = append(include, columns)
		includeArgs = append(includeArgs, includeKeys)
	}
	if len(excludeKeys) > 0 {
		exclude = append(exclude, columns)
		excludeArgs = append(excludeArgs, excludeKeys)
	}
	condition, args := "logs."+logGroupCol+" = ?", []any{group}
	if len(exclude) > 0 {
		condition = "(" + condition + " AND NOT (" + strings.Join(exclude, " OR ") + "))"
		args = append(args, excludeArgs...)
	}
	if len(include) > 0 {
		condition = "(" + condition + " OR " + strings.Join(include, " OR ") + ")"
		args = append(args, includeArgs...)
	}
	return query.Where(condition, args...)
}

func sumBillingEffectiveLogStatistics(filter CostAccountingFilter) (LogStatistics, error) {
	if filter.LogType == LogTypeBillingCorrection {
		// An explicit administrative audit query retains execution-date deltas.
		return sumLogFinancialStatistics(filter)
	}
	base := filter
	base.IncludeBillingCorrections = false
	statistics, err := sumLogFinancialStatistics(base)
	if err != nil {
		return statistics, err
	}
	adjustments, err := billingEffectiveLogs(context.Background(), filter)
	if err != nil {
		return statistics, err
	}
	if len(adjustments) == 0 {
		return statistics, nil
	}
	for _, adjustment := range adjustments {
		originalIncluded := filter.Group == "" || filter.Group == adjustment.Original.Group
		effectiveIncluded := filter.Group == "" || filter.Group == adjustment.Group
		old, current := int64(0), int64(0)
		if originalIncluded {
			old = int64(adjustment.Original.Quota)
		}
		if effectiveIncluded {
			current = adjustment.Quota
			if adjustment.Original.Type == LogTypeRefund {
				current = -current
			}
		}
		if adjustment.Original.Type == LogTypeRefund {
			statistics.RefundQuota, err = billingProjectionQuotaTotal(statistics.RefundQuota, current-old)
		} else {
			statistics.Quota, err = billingProjectionQuotaTotal(statistics.Quota, current-old)
		}
		if err != nil {
			return LogStatistics{}, err
		}
		if originalIncluded != effectiveIncluded {
			delta := int64(-1)
			if effectiveIncluded {
				delta = 1
			}
			statistics.Records += delta
			if adjustment.Original.Type == LogTypeConsume {
				statistics.Requests += delta
			}
		}
	}
	if statistics.Quota < 0 || statistics.RefundQuota < 0 {
		return LogStatistics{}, ErrBillingEvidenceIntegrity
	}
	statistics.RevenueQuota = statistics.Quota - statistics.RefundQuota
	return statistics, nil
}

func sumBillingEffectiveLogBuckets(filter CostAccountingFilter, bucketSeconds, offsetSeconds int64) ([]CostAccountingBucket, error) {
	base := filter
	if filter.LogType != LogTypeBillingCorrection {
		base.IncludeBillingCorrections = false
	}
	buckets, err := sumLogAccountingBuckets(base, bucketSeconds, offsetSeconds)
	if err != nil || filter.LogType == LogTypeBillingCorrection {
		return buckets, err
	}
	adjustments, err := billingEffectiveLogs(context.Background(), filter)
	if err != nil {
		return nil, err
	}
	byBucket := make(map[int64]int, len(buckets))
	for i := range buckets {
		byBucket[buckets[i].Bucket] = i
	}
	for _, adjustment := range adjustments {
		originalIncluded := filter.Group == "" || filter.Group == adjustment.Original.Group
		effectiveIncluded := filter.Group == "" || filter.Group == adjustment.Group
		old, current := int64(0), int64(0)
		if originalIncluded {
			old = int64(adjustment.Original.Quota)
			if adjustment.Original.Type == LogTypeRefund {
				old = -old
			}
		}
		if effectiveIncluded {
			current = adjustment.Quota
		}
		at := (adjustment.Original.CreatedAt+offsetSeconds)/bucketSeconds*bucketSeconds - offsetSeconds
		index, found := byBucket[at]
		if !found {
			index = len(buckets)
			byBucket[at] = index
			buckets = append(buckets, CostAccountingBucket{Bucket: at})
		}
		buckets[index].RevenueQuota, err = billingProjectionQuotaTotal(buckets[index].RevenueQuota, current-old)
		if err != nil {
			return nil, err
		}
		if originalIncluded != effectiveIncluded {
			if effectiveIncluded {
				buckets[index].Records++
			} else {
				buckets[index].Records--
			}
		}
	}
	sort.Slice(buckets, func(i, j int) bool { return buckets[i].Bucket < buckets[j].Bucket })
	return buckets, nil
}

func billingProjectionQuotaTotal(total, delta int64) (int64, error) {
	// This is an aggregate, potentially across many wallets. Preserve existing
	// aggregate ranges while rejecting arithmetic overflow instead of wrapping
	// an additional charge into a credit.
	if (delta > 0 && total > math.MaxInt64-delta) || (delta < 0 && total < math.MinInt64-delta) {
		return 0, ErrBillingEvidenceIntegrity
	}
	return total + delta, nil
}
