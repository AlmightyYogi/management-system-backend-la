package domain

type VSSDelayEvent struct {
	DetectedAt string `json:"detected_at"`
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
	NodeID     string `json:"node_id,omitempty"`
	DTU        string `json:"dtu"`
	ReportTime int64  `json:"report_time"`
	IsLater    bool   `json:"is_later"`
	DelaySec   int64  `json:"delay_sec"`
	Reason     string `json:"reason"`
	Message    string `json:"message"`
	Action      string `json:"action,omitempty"`
	AlarmID     string `json:"alarm_id,omitempty"`
	AlarmDetail string `json:"alarm_detail,omitempty"`
	EventType   string `json:"event_type,omitempty"`
}

type VSSDelayListResult struct {
	Data    []VSSDelayEvent `json:"data"`
	Total   int             `json:"total"`
	Page    int             `json:"page"`
	PerPage int             `json:"per_page"`
}