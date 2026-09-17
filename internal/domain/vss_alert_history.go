package domain

import (
	"time"

	"gorm.io/gorm"
)

type VSSAlertHistory struct {
	ID         	uint           	`gorm:"primaryKey" json:"id"`
	DetectedAt 	time.Time      	`gorm:"index;not null" json:"detected_at"`
	DeviceID   	string         	`gorm:"size:64;index;not null" json:"device_id"`
	DeviceName 	string         	`gorm:"size:128;index" json:"device_name"`
	NodeID     	string         	`gorm:"size:32" json:"node_id"`
	DTU        	string         	`gorm:"size:32" json:"dtu"`
	ReportTime 	int64          	`json:"report_time"`
	IsLater    	bool           	`json:"is_later"`
	DelaySec   	int64          	`json:"delay_sec"`
	Reason     	string         	`gorm:"size:64;index;not null" json:"reason"`
	Message    	string         	`gorm:"type:text" json:"message"`
	EmailSent  	bool           	`gorm:"default:false" json:"email_sent"`
	YearMonth  	string         	`gorm:"size:7;index;not null" json:"year_month"`
	CreatedAt  	time.Time      	`json:"created_at"`
	UpdatedAt  	time.Time      	`json:"updated_at"`
	DeletedAt  	gorm.DeletedAt 	`gorm:"index" json:"-"`
	Action     	string 			`gorm:"size:16;index" json:"action"`
	AlarmID    	string 			`gorm:"size:64;index" json:"alarm_id"`
	AlarmDetail	string 			`gorm:"type:text" json:"alarm_detail"`
	EventType   string 			`gorm:"size:32;index" json:"event_type"`
}

func (VSSAlertHistory) TableName() string {
	return "vss_alert_histories"
}