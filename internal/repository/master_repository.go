package repository

import (
	"github.com/AlmightyOggy/management-system/internal/domain"
	"gorm.io/gorm"
)

type MasterRepository interface {
	GetSeverities() ([]domain.MstSeverity, error)
	GetApps() ([]domain.MstApp, error)
	GetAssignedTo() ([]domain.MstAssignedTo, error)
	GetScopes(reportType string) ([]domain.MstScope, error)
	GetExternalTeams() ([]domain.MstExternalTeam, error)
	GetImpacts() ([]domain.MstImpact, error)
	GetPriorities() ([]domain.MstPriority, error)
	GetRoles() ([]domain.MstRole, error)
}

type masterRepository struct {
	db *gorm.DB
}

func NewMasterRepository(db *gorm.DB) MasterRepository {
	return &masterRepository{db: db}
}

func (r *masterRepository) GetSeverities() ([]domain.MstSeverity, error) {
	var data []domain.MstSeverity
	err := r.db.Where("is_active = ?", true).Order("level ASC").Find(&data).Error
	return data, err
}

func (r *masterRepository) GetApps() ([]domain.MstApp, error) {
	var data []domain.MstApp
	err := r.db.Where("is_active = ?", true).Order("name ASC").Find(&data).Error
	return data, err
}

func (r *masterRepository) GetAssignedTo() ([]domain.MstAssignedTo, error) {
	var data []domain.MstAssignedTo
	err := r.db.Where("is_active = ?", true).Order("name ASC").Find(&data).Error
	return data, err
}

func (r *masterRepository) GetScopes(reportType string) ([]domain.MstScope, error) {
	var data []domain.MstScope
	query := r.db.Where("is_active = ?", true)
	if reportType != "" {
		query = query.Where("type = ? OR type = ?", reportType, "All")
	}
	err := query.Order("name ASC").Find(&data).Error
	return data, err
}

func (r *masterRepository) GetExternalTeams() ([]domain.MstExternalTeam, error) {
	var data []domain.MstExternalTeam
	err := r.db.Where("is_active = ?", true).Order("name ASC").Find(&data).Error
	return data, err
}

func (r *masterRepository) GetImpacts() ([]domain.MstImpact, error) {
	var data []domain.MstImpact
	err := r.db.Where("is_active = ?", true).Order("name ASC").Find(&data).Error
	return data, err
}

func (r *masterRepository) GetPriorities() ([]domain.MstPriority, error) {
	var data []domain.MstPriority
	err := r.db.Where("is_active = ?", true).Order("level ASC").Find(&data).Error
	return data, err
}

func (r *masterRepository) GetRoles() ([]domain.MstRole, error) {
	var data []domain.MstRole
	err := r.db.Find(&data).Error
	return data, err
}