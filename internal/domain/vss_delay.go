package domain

type VSSDelayEvent struct {
	DetectedAt  string `json:"detected_at"`
	DeviceID    string `json:"device_id"`
	DeviceName  string `json:"device_name"`
	NodeID      string `json:"node_id,omitempty"`
	DTU         string `json:"dtu"`
	ReportTime  int64  `json:"report_time"`
	IsLater     bool   `json:"is_later"`
	DelayMs     int64  `json:"delay_ms"`
	DelaySec    int64  `json:"delay_sec"`
	Reason      string `json:"reason"`
	Message     string `json:"message"`
	Action      string `json:"action,omitempty"`
	AlarmID     string `json:"alarm_id,omitempty"`
	AlarmDetail string `json:"alarm_detail,omitempty"`
	EventType   string `json:"event_type,omitempty"`
}

type VSSLiveDevice struct {
	DeviceID    string  `json:"device_id"`
	DeviceName  string  `json:"device_name"`
	NodeID      string  `json:"node_id"`
	DTU         string  `json:"dtu"`
	LastSeen    string  `json:"last_seen"`
	DelayMs     int64   `json:"delay_ms"`
	DelaySec    int64   `json:"delay_sec"`
	Action      string  `json:"action"`
	Reason      string  `json:"reason"`
	Status      string  `json:"status"`
	IsLater     bool    `json:"is_later"`
	ReportTime  int64   `json:"report_time"`
}

type VSSLiveResult struct {
	Data        []VSSLiveDevice `json:"data"`
	Total       int             `json:"total"`
	Page        int             `json:"page"`
	PerPage     int             `json:"per_page"`
	TotalWS     int             `json:"total_ws"`
	Count80003  int             `json:"count_80003"`
	Count80004  int             `json:"count_80004"`
	Normal      int             `json:"normal"`
	Issue       int             `json:"issue"`
}

type VSSDelayListResult struct {
	Data    []VSSDelayEvent `json:"data"`
	Total   int             `json:"total"`
	Page    int             `json:"page"`
	PerPage int             `json:"per_page"`
}

type VSSDaySummaryDevice struct {
	DeviceID	string	`json:"device_id"`
	DeviceName	string	`json:"device_name"`
	MaxDelaySec	int64	`json:"max_delay_sec"`
	EventCount	int		`json:"event_count"`
}

type VSSDaySummary struct {
	Date			string				`json:"date"`
	TotalDevices	int					`json:"total_devices"`
	DelayOver5Count	int					`json:"delay_over_5_count"`
	TotalEvents		int					`json:"total_events"`
	MaxDelaySec		int64				`json:"max_delay_sec"`
	MaxDelayDevice	string				`json:"max_delay_device"`
	MaxDelayDeviceID string				`json:"max_delay_device_id"`
	Distribution	map[string]int		`json:"distribution"`
	TopDevices		[]VSSDaySummaryDevice `json:"top_devices"`
}

type VSSDayDeviceDelayStat struct {
	DeviceID   string  `json:"device_id"`
	DeviceName string  `json:"device_name"`
	MaxDelaySec int64  `json:"max_delay_sec"`
	MinDelaySec int64  `json:"min_delay_sec"`
	AvgDelaySec float64 `json:"avg_delay_sec"`
	EventCount  int    `json:"event_count"`
}