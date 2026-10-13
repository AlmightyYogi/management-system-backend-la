package repository

import (
	"strings"
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
	SortBy     string
	SortDir    string
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

	orderCol := mapHistorySort(filter.SortBy)
	orderDir := "DESC"
	if strings.EqualFold(filter.SortDir, "asc") {
		orderDir = "ASC"
	}

	var rows []domain.VSSAlertHistory
	err := q.Order(orderCol + " " + orderDir).
		Offset(offset).
		Limit(filter.PerPage).
		Find(&rows).Error
	return rows, total, err
}

func mapHistorySort(sortBy string) string {
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "device_name", "name":
		return "device_name"
	case "device_id":
		return "device_id"
	case "delay_sec", "delay":
		return "delay_sec"
	case "dtu", "device_time":
		return "dtu"
	case "reason":
		return "reason"
	case "action":
		return "action"
	case "report_time", "sent_to_middleware":
		return "report_time"
	case "detected_at", "last_seen", "last_report":
		return "detected_at"
	default:
		return "detected_at"
	}
}

func (r *vssAlertHistoryRepository) CountByMonth(yearMonth string) (int64, error) {
	var total int64
	err := r.db.Model(&domain.VSSAlertHistory{}).
		Where("year_month = ?", yearMonth).
		Count(&total).Error
	return total, err
}