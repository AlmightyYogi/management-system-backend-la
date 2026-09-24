package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/AlmightyOggy/management-system/internal/repository"
	"github.com/AlmightyOggy/management-system/internal/service"
	"github.com/AlmightyOggy/management-system/internal/utils"
	"github.com/gin-gonic/gin"
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
	)
	utils.Success(c, "OK", res)
}