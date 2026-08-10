package service

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/AlmightyOggy/management-system/internal/domain"
	"github.com/AlmightyOggy/management-system/internal/dto"
	"github.com/AlmightyOggy/management-system/internal/repository"
	"github.com/AlmightyOggy/management-system/internal/utils"
	"github.com/google/uuid"
)

type ReportService interface {
	CreateReport(req dto.CreateReportRequest, files []string) (*domain.Report, error)
	UpdateReport(uuidStr string, req dto.UpdateReportRequest, newFiles []string, existingDowntimeFiles []string, newRestorationFiles []string, existingRestorationFiles []string) (*domain.Report, error)
	GetReportByUUID(uuidStr string) (*domain.Report, error)
	GetAllReports(filter repository.ReportFilter) ([]domain.Report, int64, error)
	CountReports(filter repository.ReportFilter) (int64, error)
	MarkRestored(uuidStr string) (*domain.Report, error)
	ToggleHandled(uuidStr string, handled bool) (*domain.Report, error)
	GenerateIncidentCode(reportType string) string
	ExportReports(filter repository.ReportFilter) ([]domain.Report, error)
	SendOpenTicketReminder() error
}

type reportService struct {
	reportRepo repository.ReportRepository
}

func NewReportService(reportRepo repository.ReportRepository) ReportService {
	return &reportService{reportRepo: reportRepo}
}

func (s *reportService) GenerateIncidentCode(reportType string) string {
	prefix := "INC"
	switch reportType {
	case "Request":
		prefix = "REQ"
	case "Activity":
		prefix = "ACT"
	}

	lastReport, err := s.reportRepo.FindLastByPrefix(prefix)
	if err != nil || lastReport == nil {
		return prefix + "00001"
	}

	incidentNum := lastReport.Incident[len(prefix):]
	number, err := strconv.Atoi(incidentNum)
	if err != nil {
		return prefix + "00001"
	}

	return prefix + fmt.Sprintf("%05d", number+1)
}

func (s *reportService) CreateReport(req dto.CreateReportRequest, files []string) (*domain.Report, error) {
	fmt.Printf("[DEBUG CreateReport] Request: %+v | Files: %d\n", req, len(files))
	requestDate, err := time.Parse("2006-01-02", req.RequestDate)
	if err != nil {
		return nil, errors.New("invalid request_date format, use YYYY-MM-DD")
	}

	if !isValidTime(req.ReportTime) {
		return nil, errors.New("invalid report_time format, use HH:MM")
	}

	incidentCode := s.GenerateIncidentCode(req.Type)
	now := time.Now()

	status := req.Status
	if status == 0 {
		status = 1
	}

	report := &domain.Report{
		UUID:                  uuid.New(),
		Incident:              incidentCode,
		Requestor:             req.Requestor,
		RequestorEmail:        req.RequestorEmail,
		RequestDate:           requestDate,
		ReportTime:            req.ReportTime,
		Apps:                  req.Apps,
		Type:                  req.Type,
		Severity:              req.Severity,
		AssignedTo:            req.AssignedTo,
		Scope:                 req.Scope,
		Description:           req.Description,
		Status:                status,
		ResponseTime:          &now,
		FileDowntimeEvidence:  domain.StringArray(files),
		CreatedAt:             now,
	}

	if err := s.reportRepo.Create(report); err != nil {
		return nil, err
	}

	return report, nil
}

func (s *reportService) UpdateReport(
	uuidStr string,
	req dto.UpdateReportRequest,
	newDowntimeFiles []string,
	existingDowntimeFiles []string,
	newRestorationFiles []string,
	existingRestorationFiles []string,
) (*domain.Report, error) {
	report, err := s.reportRepo.FindByUUID(uuidStr)
	if err != nil {
		return nil, errors.New("report not found")
	}

	if req.Requestor != "" {
		report.Requestor = req.Requestor
	}
	if req.RequestorEmail != "" {
		report.RequestorEmail = req.RequestorEmail
	}
	if req.RequestDate != "" {
		parsed, err := time.Parse("2006-01-02", req.RequestDate)
		if err == nil {
			report.RequestDate = parsed
		}
	}
	if req.ReportTime != "" {
		if !isValidTime(req.ReportTime) {
			return nil, errors.New("invalid report_time format, use HH:MM")
		}
		report.ReportTime = req.ReportTime
	}
	if req.Apps != "" {
		report.Apps = req.Apps
	}
	if req.Type != "" && req.Type != report.Type {
		report.Incident = s.GenerateIncidentCode(req.Type)
		report.Type = req.Type
	}
	if req.Severity != "" {
		report.Severity = req.Severity
	}
	if req.AssignedTo != "" {
		report.AssignedTo = req.AssignedTo
	}
	if req.Scope != "" {
		report.Scope = req.Scope
	}
	if req.Description != "" {
		report.Description = req.Description
	}
	if req.Resolution != "" {
		report.Resolution = req.Resolution
	}
	if req.RCA != "" {
		report.RCA = req.RCA
	}

	if req.Status != nil {
		newStatus := *req.Status
		oldStatus := report.Status

		if newStatus == 0 && oldStatus != 0 {
			now := time.Now()
			report.ClosedAt = &now
			report.ResolvedTime = &now
			report.Status = newStatus
		} else if newStatus != 0 && oldStatus == 0 {
			report.ClosedAt = nil
			report.ResolvedTime = nil
			report.Status = newStatus
		} else {
			report.Status = newStatus
		}
	}

	if req.HandledBy != nil {
		if *req.HandledBy {
			report.HandledBy = 1
		} else {
			report.HandledBy = 0
		}
	}

	if req.ServicerestoredTime != nil && *req.ServicerestoredTime != "" {
		parsedRestored, err := parseFlexibleTime(*req.ServicerestoredTime)
		if err != nil {
			return nil, fmt.Errorf("invalid servicerestored_time: %v", err)
		}
		if parsedRestored.IsZero() {
			return nil, errors.New("servicerestored_time tidak valid, hasil parse adalah zero time")
		}

		report.ServicerestoredTime = &parsedRestored

		createdAt := report.CreatedAt
		diffSeconds := int64(math.Abs(parsedRestored.Sub(createdAt).Seconds()))
		report.RestoredTime    = &diffSeconds
		internalDuration       := diffSeconds
		report.TotalInternalDuration = &internalDuration
	} else {
		// Jika tidak dikirim, jangan ubah nilai yang sudah ada
		// (tidak set nil supaya tidak menimpa data existing)
	}

	mergedDowntime := append(existingDowntimeFiles, newDowntimeFiles...)
	if len(mergedDowntime) > 0 {
		report.FileDowntimeEvidence = domain.StringArray(mergedDowntime)
	} else {
		report.FileDowntimeEvidence = nil
	}

	mergedRestoration := append(existingRestorationFiles, newRestorationFiles...)
	if len(mergedRestoration) > 0 {
		report.RestorationEvidence = domain.StringArray(mergedRestoration)
	} else {
		report.RestorationEvidence = nil
	}

	if err := s.reportRepo.Update(report); err != nil {
		return nil, err
	}

	return report, nil
}

func (s *reportService) GetAllReports(filter repository.ReportFilter) ([]domain.Report, int64, error) {
	return s.reportRepo.FindWithFilters(filter)
}

func (s *reportService) CountReports(filter repository.ReportFilter) (int64, error) {
	return s.reportRepo.CountWithFilters(filter)
}

func (s *reportService) GetReportByUUID(uuidStr string) (*domain.Report, error) {
	return s.reportRepo.FindByUUID(uuidStr)
}

func (s *reportService) MarkRestored(uuidStr string) (*domain.Report, error) {
	report, err := s.reportRepo.FindByUUID(uuidStr)
	if err != nil {
		return nil, errors.New("report not found")
	}

	if report.ServicerestoredTime != nil {
		return nil, errors.New("already restored")
	}

	now := time.Now()
	diffSeconds := int64(math.Abs(now.Sub(report.CreatedAt).Seconds()))

	internalDuration := diffSeconds

	report.ServicerestoredTime = &now
	report.RestoredTime = &diffSeconds
	report.TotalInternalDuration = &internalDuration
	report.Status = 2

	if err := s.reportRepo.Update(report); err != nil {
		return nil, err
	}

	return report, nil
}

func (s *reportService) ToggleHandled(uuidStr string, handled bool) (*domain.Report, error) {
	report, err := s.reportRepo.FindByUUID(uuidStr)
	if err != nil {
		return nil, errors.New("report not found")
	}

	if handled {
		report.HandledBy = 1
	} else {
		report.HandledBy = 0
	}

	if err := s.reportRepo.Update(report); err != nil {
		return nil, err
	}

	return report, nil
}

func isValidTime(t string) bool {
	parts := strings.Split(t, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return false
	}
	_, err1 := strconv.Atoi(parts[0])
	_, err2 := strconv.Atoi(parts[1])
	return err1 == nil && err2 == nil
}

func parseFlexibleTime(input string) (time.Time, error) {
	formats := []string{
		"2006-01-02 15:04",
		"2006-01-02T15:04",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"02-01-2006 15:04",
		"02/01/2006 15:04",
		"01/02/2006 15:04",
		"2006-01-02T15:04:05Z",
	}

	input = strings.TrimSpace(input)

	for _, format := range formats {
		if t, err := time.Parse(format, input); err == nil {
			if !t.IsZero() {
				return t, nil
			}
		}
	}

	return time.Time{}, fmt.Errorf("format waktu tidak dikenali: %s", input)
}

func (s *reportService) ExportReports(filter repository.ReportFilter) ([]domain.Report, error) {
	return s.reportRepo.FindAllForExport(filter)
}

func (s *reportService) SendOpenTicketReminder() error {
	statusOpen := 1
	filter := repository.ReportFilter{
		Status: &statusOpen,
		Type:   "Incident",
	}

	reports, _, err := s.reportRepo.FindWithFilters(filter)
	if err != nil {
		return err
	}

	oldTickets := []domain.Report{}
	for _, r := range reports {
		if time.Since(r.CreatedAt) > 3*24*time.Hour {
			oldTickets = append(oldTickets, r)
		}
	}

	if len(oldTickets) == 0 {
		return nil
	}

	return utils.SendOpenTicketReminder(oldTickets)
}