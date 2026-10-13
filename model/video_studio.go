package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

// VideoProject is a user's video studio workspace. Draft and Canvas are
// opaque client JSON stored as TEXT so every supported database can hold it.
type VideoProject struct {
	ID          int64  `json:"-" gorm:"primaryKey"`
	PublicID    string `json:"id" gorm:"type:varchar(64);uniqueIndex"`
	OwnerUserID int    `json:"-" gorm:"index:idx_video_projects_owner,priority:1"`
	Name        string `json:"name" gorm:"type:varchar(128)"`
	Draft       string `json:"draft" gorm:"type:text"`
	Canvas      string `json:"canvas" gorm:"type:text"`
	Revision    int64  `json:"revision"`
	CreatedAt   int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt   int64  `json:"updated_at" gorm:"bigint;index:idx_video_projects_owner,priority:2"`
	DeletedAt   int64  `json:"-" gorm:"bigint;index"`
}

// VideoCreationRun links one explicit "generate" action to its Task. Task
// remains the source of truth for status and charges.
type VideoCreationRun struct {
	ID              int64  `json:"-" gorm:"primaryKey"`
	PublicID        string `json:"id" gorm:"type:varchar(64);uniqueIndex"`
	ProjectID       int64  `json:"-" gorm:"index:idx_video_runs_project,priority:1"`
	NodeID          string `json:"node_id" gorm:"type:varchar(64)"`
	OwnerUserID     int    `json:"-" gorm:"uniqueIndex:idx_video_runs_client_request,priority:1"`
	ClientRequestID string `json:"client_request_id" gorm:"type:varchar(64);uniqueIndex:idx_video_runs_client_request,priority:2"`
	RequestHash     string `json:"-" gorm:"type:char(64)"`
	Snapshot        string `json:"snapshot" gorm:"type:text"`
	TaskID          string `json:"task_id" gorm:"type:varchar(191);index"`
	ErrorMessage    string `json:"error_message,omitempty" gorm:"type:text"`
	CreatedAt       int64  `json:"created_at" gorm:"bigint;index:idx_video_runs_project,priority:2"`
}

var ErrVideoProjectRevisionConflict = errors.New("video project was modified elsewhere")

func FindVideoProject(ownerUserID int, publicID string) (*VideoProject, error) {
	var project VideoProject
	err := DB.Where("owner_user_id = ? AND public_id = ? AND deleted_at = 0", ownerUserID, publicID).First(&project).Error
	if err != nil {
		return nil, err
	}
	return &project, nil
}

func ListVideoProjects(ownerUserID int, offset int, limit int) ([]VideoProject, int64, error) {
	query := DB.Model(&VideoProject{}).Where("owner_user_id = ? AND deleted_at = 0", ownerUserID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var projects []VideoProject
	err := query.Select("id", "public_id", "name", "revision", "created_at", "updated_at").
		Order("updated_at desc").Offset(offset).Limit(limit).Find(&projects).Error
	return projects, total, err
}

// UpdateVideoProject applies an optimistic-concurrency save: it only succeeds
// when the stored revision still equals expectedRevision.
func UpdateVideoProject(ownerUserID int, publicID string, expectedRevision int64, updates map[string]any) (*VideoProject, error) {
	updates["revision"] = gorm.Expr("revision + 1")
	updates["updated_at"] = common.GetTimestamp()
	result := DB.Model(&VideoProject{}).
		Where("owner_user_id = ? AND public_id = ? AND deleted_at = 0 AND revision = ?", ownerUserID, publicID, expectedRevision).
		Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		if _, err := FindVideoProject(ownerUserID, publicID); err != nil {
			return nil, err
		}
		return nil, ErrVideoProjectRevisionConflict
	}
	return FindVideoProject(ownerUserID, publicID)
}

func DeleteVideoProject(ownerUserID int, publicID string) error {
	result := DB.Model(&VideoProject{}).
		Where("owner_user_id = ? AND public_id = ? AND deleted_at = 0", ownerUserID, publicID).
		Update("deleted_at", common.GetTimestamp())
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func FindVideoCreationRunByClientRequest(ownerUserID int, clientRequestID string) (*VideoCreationRun, error) {
	var run VideoCreationRun
	err := DB.Where("owner_user_id = ? AND client_request_id = ?", ownerUserID, clientRequestID).First(&run).Error
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func ListVideoCreationRuns(ownerUserID int, projectID int64, offset int, limit int) ([]VideoCreationRun, int64, error) {
	query := DB.Model(&VideoCreationRun{}).Where("owner_user_id = ? AND project_id = ?", ownerUserID, projectID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var runs []VideoCreationRun
	err := query.Order("id desc").Offset(offset).Limit(limit).Find(&runs).Error
	return runs, total, err
}
