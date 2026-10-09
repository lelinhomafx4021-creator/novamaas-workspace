package model

import (
	"context"
	"sort"
)

// Corrections change revenue, never the upstream cost. Cost snapshots and the
// legacy unconfigured-channel fallback continue to use the original amount.
// A group is a customer billing discount group, not the upstream channel. When
// filtering by the effective group, its source's unchanged cost follows that
// request so profit agrees with its revenue. Snapshot group/channel metadata,
// supplier attribution and global upstream costs remain immutable.
func billingEffectiveCostChanges(filter CostAccountingFilter) ([]billingEffectiveLog, map[billingEffectiveLogKey]int64, error) {
	adjustments, err := billingEffectiveLogs(context.Background(), filter)
	if err != nil || len(adjustments) == 0 {
		return adjustments, nil, err
	}
	logs := make([]*Log, 0, len(adjustments))
	for i := range adjustments {
		logs = append(logs, &adjustments[i].Original)
	}
	if err := AttachLogAccounting(logs); err != nil {
		return nil, nil, err
	}
	unconfigured, err := unconfiguredCostChannelIDs(filter)
	if err != nil {
		return nil, nil, err
	}
	fallback := make(map[int]bool, len(unconfigured))
	for _, id := range unconfigured {
		fallback[id] = true
	}
	costs := make(map[billingEffectiveLogKey]int64, len(adjustments))
	for i := range adjustments {
		log := &adjustments[i].Original
		if log.CostQuota != nil {
			costs[billingLogIdentity(log)] = *log.CostQuota
		} else if fallback[log.ChannelId] {
			cost := int64(log.Quota)
			if log.Type == LogTypeRefund {
				cost = -cost
			}
			costs[billingLogIdentity(log)] = cost
		}
	}
	return adjustments, costs, nil
}

func projectBillingCostTotals(filter CostAccountingFilter, totals CostAccountingTotals) (CostAccountingTotals, error) {
	adjustments, costs, err := billingEffectiveCostChanges(filter)
	if err != nil || len(adjustments) == 0 {
		return totals, err
	}
	statistics, err := sumBillingEffectiveLogStatistics(filter)
	if err != nil {
		return totals, err
	}
	for _, adjustment := range adjustments {
		originalIncluded := filter.Group == "" || filter.Group == adjustment.Original.Group
		effectiveIncluded := filter.Group == "" || filter.Group == adjustment.Group
		if originalIncluded == effectiveIncluded {
			continue
		}
		cost, found := costs[billingLogIdentity(&adjustment.Original)]
		if !found {
			continue
		}
		direction := int64(-1)
		if effectiveIncluded {
			direction = 1
		}
		totals.CostQuota, err = billingProjectionQuotaTotal(totals.CostQuota, direction*cost)
		if err != nil {
			return totals, err
		}
		if adjustment.Original.CostQuota != nil {
			totals.LinkedRecords += direction
			totals.Records += direction
		} else {
			totals.DefaultedRecords += direction
			totals.DefaultedCostQuota, err = billingProjectionQuotaTotal(totals.DefaultedCostQuota, direction*cost)
			if err != nil {
				return totals, err
			}
		}
	}
	totals.ConsumptionQuota, totals.RefundQuota, totals.RevenueQuota = statistics.Quota, statistics.RefundQuota, statistics.RevenueQuota
	totals.ProfitQuota = totals.RevenueQuota - totals.CostQuota
	return totals, nil
}

func projectBillingCostBuckets(filter CostAccountingFilter, buckets []CostAccountingBucket, seconds, offset int64) ([]CostAccountingBucket, error) {
	adjustments, costs, err := billingEffectiveCostChanges(filter)
	if err != nil || len(adjustments) == 0 {
		return buckets, err
	}
	byBucket := make(map[int64]int, len(buckets))
	for i := range buckets {
		byBucket[buckets[i].Bucket] = i
	}
	for _, adjustment := range adjustments {
		originalIncluded := filter.Group == "" || filter.Group == adjustment.Original.Group
		effectiveIncluded := filter.Group == "" || filter.Group == adjustment.Group
		if originalIncluded == effectiveIncluded {
			continue
		}
		cost, found := costs[billingLogIdentity(&adjustment.Original)]
		if !found {
			continue
		}
		at := (adjustment.Original.CreatedAt+offset)/seconds*seconds - offset
		index, found := byBucket[at]
		if !found {
			index = len(buckets)
			byBucket[at] = index
			buckets = append(buckets, CostAccountingBucket{Bucket: at})
		}
		if effectiveIncluded {
			buckets[index].CostQuota, err = billingProjectionQuotaTotal(buckets[index].CostQuota, cost)
		} else {
			buckets[index].CostQuota, err = billingProjectionQuotaTotal(buckets[index].CostQuota, -cost)
		}
		if err != nil {
			return nil, err
		}
	}
	financial, err := sumBillingEffectiveLogBuckets(filter, seconds, offset)
	if err != nil {
		return nil, err
	}
	for i := range buckets {
		buckets[i].RevenueQuota, buckets[i].Records = 0, 0
	}
	for _, current := range financial {
		index, found := byBucket[current.Bucket]
		if !found {
			index = len(buckets)
			byBucket[current.Bucket] = index
			buckets = append(buckets, CostAccountingBucket{Bucket: current.Bucket})
		}
		buckets[index].RevenueQuota, buckets[index].Records = current.RevenueQuota, current.Records
	}
	for i := range buckets {
		buckets[i].ProfitQuota = buckets[i].RevenueQuota - buckets[i].CostQuota
	}
	sort.Slice(buckets, func(i, j int) bool { return buckets[i].Bucket < buckets[j].Bucket })
	return buckets, nil
}
