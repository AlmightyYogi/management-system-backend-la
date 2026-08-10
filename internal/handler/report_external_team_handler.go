package handler

import (
	"net/http"
	"strconv"

	"github.com/AlmightyOggy/management-system/internal/dto"
	"github.com/AlmightyOggy/management-system/internal/service"
	"github.com/AlmightyOggy/management-system/internal/utils"
	"github.com/gin-gonic/gin"
)

type ReportExternalTeamHandler struct {
	externalTeamService service.ReportExternalTeamService
}

func NewReportExternalTeamHandler(externalTeamService service.ReportExternalTeamService) *ReportExternalTeamHandler {
	return &ReportExternalTeamHandler{externalTeamService: externalTeamService}
}

func (h *ReportExternalTeamHandler) GetByReportID(c *gin.Context) {
	reportIDStr := c.Query("report_id")
	if reportIDStr == "" {
		utils.BadRequest(c, "report_id query parameter is required")
		return
	}
	reportID, err := strconv.ParseUint(reportIDStr, 10, 64)
	if err != nil {
		utils.BadRequest(c, "Invalid report_id parameter")
		return
	}

	data, err := h.externalTeamService.GetByReportID(uint(reportID))
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to fetch external team data", err)
		return
	}

	utils.Success(c, "External team data retrieved successfully", data)
}

func (h *ReportExternalTeamHandler) CreateExternalTeam(c *gin.Context) {
	if err := c.Request.ParseMultipartForm(50 << 20); err != nil {
		utils.Error(c, http.StatusBadRequest, "Invalid form data", err)
		return
	}

	fileHeaders := utils.GetMultipartFileHeaders(c, "evidence_file_external")

	var req dto.CreateReportExternalTeamRequest
	if err := c.ShouldBind(&req); err != nil {
		utils.ValidationError(c, err.Error())
		return
	}
	if err := utils.ValidateStruct(&req); err != nil {
		utils.ValidationError(c, err.Error())
		return
	}

	newFiles, err := utils.SaveFileHeaders(fileHeaders, "external_evidence")
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "Gagal upload evidence file", err)
		return
	}

	ext, err := h.externalTeamService.CreateExternalTeam(req, newFiles)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to create external team data", err)
		return
	}

	utils.Created(c, "Data external team created successfully", ext)
}

func (h *ReportExternalTeamHandler) UpdateExternalTeam(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		utils.BadRequest(c, "Invalid id parameter")
		return
	}

	if err := c.Request.ParseMultipartForm(50 << 20); err != nil {
		utils.Error(c, http.StatusBadRequest, "Invalid form data", err)
		return
	}

	fileHeaders := utils.GetMultipartFileHeaders(c, "evidence_file_external")
	existingFiles := utils.GetExistingFiles(c, "existing_files[]")

	var req dto.UpdateReportExternalTeamRequest
	if err := c.ShouldBind(&req); err != nil {
		utils.ValidationError(c, err.Error())
		return
	}
	if err := utils.ValidateStruct(&req); err != nil {
		utils.ValidationError(c, err.Error())
		return
	}

	newFiles, err := utils.SaveFileHeaders(fileHeaders, "external_evidence")
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "Gagal upload evidence file", err)
		return
	}

	ext, err := h.externalTeamService.UpdateExternalTeam(id, req, newFiles, existingFiles)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to update external team data", err)
		return
	}

	utils.Success(c, "External team data has been successfully updated", ext)
}

func (h *ReportExternalTeamHandler) DeleteExternalTeam(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		utils.BadRequest(c, "Invalid id parameter")
		return
	}

	reportUUID, err := h.externalTeamService.DeleteExternalTeam(id)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to delete external team data", err)
		return
	}

	utils.Success(c, "External data successfully deleted", gin.H{"report_uuid": reportUUID})
}

func parseUintParam(c *gin.Context, key string) (uint, error) {
	val := c.Param(key)
	parsed, err := strconv.ParseUint(val, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(parsed), nil
}