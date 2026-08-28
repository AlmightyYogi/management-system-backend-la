package handler

import (
	"fmt"
	"math"
	"mime/multipart"
	"net/http"
	"strconv"
	"time"

	"github.com/AlmightyOggy/management-system/internal/domain"
	"github.com/AlmightyOggy/management-system/internal/dto"
	"github.com/AlmightyOggy/management-system/internal/repository"
	"github.com/AlmightyOggy/management-system/internal/service"
	"github.com/AlmightyOggy/management-system/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

type ReportHandler struct {
	reportService service.ReportService
}

func NewReportHandler(reportService service.ReportService) *ReportHandler {
	return &ReportHandler{reportService: reportService}
}

func (h *ReportHandler) CreateReport(c *gin.Context) {
	// Parse multipart form dulu
	if err := c.Request.ParseMultipartForm(50 << 20); err != nil {
		utils.Error(c, http.StatusBadRequest, "Failed to parse multipart form", err)
		return
	}

	var req dto.CreateReportRequest

	if err := c.ShouldBind(&req); err != nil {
		utils.ValidationError(c, "Invalid form data: "+err.Error())
		return
	}

	if err := utils.ValidateStruct(&req); err != nil {
		utils.ValidationError(c, err.Error())
		return
	}

	files, err := utils.SaveUploadedFiles(c, "file_downtime_evidence[]", "file_downtime_evidences")
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "Gagal upload file", err)
		return
	}

	fmt.Printf("[DEBUG] CreateReport - Files uploaded: %d, Type: %s\n", len(files), req.Type)

	report, err := h.reportService.CreateReport(req, files)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to create report", err)
		return
	}

	utils.Created(c, "Report created successfully", toReportResponse(report))
}

func (h *ReportHandler) GetAllReports(c *gin.Context) {
	filter := repository.ReportFilter{
		Search:    c.Query("search"),
		StartDate: c.Query("start_date"),
		EndDate:   c.Query("end_date"),
		Type:      c.Query("type"),
		Page:      getIntQuery(c, "page", 1),
		PerPage:   getIntQuery(c, "per_page", 15),
	}

	if statusStr := c.Query("status"); statusStr != "" {
		if status, err := strconv.Atoi(statusStr); err == nil {
			filter.Status = &status
		}
	}

	reports, total, err := h.reportService.GetAllReports(filter)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to fetch reports", err)
		return
	}

	utils.Success(c, "Reports retrieved successfully", gin.H{
		"data":  reports,
		"total": total,
		"page":  filter.Page,
	})
}

func (h *ReportHandler) GetReportByUUID(c *gin.Context) {
	uuid := c.Param("uuid")
	report, err := h.reportService.GetReportByUUID(uuid)
	if err != nil {
		utils.Error(c, http.StatusNotFound, "Report not found", err)
		return
	}

	utils.Success(c, "Report retrieved successfully", toReportResponse(report))
}

func (h *ReportHandler) UpdateReport(c *gin.Context) {
	uuid := c.Param("uuid")

	_ = c.Request.ParseMultipartForm(50 << 20)

	var downtimeFileHeaders []*multipart.FileHeader
	var restorationFileHeaders []*multipart.FileHeader

	if c.Request.MultipartForm != nil {
		downtimeFileHeaders  = c.Request.MultipartForm.File["file_downtime_evidence[]"]
		restorationFileHeaders = c.Request.MultipartForm.File["restoration_evidence[]"]

		if len(downtimeFileHeaders) == 0 {
			downtimeFileHeaders = c.Request.MultipartForm.File["file_downtime_evidence"]
		}
		if len(restorationFileHeaders) == 0 {
			restorationFileHeaders = c.Request.MultipartForm.File["restoration_evidence"]
		}
	}

	existingDowntime     := utils.GetExistingFiles(c, "existing_downtime_files[]")
	existingRestoration  := utils.GetExistingFiles(c, "existing_restoration_files[]")

	var req dto.UpdateReportRequest
	if err := c.ShouldBind(&req); err != nil {
		utils.ValidationError(c, err.Error())
		return
	}
	if err := utils.ValidateStruct(&req); err != nil {
		utils.ValidationError(c, err.Error())
		return
	}

	newDowntimeFiles, err := utils.SaveFileHeaders(downtimeFileHeaders, "file_downtime_evidences")
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "Gagal upload file downtime", err)
		return
	}

	newRestorationFiles, err := utils.SaveFileHeaders(restorationFileHeaders, "restoration_evidence")
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "Gagal upload restoration evidence", err)
		return
	}

	report, err := h.reportService.UpdateReport(
		uuid,
		req,
		newDowntimeFiles,
		existingDowntime,
		newRestorationFiles,
		existingRestoration,
	)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to update report", err)
		return
	}

	utils.Success(c, "Report updated successfully", report)
}

func (h *ReportHandler) DeleteReport(c *gin.Context) {
	uuid := c.Param("uuid")
	if uuid == "" {
		utils.Error(c, http.StatusBadRequest, "UUID required", nil)
		return
	}

	if err := h.reportService.DeleteReport(uuid); err != nil {
		if err.Error() == "report not found" {
			utils.Error(c, http.StatusNotFound, "Report not found", err)
			return
		}
		utils.Error(c, http.StatusInternalServerError, "Failed to delete report", err)
		return
	}

	utils.Success(c, "Report deleted successfully", nil)
}

func (h *ReportHandler) MarkRestored(c *gin.Context) {
	uuid := c.Param("uuid")

	report, err := h.reportService.MarkRestored(uuid)
	if err != nil {
		if err.Error() == "already restored" {
			utils.Error(c, http.StatusConflict, "Already restored", err)
			return
		}
		utils.Error(c, http.StatusInternalServerError, "Failed to mark restored", err)
		return
	}

	utils.Success(c, "Service marked as restored", toReportResponse(report))
}

func (h *ReportHandler) ToggleHandled(c *gin.Context) {
	uuid := c.Param("uuid")

	var req dto.ToggleHandledRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ValidationError(c, err.Error())
		return
	}

	report, err := h.reportService.ToggleHandled(uuid, req.HandledBy)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to toggle handled", err)
		return
	}

	utils.Success(c, "Handled status updated", toReportResponse(report))
}

func (h *ReportHandler) ExportCount(c *gin.Context) {
	filter := repository.ReportFilter{
		Search: 	c.Query("search"),
		StartDate: 	c.Query("start_date"),
		EndDate: 	c.Query("end_date"),
	}

	count, err := h.reportService.CountReports(filter)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to count reports", err)
		return
	}

	utils.Success(c, "Count retrieved", gin.H{"count": count})
}

func (h *ReportHandler) ExportRCA(c *gin.Context) {
	uuid := c.Param("uuid")

	var req struct {
		DocumentDate string `json:"document_date"`
		Contributors []struct {
			Name       string `json:"name"`
			SignedDate string `json:"signed_date"`
		} `json:"contributors"`
		Objectives   string `json:"objectives"`
		Kronologi    string `json:"kronologi"`
		RootCause    string `json:"root_cause"`
		LessonLearnt string `json:"lesson_learnt"`
		Tindakan     string `json:"tindakan"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request data")
		return
	}

	report, err := h.reportService.GetReportByUUID(uuid)
	if err != nil {
		utils.NotFound(c, "Report not found")
		return
	}

	contributors := make([]utils.Contributor, 0, len(req.Contributors))
	for _, ct := range req.Contributors {
		contributors = append(contributors, utils.Contributor{
			Name:       ct.Name,
			SignedDate: ct.SignedDate,
		})
	}

	pdfBytes, err := utils.GenerateRCAPDF(
		report,
		contributors,
		req.DocumentDate,
		req.Objectives,
		req.Kronologi,
		req.RootCause,
		req.LessonLearnt,
		req.Tindakan,
	)
	if err != nil {
		utils.Error(c, 500, "Failed to generate PDF", err)
		return
	}

	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=RCA_%s.pdf", report.Incident))
	c.Data(200, "application/pdf", pdfBytes)
}

func toReportResponse(r *domain.Report) gin.H {
	return gin.H{
		"id":                     r.ID,
		"uuid":                   r.UUID.String(),
		"incident":               r.Incident,
		"requestor":              r.Requestor,
		"requestor_email":        r.RequestorEmail,
		"request_date":           r.RequestDate.Format("2006-01-02"),
		"report_time":            r.ReportTime,
		"apps":                   r.Apps,
		"type":                   r.Type,
		"severity":               r.Severity,
		"assigned_to":            r.AssignedTo,
		"scope":                  r.Scope,
		"description":            r.Description,
		"resolution":             r.Resolution,
		"rca":                    r.RCA,
		"status":                 r.Status,
		"handled_by":             r.HandledBy,
		"response_time":          r.ResponseTime,
		"servicerestored_time":   r.ServicerestoredTime,
		"restored_time":          r.RestoredTime,
		"resolved_time":          r.ResolvedTime,
		"closed_at":              r.ClosedAt,
		"file_downtime_evidence": r.FileDowntimeEvidence,
		"restoration_evidence":   r.RestorationEvidence,
		"created_at":             r.CreatedAt,
		"updated_at":             r.UpdatedAt,
	}
}

func getIntQuery(c *gin.Context, key string, defaultValue int) int {
	if val := c.Query(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultValue
}

func (h *ReportHandler) ExportExcel(c *gin.Context) {
	filter := repository.ReportFilter{
		Search: 	c.Query("search"),
		StartDate: 	c.Query("start_date"),
		EndDate: 	c.Query("end_date"),
	}

	reports, err := h.reportService.ExportReports(filter)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to fetch reports for export", err)
		return
	}

	f := excelize.NewFile()
	defer f.Close()

	sheet := "Reports"
	f.SetSheetName("Sheet1", sheet)

	titleStyle, _ := f.NewStyle(&excelize.Style{
        Font:      &excelize.Font{Bold: true, Size: 14, Color: "FFFFFF", Family: "Arial"},
        Fill:      excelize.Fill{Type: "pattern", Color: []string{"1F4E79"}, Pattern: 1},
        Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
    })
    infoStyle, _ := f.NewStyle(&excelize.Style{
        Font: &excelize.Font{Size: 10, Color: "FFFFFF", Italic: true, Family: "Arial"},
        Fill: excelize.Fill{Type: "pattern", Color: []string{"2E75B6"}, Pattern: 1},
        Alignment: &excelize.Alignment{Horizontal: "center"},
    })
    headerStyle, _ := f.NewStyle(&excelize.Style{
        Font:      &excelize.Font{Bold: true, Color: "FFFFFF", Size: 10, Family: "Arial"},
        Fill:      excelize.Fill{Type: "pattern", Color: []string{"1F4E79"}, Pattern: 1},
        Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
        Border: []excelize.Border{
            {Type: "left", Color: "B0C4DE", Style: 1},
            {Type: "right", Color: "B0C4DE", Style: 1},
            {Type: "top", Color: "B0C4DE", Style: 1},
            {Type: "bottom", Color: "B0C4DE", Style: 1},
        },
    })

    lastCol := "Z"
    mergeRange := fmt.Sprintf("A1:%s1", lastCol)
    f.MergeCell(sheet, "A1", lastCol+"1")
    f.MergeCell(sheet, "A2", lastCol+"2")
    f.MergeCell(sheet, "A3", lastCol+"3")

    f.SetCellValue(sheet, "A1", "REPORT EXPORT")
    f.SetCellValue(sheet, "A2", "Exported at: "+time.Now().Format("02 January 2006 15:04"))
    f.SetCellValue(sheet, "A3", fmt.Sprintf("Total Records: %d", len(reports)))

    f.SetCellStyle(sheet, "A1", lastCol+"1", titleStyle)
    f.SetCellStyle(sheet, "A2", lastCol+"2", infoStyle)
    f.SetCellStyle(sheet, "A3", lastCol+"3", infoStyle)

    f.SetRowHeight(sheet, 1, 28)
    f.SetRowHeight(sheet, 2, 18)
    f.SetRowHeight(sheet, 3, 18)

    _ = mergeRange

    headers := []string{
        "No", "Incident Code", "Requestor", "Requestor Email",
        "Request Date", "Report Time", "Application", "Type",
        "Severity / Priority / Impact", "Assigned To", "Scope",
        "Description", "Resolution", "Root Cause Analysis (RCA)",
        "Status", "Handled by External Team",
        "Restored Duration (HH:MM:SS)", "Internal Duration (HH:MM:SS)",
        "Service Restored At", "Closed At", "Created At",
        "File Downtime Evidence", "Restoration Evidence",
    }

    colWidths := map[string]float64{
        "A": 6,  "B": 18, "C": 22, "D": 30,
        "E": 16, "F": 14, "G": 24, "H": 14,
        "I": 28, "J": 28, "K": 22, "L": 40,
        "M": 40, "N": 40, "O": 16, "P": 22,
        "Q": 26, "R": 26, "S": 22, "T": 22,
        "U": 22, "V": 35, "W": 35,
    }
    for col, width := range colWidths {
        f.SetColWidth(sheet, col, col, width)
    }

    cols := []string{"A","B","C","D","E","F","G","H","I","J","K","L","M","N","O","P","Q","R","S","T","U","V","W"}
    for i, h := range headers {
        cell := fmt.Sprintf("%s4", cols[i])
        f.SetCellValue(sheet, cell, h)
        f.SetCellStyle(sheet, cell, cell, headerStyle)
    }
    f.SetRowHeight(sheet, 4, 30)

    evenStyle, _ := f.NewStyle(&excelize.Style{
        Fill: excelize.Fill{Type: "pattern", Color: []string{"D6E4F0"}, Pattern: 1},
        Font: &excelize.Font{Size: 10, Family: "Arial"},
        Alignment: &excelize.Alignment{Vertical: "top", WrapText: true},
        Border: []excelize.Border{
            {Type: "left", Color: "B0C4DE", Style: 1},
            {Type: "right", Color: "B0C4DE", Style: 1},
            {Type: "bottom", Color: "B0C4DE", Style: 1},
        },
    })
    oddStyle, _ := f.NewStyle(&excelize.Style{
        Fill: excelize.Fill{Type: "pattern", Color: []string{"FFFFFF"}, Pattern: 1},
        Font: &excelize.Font{Size: 10, Family: "Arial"},
        Alignment: &excelize.Alignment{Vertical: "top", WrapText: true},
        Border: []excelize.Border{
            {Type: "left", Color: "B0C4DE", Style: 1},
            {Type: "right", Color: "B0C4DE", Style: 1},
            {Type: "bottom", Color: "B0C4DE", Style: 1},
        },
    })

    statusStyleMap := map[string]*excelize.Style{
        "Open":         {Font: &excelize.Font{Bold: true, Color: "FFFFFF", Size: 10}, Fill: excelize.Fill{Type: "pattern", Color: []string{"198754"}, Pattern: 1}, Alignment: &excelize.Alignment{Horizontal: "center"}},
        "Closed":       {Font: &excelize.Font{Bold: true, Color: "FFFFFF", Size: 10}, Fill: excelize.Fill{Type: "pattern", Color: []string{"DC3545"}, Pattern: 1}, Alignment: &excelize.Alignment{Horizontal: "center"}},
        "Restored":     {Font: &excelize.Font{Bold: true, Color: "000000", Size: 10}, Fill: excelize.Fill{Type: "pattern", Color: []string{"FFC107"}, Pattern: 1}, Alignment: &excelize.Alignment{Horizontal: "center"}},
        "Done":         {Font: &excelize.Font{Bold: true, Color: "FFFFFF", Size: 10}, Fill: excelize.Fill{Type: "pattern", Color: []string{"198754"}, Pattern: 1}, Alignment: &excelize.Alignment{Horizontal: "center"}},
        "Done Partial": {Font: &excelize.Font{Bold: true, Color: "FFFFFF", Size: 10}, Fill: excelize.Fill{Type: "pattern", Color: []string{"FD7E14"}, Pattern: 1}, Alignment: &excelize.Alignment{Horizontal: "center"}},
        "Rollback":     {Font: &excelize.Font{Bold: true, Color: "FFFFFF", Size: 10}, Fill: excelize.Fill{Type: "pattern", Color: []string{"DC3545"}, Pattern: 1}, Alignment: &excelize.Alignment{Horizontal: "center"}},
    }

    statusText := func(status int) string {
        switch status {
        case 0: return "Closed"
        case 1: return "Open"
        case 2: return "Restored"
        case 4: return "Done"
        case 5: return "Done Partial"
        case 6: return "Rollback"
        default: return "Unknown"
        }
    }

    fmtSeconds := func(sec int64) string {
        s := int64(math.Abs(float64(sec)))
        h := s / 3600
        m := (s % 3600) / 60
        ss := s % 60
        return fmt.Sprintf("%02d:%02d:%02d", h, m, ss)
    }

    joinFiles := func(files domain.StringArray) string {
        if len(files) == 0 { return "-" }
        result := ""
        for i, f := range files {
            if i > 0 { result += ", " }
            result += f
        }
        return result
    }

    for i, report := range reports {
        row := i + 5
        baseStyle := oddStyle
        if row%2 == 0 { baseStyle = evenStyle }

        statusStr := statusText(report.Status)

        restoredDuration := "-"
        if report.RestoredTime != nil {
            restoredDuration = fmtSeconds(*report.RestoredTime)
        }

        internalDuration := "-"
        if report.TotalInternalDuration != nil {
            internalDuration = fmtSeconds(*report.TotalInternalDuration)
        }

        serviceRestoredAt := "-"
        if report.ServicerestoredTime != nil {
            serviceRestoredAt = report.ServicerestoredTime.Format("02/01/2006 15:04")
        }

        closedAt := "-"
        if report.ClosedAt != nil {
            closedAt = report.ClosedAt.Format("02/01/2006 15:04")
        }

        handledBy := "No"
        if report.HandledBy == 1 { handledBy = "Yes" }

        rowData := []interface{}{
            i + 1,
            report.Incident,
            report.Requestor,
            report.RequestorEmail,
            report.RequestDate.Format("02/01/2006"),
            report.ReportTime,
            report.Apps,
            report.Type,
            report.Severity,
            report.AssignedTo,
            report.Scope,
            report.Description,
            report.Resolution,
            report.RCA,
            statusStr,
            handledBy,
            restoredDuration,
            internalDuration,
            serviceRestoredAt,
            closedAt,
            report.CreatedAt.Format("02/01/2006 15:04"),
            joinFiles(report.FileDowntimeEvidence),
            joinFiles(report.RestorationEvidence),
        }

        for j, val := range rowData {
            cell := fmt.Sprintf("%s%d", cols[j], row)
            f.SetCellValue(sheet, cell, val)
        }

        f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("W%d", row), baseStyle)

        statusCell := fmt.Sprintf("O%d", row)
        if st, ok := statusStyleMap[statusStr]; ok {
            stIdx, _ := f.NewStyle(st)
            f.SetCellStyle(sheet, statusCell, statusCell, stIdx)
        }

        f.SetRowHeight(sheet, row, 18)
    }

    f.SetPanes(sheet, &excelize.Panes{
        Freeze:      true,
        Split:       false,
        XSplit:      0,
        YSplit:      4,
        TopLeftCell: "A5",
        ActivePane:  "bottomLeft",
    })
    f.AutoFilter(sheet, fmt.Sprintf("A4:W%d", len(reports)+4), nil)

    filename := fmt.Sprintf("Reports_%s.xlsx", time.Now().Format("20060102_150405"))
    c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
    c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
    c.Header("Content-Transfer-Encoding", "binary")
    c.Header("Expose-Headers", "Content-Disposition")
    c.Header("Access-Control-Expose-Headers", "Content-Disposition")

    if err := f.Write(c.Writer); err != nil {
        utils.Error(c, http.StatusInternalServerError, "Failed to write Excel file", err)
        return
	}
}