package repository

import (
	"time"

	"github.com/AlmightyOggy/management-system/internal/domain"
	"gorm.io/gorm"
)

type VSSHistoryFilter struct {
	DeviceID   string
	DeviceName string
	Reason     string
	YearMonth  string
	StartDate  string
	EndDate    string
	Page       int
	PerPage    int
}

type VSSAlertHistoryRepository interface {
	Create(h *domain.VSSAlertHistory) error
	List(filter VSSHistoryFilter) ([]domain.VSSAlertHistory, int64, error)
	CountByMonth(yearMonth string) (int64, error)
	ExistsRecent(deviceID, reason string, within time.Duration) (bool, error)
}

type vssAlertHistoryRepository struct {
	db *gorm.DB
}

func NewVSSAlertHistoryRepository(db *gorm.DB) VSSAlertHistoryRepository {
	return &vssAlertHistoryRepository{db: db}
}

func (r *vssAlertHistoryRepository) Create(h *domain.VSSAlertHistory) error {
	return r.db.Create(h).Error
}

func (r *vssAlertHistoryRepository) ExistsRecent(deviceID, reason string, within time.Duration) (bool, error) {
	var count int64
	since := time.Now().Add(-within)
	err := r.db.Model(&domain.VSSAlertHistory{}).
		Where("device_id = ? AND reason = ? AND detected_at >= ?", deviceID, reason, since).
		Count(&count).Error
	return count > 0, err
}

func (r *vssAlertHistoryRepository) List(filter VSSHistoryFilter) ([]domain.VSSAlertHistory, int64, error) {
	q := r.db.Model(&domain.VSSAlertHistory{})

	if filter.DeviceID != "" {
		q = q.Where("device_id = ?", filter.DeviceID)
	}
	if filter.DeviceName != "" {
		q = q.Where("device_name ILIKE ?", "%"+filter.DeviceName+"%")
	}
	if filter.Reason != "" {
		q = q.Where("reason = ?", filter.Reason)
	}
	if filter.YearMonth != "" {
		q = q.Where("year_month = ?", filter.YearMonth)
	}
	if filter.StartDate != "" {
		q = q.Where("detected_at >= ?", filter.StartDate+" 00:00:00")
	}
	if filter.EndDate != "" {
		q = q.Where("detected_at <= ?", filter.EndDate+" 23:59:59")
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PerPage < 1 {
		filter.PerPage = 20
	}
	offset := (filter.Page - 1) * filter.PerPage

	var rows []domain.VSSAlertHistory
	err := q.Order("detected_at DESC").
		Offset(offset).
		Limit(filter.PerPage).
		Find(&rows).Error
	return rows, total, err
}

func (r *vssAlertHistoryRepository) CountByMonth(yearMonth string) (int64, error) {
	var total int64
	err := r.db.Model(&domain.VSSAlertHistory{}).
		Where("year_month = ?", yearMonth).
		Count(&total).Error
	return total, err
}