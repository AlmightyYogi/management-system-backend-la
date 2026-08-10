package service

import (
	"errors"
	"math"
	"time"

	"github.com/AlmightyOggy/management-system/internal/domain"
	"github.com/AlmightyOggy/management-system/internal/dto"
	"github.com/AlmightyOggy/management-system/internal/repository"
	"github.com/AlmightyOggy/management-system/internal/utils"
)

type ReportExternalTeamService interface {
	CreateExternalTeam(req dto.CreateReportExternalTeamRequest, newFiles []string) (*domain.ReportExternalTeam, error)
	UpdateExternalTeam(id uint, req dto.UpdateReportExternalTeamRequest, newFiles []string, existingFiles []string) (*domain.ReportExternalTeam, error)
	DeleteExternalTeam(id uint) (reportUUID string, err error)
	GetByReportID(reportID uint) ([]domain.ReportExternalTeam, error)
}

type reportExternalTeamService struct {
	externalTeamRepo repository.ReportExternalTeamRepository
	reportRepo		 repository.ReportRepository
}

func NewReportExternalTeamService(externalTeamRepo repository.ReportExternalTeamRepository, reportRepo repository.ReportRepository) ReportExternalTeamService {
	return &reportExternalTeamService{
		externalTeamRepo: 	externalTeamRepo,
		reportRepo: 		reportRepo,
	}
}

func (s *reportExternalTeamService) CreateExternalTeam(req dto.CreateReportExternalTeamRequest, newFiles []string) (*domain.ReportExternalTeam, error) {
	report, err := s.reportRepo.FindByUUID(req.ReportID)
	if err != nil {
		return nil, errors.New("report not found")
	}

	duration := calculateDurationMinutes(&req.StartTime, req.EndTime)

	ext := &domain.ReportExternalTeam{
		ReportID:             report.ID,
		ExternalTeams:        req.ExternalTeamID,
		PIC:                  req.PIC,
		StartTime:            &req.StartTime,
		EndTime:              req.EndTime,
		Duration:             &duration,
		EvidenceFileExternal: domain.StringArray(newFiles),
	}

	if err := s.externalTeamRepo.Create(ext); err != nil {
		return nil, err
	}

	if err := s.recalculateTotalDuration(report.ID); err != nil {
		return nil, err
	}

	updated, err := s.externalTeamRepo.FindByID(ext.ID)
	if err != nil {
		return ext, nil
	}

	return updated, nil
}

func (s *reportExternalTeamService) UpdateExternalTeam(id uint, req dto.UpdateReportExternalTeamRequest, newFiles []string, existingFiles []string) (*domain.ReportExternalTeam, error) {
	ext, err := s.externalTeamRepo.FindByID(id)
	if err != nil {
		return nil, errors.New("external team data not found")
	}

	if req.PIC != "" {
		ext.PIC = req.PIC
	}

	if req.StartTime != nil {
		ext.StartTime = req.StartTime
	}

	if req.EndTime != nil {
		ext.EndTime = req.EndTime
	}

	duration := calculateDurationMinutes(ext.StartTime, ext.EndTime)
	ext.Duration = &duration

	merged := append(existingFiles, newFiles...)
	if len(merged) > 0 {
		ext.EvidenceFileExternal = domain.StringArray(merged)
	} else {
		ext.EvidenceFileExternal = nil
	}

	if err := s.externalTeamRepo.Update(ext); err != nil {
		return nil, err
	}

	if err := s.recalculateTotalDuration(ext.ReportID); err != nil {
		return nil, err
	}

	updated, err := s.externalTeamRepo.FindByID(ext.ID)
	if err != nil {
		return ext, nil
	}

	return updated, nil
}

func (s *reportExternalTeamService) DeleteExternalTeam(id uint) (string, error) {
	ext, err := s.externalTeamRepo.FindByID(id)
	if err != nil {
		return "", errors.New("external team data not found")
	}

	report, err := s.reportRepo.GetByID(ext.ReportID)
	if err != nil {
		return "", errors.New("report not found")
	}

	if len(ext.EvidenceFileExternal) > 0 {
		utils.DeleteFilesByFolder("external_evidences", []string(ext.EvidenceFileExternal))
	}

	if err := s.externalTeamRepo.Delete(id); err != nil {
		return "", err
	}

	if err := s.recalculateTotalDuration(ext.ReportID); err != nil {
		return "", err
	}

	return report.UUID.String(), nil
}

func (s *reportExternalTeamService) GetByReportID(reportID uint) ([]domain.ReportExternalTeam, error) {
	return s.externalTeamRepo.FindByReportID(reportID)
}

func (s *reportExternalTeamService) recalculateTotalDuration(reportID uint) error {
	total, err := s.externalTeamRepo.SumDurationByReportID(reportID)
	if err != nil {
		return err
	}
	return s.externalTeamRepo.UpdateTotalExternalDuration(reportID, total)
}

func calculateDurationMinutes(start *time.Time, end *time.Time) int64 {
	if start == nil || end == nil {
		return 0
	}
	return int64(math.Abs(end.Sub(*start).Minutes()))
}