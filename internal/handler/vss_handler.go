package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/AlmightyOggy/management-system/internal/repository"
	"github.com/AlmightyOggy/management-system/internal/service"
	"github.com/AlmightyOggy/management-system/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

type VSSHandler struct {
	svc service.VSSMonitorService
}

func NewVSSHandler(svc service.VSSMonitorService) *VSSHandler {
	return &VSSHandler{svc: svc}
}

func (h *VSSHandler) ListDelays(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	filter := repository.VSSDelayFilter{
		DeviceID:   c.Query("device_id"),
		DeviceName: c.Query("device_name"),
		Reason:     c.Query("reason"),
		Date:       c.Query("date"),
		Page:       page,
		PerPage:    perPage,
	}

	result, err := h.svc.ListDelays(filter)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to read VSS delay logs", err)
		return
	}
	utils.Success(c, "OK", result)
}

func (h *VSSHandler) ListHistory(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	filter := repository.VSSHistoryFilter{
		DeviceID:   c.Query("device_id"),
		DeviceName: c.Query("device_name"),
		Reason:     c.Query("reason"),
		YearMonth:  c.Query("year_month"),
		StartDate:  c.Query("start_date"),
		EndDate:    c.Query("end_date"),
		Page:       page,
		PerPage:    perPage,
		SortBy:     c.DefaultQuery("sort_by", "detected_at"),
		SortDir:    c.DefaultQuery("sort_dir", "desc"),
	}

	rows, total, err := h.svc.ListHistory(filter)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to list VSS history", err)
		return
	}
	utils.Success(c, "OK", gin.H{
		"data":     rows,
		"total":    total,
		"page":     page,
		"per_page": perPage,
	})
}

func (h *VSSHandler) ListLive(c *gin.Context) {
	onlyIssue := c.Query("only_issue") == "1" || strings.EqualFold(c.Query("only_issue"), "true")
	page, _    := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "50"))

	res := h.svc.ListLive(
		c.Query("device_name"),
		c.Query("action"),
		c.Query("status"),
		onlyIssue,
		page,
		perPage,
		c.DefaultQuery("sort_by", "delay_sec"),
		c.DefaultQuery("sort_dir", "asc"),
	)
	utils.Success(c, "OK", res)
}

func (h *VSSHandler) GetTraffic(c *gin.Context) {
	res := h.svc.GetTraffic()
	utils.Success(c, "OK", res)
}

func (h *VSSHandler) GetDaySummary(c *gin.Context) {
	date := c.Query("date")
	res, err := h.svc.GetDaySummary(date)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error(), err)
		return
	}
	utils.Success(c, "OK", res)
}

func (h *VSSHandler) ExportDayDelayExcel(c *gin.Context) {
	date := c.Query("date")
	rows, err := h.svc.GetDayDelayExport(date)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error(), err)
		return
	}

	f := excelize.NewFile()
	sheet := "Rekap Delay"
	_ = f.SetSheetName("Sheet1", sheet)

	headers := []string{"No", "Name", "Device ID", "Delay Max", "Delay Min", "Delay Avg"}
	for i, htxt := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, htxt)
	}

	// style header
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"6366F1"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	_ = f.SetCellStyle(sheet, "A1", "F1", headerStyle)

	formatSec := func(sec int64) string {
		neg := sec < 0
		s := sec
		if s < 0 {
			s = -s
		}
		var out string
		if s < 60 {
			out = fmt.Sprintf("%ds", s)
		} else {
			h := s / 3600
			m := (s % 3600) / 60
			secR := s % 60
			parts := []string{}
			if h > 0 {
				parts = append(parts, fmt.Sprintf("%dh", h))
			}
			if m > 0 {
				parts = append(parts, fmt.Sprintf("%dm", m))
			}
			if secR > 0 || len(parts) == 0 {
				parts = append(parts, fmt.Sprintf("%ds", secR))
			}
			out = strings.Join(parts, " ")
		}
		if neg {
			return "-" + out
		}
		return out
	}

	formatAvg := func(v float64) string {
		return formatSec(int64(v + 0.5))
	}

	for i, r := range rows {
		row := i + 2
		_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", row), i+1)
		_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", row), r.DeviceName)
		_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", row), r.DeviceID)
		_ = f.SetCellValue(sheet, fmt.Sprintf("D%d", row), formatSec(r.MaxDelaySec))
		_ = f.SetCellValue(sheet, fmt.Sprintf("E%d", row), formatSec(r.MinDelaySec))
		_ = f.SetCellValue(sheet, fmt.Sprintf("F%d", row), formatAvg(r.AvgDelaySec))
	}

	_ = f.SetColWidth(sheet, "A", "A", 6)
	_ = f.SetColWidth(sheet, "B", "B", 18)
	_ = f.SetColWidth(sheet, "C", "C", 22)
	_ = f.SetColWidth(sheet, "D", "F", 14)

	if date == "" {
		date = "export"
	}
	filename := fmt.Sprintf("vss-rekap-delay-%s.xlsx", date)
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Header("Content-Transfer-Encoding", "binary")

	if err := f.Write(c.Writer); err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to write excel", err)
		return
	}
}