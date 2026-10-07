package model

import (
	"encoding/json"
	"errors"

	"gorm.io/gorm"
)

type taskWebhookData struct {
	TaskID            string          `json:"task_id"`
	Platform          string          `json:"platform"`
	Status            TaskStatus      `json:"status"`
	StateVersion      int64           `json:"state_version"`
	Progress          string          `json:"progress"`
	FailReason        string          `json:"fail_reason,omitempty"`
	Response          json.RawMessage `json:"response"`
	ResponseOmitted   bool            `json:"response_omitted,omitempty"`
	ResponseTruncated bool            `json:"response_truncated,omitempty"`
	ResponseSHA256    string          `json:"response_sha256,omitempty"`
}

func (task *Task) queueStatusWebhook(tx *gorm.DB) error {
	if task.UserId <= 0 {
		return nil
	}
	if task.WebhookVersion <= 0 {
		return errors.New("task webhook state version must be positive")
	}
	response, hash, omitted, truncated := taskPollResponse(task.Data)
	_, err := QueueWebhookEvent(tx, task.UserId, WebhookEventTaskStatusChanged, 0, taskWebhookData{
		TaskID: task.TaskID, Platform: string(task.Platform), Status: task.Status, StateVersion: task.WebhookVersion,
		Progress: task.Progress, FailReason: task.FailReason, Response: response,
		ResponseOmitted: omitted, ResponseTruncated: truncated, ResponseSHA256: hash,
	})
	return err
}
