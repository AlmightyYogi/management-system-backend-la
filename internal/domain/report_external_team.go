package domain

import (
	"time"
)

type ReportExternalTeam struct {
	ID						uint		`gorm:"primaryKey" json:"id"`
	ReportID				uint    	`gorm:"index" json:"report_id"`
	ExternalTeams			uint    	`gorm:"index" json:"external_team_id"`
	PIC						string		`gorm:"size:255" json:"pic"`
	StartTime				*time.Time	`json:"start_time"`
	EndTime					*time.Time	`json:"end_time"`
	Duration				*int64      `json:"duration"`
	TotalExternalDuration	*int64      `json:"total_external_duration"`
	EvidenceFileExternal	StringArray `gorm:"type:json" json:"evidence_file_external"`
	CreatedAt             	time.Time   `json:"created_at"`
	UpdatedAt             	time.Time   `json:"updated_at"`
}

func (ReportExternalTeam) TableName() string {
	return "report_external_teams"
}