package model

import (
	"encoding/json"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type billingHistoricalRate struct {
	Group  string
	Ratio  string
	TaskID string
}

// Resolve only immutable usage evidence, never today's token or pricing config.
// A missing or ambiguous source keeps its ledger amount and an unknown rate.
func billingEntryHistoricalRates(scope *gorm.DB, userID int, entries []BillingEntry) (map[int64]billingHistoricalRate, error) {
	result := make(map[int64]billingHistoricalRate, len(entries))
	var sourceIDs []int
	var requestIDs, taskIDs []string
	for _, entry := range entries {
		if entry.Kind == "rate_correction" {
			result[entry.Sequence] = billingHistoricalRate{Group: entry.BillingGroup, Ratio: entry.BillingRate}
			continue
		}
		if entry.Kind == "task_adjustment" {
			if entry.RequestID != "" {
				taskIDs = append(taskIDs, entry.RequestID)
			}
			continue
		}
		if entry.SourceLogID > 0 && !common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
			sourceIDs = append(sourceIDs, entry.SourceLogID)
		} else if entry.RequestID != "" {
			requestIDs = append(requestIDs, entry.RequestID)
		}
	}
	logsDB := LOG_DB
	if LOG_DB == DB {
		logsDB = scope.Session(&gorm.Session{NewDB: true})
	}
	byID := make(map[int][]Log)
	byRequest := make(map[string][]Log)
	for _, query := range []struct {
		column string
		values any
		count  int
	}{{"id", sourceIDs, len(sourceIDs)}, {"request_id", requestIDs, len(requestIDs)}} {
		if query.count == 0 {
			continue
		}
		var logs []Log
		if err := logsDB.Model(&Log{}).Select([]string{"id", "user_id", "type", "model_name", "quota", "group", "request_id", "other"}).
			Where("user_id = ? AND type IN ?", userID, []int{LogTypeConsume, LogTypeRefund}).Where(query.column+" IN ?", query.values).Find(&logs).Error; err != nil {
			return nil, err
		}
		for _, log := range logs {
			if query.column == "id" {
				byID[log.Id] = append(byID[log.Id], log)
			} else {
				byRequest[log.RequestId] = append(byRequest[log.RequestId], log)
			}
		}
	}
	byTask := make(map[string][]Task)
	if len(taskIDs) > 0 {
		var tasks []Task
		if err := scope.Session(&gorm.Session{NewDB: true}).Model(&Task{}).
			Select([]string{"task_id", "group", "private_data"}).Where("user_id = ? AND task_id IN ?", userID, taskIDs).Find(&tasks).Error; err != nil {
			return nil, err
		}
		for _, task := range tasks {
			byTask[task.TaskID] = append(byTask[task.TaskID], task)
		}
	}
	for _, entry := range entries {
		if entry.Kind == "rate_correction" {
			continue
		}
		if entry.Kind == "task_adjustment" {
			tasks := byTask[entry.RequestID]
			if len(tasks) == 1 && tasks[0].PrivateData.BillingContext != nil {
				ratio := tasks[0].PrivateData.BillingContext.GroupRatio
				if ratio >= 0 && !math.IsNaN(ratio) && !math.IsInf(ratio, 0) {
					result[entry.Sequence] = billingHistoricalRate{Group: tasks[0].Group, Ratio: decimal.NewFromFloat(ratio).String(), TaskID: entry.RequestID}
				}
			}
			continue
		}
		candidates := byRequest[entry.RequestID]
		if entry.SourceLogID > 0 && !common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
			candidates = byID[entry.SourceLogID]
		}
		quota, logType := entry.Quota, LogTypeConsume
		if quota < 0 {
			quota, logType = -quota, LogTypeRefund
		}
		var matched *Log
		ambiguous := false
		for i := range candidates {
			log := &candidates[i]
			if log.ModelName != entry.ModelName || int64(log.Quota) != quota || log.Type != logType {
				continue
			}
			if matched != nil {
				ambiguous = true
				break
			}
			matched = log
		}
		if matched == nil || ambiguous {
			continue
		}
		rate := billingHistoricalRate{Group: matched.Group}
		var metadata struct {
			GroupRatio json.RawMessage `json:"group_ratio"`
			TaskID     string          `json:"task_id"`
		}
		if common.UnmarshalJsonStr(matched.Other, &metadata) == nil {
			rate.TaskID = metadata.TaskID
			value := strings.Trim(string(metadata.GroupRatio), "\"")
			if ratio, err := decimal.NewFromString(value); err == nil && !ratio.IsNegative() {
				rate.Ratio = ratio.String()
			}
		}
		result[entry.Sequence] = rate
	}
	return result, nil
}
