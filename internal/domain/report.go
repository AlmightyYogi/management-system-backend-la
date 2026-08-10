package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Report struct {
	ID                    uint           `gorm:"primaryKey" json:"id"`
	UUID                  uuid.UUID      `gorm:"type:char(36);uniqueIndex" json:"uuid"`
	Incident              string         `gorm:"size:50;uniqueIndex" json:"incident"`
	Requestor             string         `gorm:"size:255;not null" json:"requestor"`
	RequestorEmail        string         `gorm:"size:255;not null" json:"requestor_email"`
	RequestDate           time.Time      `gorm:"type:date;not null" json:"request_date"`
	ReportTime            string         `gorm:"size:10;not null" json:"report_time"`

	Apps                  string         `gorm:"size:100;not null" json:"apps"`
	Type                  string         `gorm:"size:50;not null" json:"type"`
	Severity              string         `gorm:"size:100" json:"severity"`
	AssignedTo            string         `gorm:"size:255" json:"assigned_to"`
	Scope                 string         `gorm:"size:255" json:"scope"`
	Description           string         `gorm:"type:text" json:"description"`
	Resolution            string         `gorm:"type:text" json:"resolution"`
	RCA                   string         `gorm:"type:text" json:"rca"`

	Status                int            `gorm:"default:1" json:"status"`
	HandledBy             int            `gorm:"default:0" json:"handled_by"`

	ResponseTime          *time.Time     `json:"response_time"`
	ServicerestoredTime   *time.Time     `json:"servicerestored_time"`
	RestoredTime          *int64         `json:"restored_time"`
	TotalInternalDuration *int64         `json:"total_internal_duration"`
	ResolvedTime          *time.Time     `json:"resolved_time"`
	ClosedAt              *time.Time     `json:"closed_at"`

	FileDowntimeEvidence  StringArray    `gorm:"type:json" json:"file_downtime_evidence"`
	RestorationEvidence   StringArray    `gorm:"type:json" json:"restoration_evidence"`

	CreatedAt             time.Time      `json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`
	DeletedAt             gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Report) TableName() string {
	return "reports"
}