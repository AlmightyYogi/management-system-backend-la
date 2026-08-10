package dto

import "time"

type CreateReportExternalTeamRequest struct {
	ReportID             string     `json:"report_id" form:"report_id" validate:"required,uuid"`
	ExternalTeamID       uint       `json:"external_team_id" form:"external_team_id" validate:"required"`
	PIC                  string     `json:"pic" form:"pic" validate:"omitempty,max=255"`
	StartTime            time.Time  `json:"start_time" form:"start_time" validate:"required" time_format:"2006-01-02T15:04"`
	EndTime              *time.Time `json:"end_time" form:"end_time" validate:"omitempty,gtfield=StartTime" time_format:"2006-01-02T15:04"`
	EvidenceFileExternal []string   `json:"evidence_file_external" form:"-"`
}

type UpdateReportExternalTeamRequest struct {
	PIC                  string     `json:"pic" form:"pic" validate:"omitempty,max=255"`
	StartTime            *time.Time `json:"start_time" form:"start_time" validate:"omitempty" time_format:"2006-01-02T15:04"`
	EndTime              *time.Time `json:"end_time" form:"end_time" validate:"omitempty" time_format:"2006-01-02T15:04"`
	EvidenceFileExternal []string   `json:"evidence_file_external" form:"-"`
}