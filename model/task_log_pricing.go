package model

import (
	"math"

	"github.com/QuantumNous/new-api/common"
)

// Older submission logs saved rounded multipliers only in their Content. The
// task's original billing snapshot can supply exact display evidence without
// rewriting a log or consulting today's prices. Effective corrections run later.
func attachHistoricalTaskLogPricing(logs []*Log) error {
	type taskIdentity struct {
		userID int
		taskID string
	}
	metadataByLog := make(map[*Log]map[string]any)
	identities := make(map[*Log]taskIdentity)
	taskIDs, userIDs := make(map[string]bool), make(map[int]bool)
	for _, log := range logs {
		if log == nil || log.Type != LogTypeConsume || log.UserId <= 0 {
			continue
		}
		var metadata map[string]any
		if common.UnmarshalJsonStr(log.Other, &metadata) != nil || metadata == nil {
			continue
		}
		// An explicit multiplier map, including {}, is authoritative. Preserve
		// older exact top-level multipliers too; never replace them from a task.
		if _, present := metadata["other_ratios"]; present {
			continue
		}
		if _, present := metadata["video_input"]; present {
			continue
		}
		if value, present := metadata["is_task"]; present && value != true {
			continue
		}
		taskID, ok := metadata["task_id"].(string)
		if !ok || taskID == "" || len(taskID) > 191 {
			continue
		}
		metadataByLog[log] = metadata
		identities[log] = taskIdentity{userID: log.UserId, taskID: taskID}
		taskIDs[taskID], userIDs[log.UserId] = true, true
	}
	if len(identities) == 0 {
		return nil
	}
	taskValues, userValues := make([]string, 0, len(taskIDs)), make([]int, 0, len(userIDs))
	for id := range taskIDs {
		taskValues = append(taskValues, id)
	}
	for id := range userIDs {
		userValues = append(userValues, id)
	}
	var tasks []Task
	if err := DB.Select("task_id", "user_id", "channel_id", "group", "private_data").Where("user_id IN ? AND task_id IN ?", userValues, taskValues).Find(&tasks).Error; err != nil {
		return err
	}
	byIdentity := make(map[taskIdentity][]Task)
	for _, task := range tasks {
		identity := taskIdentity{userID: task.UserId, taskID: task.TaskID}
		byIdentity[identity] = append(byIdentity[identity], task)
	}
	for log, identity := range identities {
		matches := byIdentity[identity]
		if len(matches) != 1 {
			continue
		}
		task := &matches[0]
		bc := task.PrivateData.BillingContext
		if bc == nil || bc.OriginModelName != log.ModelName || task.ChannelId != log.ChannelId || task.PrivateData.TokenId != log.TokenId || task.Group != log.Group {
			continue
		}
		metadata := metadataByLog[log]
		groupRatio, ok := metadata["group_ratio"].(float64)
		if !ok || math.IsNaN(groupRatio) || math.IsInf(groupRatio, 0) || groupRatio < 0 || groupRatio > 10 || groupRatio != bc.GroupRatio {
			continue
		}
		if value, present := metadata["model_ratio"]; present {
			modelRatio, ok := value.(float64)
			if !ok || modelRatio != bc.ModelRatio {
				continue
			}
		}
		videoRatio, found := bc.OtherRatios["video_input"]
		if !found || videoRatio <= 0 || math.IsNaN(videoRatio) || math.IsInf(videoRatio, 0) {
			continue
		}
		metadata["is_task"] = true
		metadata["other_ratios"] = map[string]float64{"video_input": videoRatio}
		body, err := common.Marshal(metadata)
		if err != nil {
			return err
		}
		log.Other = string(body)
	}
	return nil
}
