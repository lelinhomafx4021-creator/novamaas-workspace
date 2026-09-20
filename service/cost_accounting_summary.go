package service

import (
	"time"

	"github.com/QuantumNous/new-api/model"
	"golang.org/x/sync/errgroup"
)

type CostAccountingPeriodSummary struct {
	StartTimestamp     int64  `json:"start_timestamp"`
	EndTimestamp       int64  `json:"end_timestamp"`
	RevenueQuota       int64  `json:"revenue_quota"`
	CostQuota          int64  `json:"cost_quota"`
	ProfitQuota        *int64 `json:"profit_quota"`
	UsageRecords       int64  `json:"usage_records"`
	CostRecords        int64  `json:"cost_records"`
	MissingCostRecords int64  `json:"missing_cost_records"`
	AccountingComplete bool   `json:"accounting_complete"`
}

type CostAccountingOverview struct {
	Today     CostAccountingPeriodSummary `json:"today"`
	Yesterday CostAccountingPeriodSummary `json:"yesterday"`
}

func GetCostAccountingOverview(now time.Time, baseFilter model.CostAccountingFilter) (*CostAccountingOverview, error) {
	location := time.FixedZone("Asia/Shanghai", 8*3600)
	localNow := now.In(location)
	todayStart := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location).Unix()
	yesterdayStart := time.Unix(todayStart, 0).In(location).AddDate(0, 0, -1).Unix()

	filters := [2]model.CostAccountingFilter{baseFilter, baseFilter}
	filters[0].StartTimestamp = todayStart
	filters[0].EndTimestamp = localNow.Unix()
	filters[1].StartTimestamp = yesterdayStart
	filters[1].EndTimestamp = todayStart - 1

	var statistics [2]model.LogStatistics
	var totals [2]model.CostAccountingTotals
	var group errgroup.Group
	for index := range filters {
		index := index
		group.Go(func() error {
			value, err := model.SumLogStatistics(filters[index])
			statistics[index] = value
			return err
		})
		group.Go(func() error {
			value, err := model.SumCostAccounting(filters[index])
			totals[index] = value
			return err
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}

	periods := [2]CostAccountingPeriodSummary{}
	for index := range periods {
		totals[index] = model.ReconcileCostAccountingTotals(statistics[index], totals[index])
		periods[index] = CostAccountingPeriodSummary{
			StartTimestamp:     filters[index].StartTimestamp,
			EndTimestamp:       filters[index].EndTimestamp,
			RevenueQuota:       totals[index].RevenueQuota,
			CostQuota:          totals[index].CostQuota,
			UsageRecords:       totals[index].UsageRecords,
			CostRecords:        totals[index].Records + totals[index].DefaultedRecords,
			MissingCostRecords: totals[index].MissingCostRecords,
			AccountingComplete: totals[index].AccountingComplete,
		}
		if totals[index].AccountingComplete {
			profitQuota := totals[index].ProfitQuota
			periods[index].ProfitQuota = &profitQuota
		}
	}

	return &CostAccountingOverview{Today: periods[0], Yesterday: periods[1]}, nil
}
