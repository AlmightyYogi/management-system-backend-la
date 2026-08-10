package repository

import (
	"strings"

	"github.com/AlmightyOggy/management-system/internal/domain"
	"gorm.io/gorm"
)

type ReportRepository interface {
	BaseRepository[domain.Report]
	FindByUUID(uuid string) (*domain.Report, error)
	FindWithFilters(filter ReportFilter) ([]domain.Report, int64, error)
	FindLastByPrefix(prefix string) (*domain.Report, error)
	CountWithFilters(filter ReportFilter) (int64, error)
	FindAllForExport(filter ReportFilter) ([]domain.Report, error)
}

type reportRepository struct {
	BaseRepository[domain.Report]
	db *gorm.DB
}

func NewReportRepository(db *gorm.DB) ReportRepository {
	return &reportRepository{
		BaseRepository: NewBaseRepository[domain.Report](db),
		db:             db,
	}
}

type ReportFilter struct {
	Search    string
	StartDate string
	EndDate   string
	Type      string
	Status    *int
	Page      int
	PerPage   int
}

func (r *reportRepository) FindByUUID(uuid string) (*domain.Report, error) {
	var report domain.Report
	err := r.db.Where("uuid = ?", uuid).First(&report).Error
	if err != nil {
		return nil, err
	}
	return &report, nil
}

func (r *reportRepository) FindLastByPrefix(prefix string) (*domain.Report, error) {
	var report domain.Report
	err := r.db.Where("incident LIKE ?", prefix+"%").
		Order("id DESC").
		First(&report).Error
	if err != nil {
		return nil, err
	}
	return &report, nil
}

func (r *reportRepository) buildQuery(filter ReportFilter) *gorm.DB {
	query := r.db.Model(&domain.Report{})

	if filter.Search != "" {
		searchLower := "%" + strings.ToLower(filter.Search) + "%"
		query = query.Where(
			"LOWER(incident) LIKE ? OR LOWER(requestor) LIKE ? OR LOWER(requestor_email) LIKE ? OR LOWER(apps) LIKE ? OR LOWER(assigned_to) LIKE ? OR LOWER(scope) LIKE ? OR LOWER(severity) LIKE ?",
			searchLower, searchLower, searchLower, searchLower, searchLower, searchLower, searchLower,
		)
	}

	if filter.StartDate != "" {
		query = query.Where("request_date >= ?", filter.StartDate)
	}
	if filter.EndDate != "" {
		query = query.Where("request_date <= ?", filter.EndDate)
	}
	if filter.Type != "" {
		query = query.Where("type = ?", filter.Type)
	}
	if filter.Status != nil {
		query = query.Where("status = ?", *filter.Status)
	}

	return query
}

func (r *reportRepository) CountWithFilters(filter ReportFilter) (int64, error) {
	var total int64
	err := r.buildQuery(filter).Count(&total).Error
	return total, err
}

func (r *reportRepository) FindWithFilters(filter ReportFilter) ([]domain.Report, int64, error) {
	var reports []domain.Report
	var total int64

	query := r.buildQuery(filter)
	query.Count(&total)

	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PerPage < 1 {
		filter.PerPage = 15
	}

	offset := (filter.Page - 1) * filter.PerPage

	err := query.Order("created_at DESC").
		Offset(offset).
		Limit(filter.PerPage).
		Find(&reports).Error

	if err != nil {
		return nil, 0, err
	}

	return reports, total, nil
}

func (r *reportRepository) FindAllForExport(filter ReportFilter) ([]domain.Report, error) {
	var reports []domain.Report
	err := r.buildQuery(filter).
		Order("created_at DESC").
		Find(&reports).Error
	if err != nil {
		return nil, err
	}
	return reports, nil
}