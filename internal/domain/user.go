package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	UUID      	uuid.UUID 	   `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	Name        string         `gorm:"size:100" json:"name"`
	Email       string         `gorm:"size:100;uniqueIndex" json:"email"`
	Password    string         `gorm:"size:255" json:"-"`
	RoleID      uint           `json:"role_id"`
	Phone       string         `gorm:"size:20" json:"phone"`
	Image       string         `gorm:"size:255" json:"image"`
	LastLoginAt *time.Time     `json:"last_login_at"`
	Active      bool           `gorm:"default:true" json:"active"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (User) TableName() string {
	return "users"
}