package repository

import (
	"github.com/AlmightyOggy/management-system/internal/domain"
	"gorm.io/gorm"
)

type ReportExternalTeamRepository interface {
	BaseRepository[domain.ReportExternalTeam]
	FindByID(id uint) (*domain.ReportExternalTeam, error)
	FindByReportID(reportID uint) ([]domain.ReportExternalTeam, error)
	SumDurationByReportID(reportID uint) (int64, error)
	UpdateTotalExternalDuration(reportID uint, total int64) error
}

type reportExternalTeamRepository struct {
	BaseRepository[domain.ReportExternalTeam]
	db *gorm.DB
}

func NewReportExternalTeamRepository(db *gorm.DB) ReportExternalTeamRepository {
	return &reportExternalTeamRepository{
		BaseRepository: NewBaseRepository[domain.ReportExternalTeam](db),
		db:             db,
	}
}

func (r *reportExternalTeamRepository) FindByID(id uint) (*domain.ReportExternalTeam, error) {
	var ext domain.ReportExternalTeam
	err := r.db.Where("id = ?", id).First(&ext).Error
	if err != nil {
		return nil, err
	}
	return &ext, nil
}

func (r *reportExternalTeamRepository) FindByReportID(reportID uint) ([]domain.ReportExternalTeam, error) {
	var exts []domain.ReportExternalTeam
	err := r.db.Where("report_id = ?", reportID).
		Order("start_time ASC").
		Find(&exts).Error
	if err != nil {
		return nil, err
	}
	return exts, nil
}

func (r *reportExternalTeamRepository) SumDurationByReportID(reportID uint) (int64, error) {
	var total int64
	err := r.db.Model(&domain.ReportExternalTeam{}).
		Where("report_id = ?", reportID).
		Select("COALESCE(SUM(duration), 0)").
		Scan(&total).Error
	return total, err
}

func (r *reportExternalTeamRepository) UpdateTotalExternalDuration(reportID uint, total int64) error {
	return r.db.Model(&domain.ReportExternalTeam{}).
		Where("report_id = ?", reportID).
		Update("total_external_duration", total).Error
}