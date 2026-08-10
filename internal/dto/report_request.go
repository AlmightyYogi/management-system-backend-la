package dto

type CreateReportRequest struct {
	Requestor      string   `json:"requestor" form:"requestor" validate:"required,min=3,max=255"`
	RequestorEmail string   `json:"requestor_email" form:"requestor_email" validate:"required,email"`
	RequestDate    string   `json:"request_date" form:"request_date" validate:"required"`
	ReportTime     string   `json:"report_time" form:"report_time" validate:"required"`
	Apps           string   `json:"apps" form:"apps" validate:"required"`
	Type           string   `json:"type" form:"type" validate:"required,oneof=Incident Request Activity"`
	Description    string   `json:"description" form:"description" validate:"required"`
	Severity       string   `json:"severity" form:"severity" validate:"required"`
	AssignedTo     string   `json:"assigned_to" form:"assigned_to" validate:"required"`
	Scope          string   `json:"scope" form:"scope" validate:"required"`
	Status         int      `json:"status" form:"status" validate:"required,oneof=0 1"`

	FileDowntimeEvidence []string `form:"-"`
}

type UpdateReportRequest struct {
	Requestor           string     `json:"requestor" form:"requestor" validate:"omitempty,max=255"`
	RequestorEmail      string     `json:"requestor_email" form:"requestor_email" validate:"omitempty,email"`
	RequestDate         string     `json:"request_date" form:"request_date"`
	ReportTime          string     `json:"report_time" form:"report_time"`
	Apps                string     `json:"apps" form:"apps"`
	Type                string     `json:"type" form:"type" validate:"omitempty,oneof=Incident Request Activity"`
	Severity            string     `json:"severity" form:"severity"`
	AssignedTo          string     `json:"assigned_to" form:"assigned_to"`
	Scope               string     `json:"scope" form:"scope"`
	Description         string     `json:"description" form:"description"`
	Resolution          string     `json:"resolution" form:"resolution"`
	RCA                 string     `json:"rca" form:"rca"`
	ServicerestoredTime *string    `json:"servicerestored_time" form:"servicerestored_time"`
	CreatedAt           *string    `json:"created_at" form:"created_at"`
	Status              *int       `json:"status" form:"status"`
	HandledBy           *bool      `json:"handled_by" form:"handled_by"`
	FileDowntimeEvidence []string  `form:"-"`
	RestorationEvidence		[]string	`json:"restoration_evidence" form:"-"`
}

type MarkRestoredRequest struct {
}

type ToggleHandledRequest struct {
	HandledBy bool `json:"handled_by"`
}