package handler

import (
	"net/http"

	"github.com/AlmightyOggy/management-system/internal/repository"
	"github.com/AlmightyOggy/management-system/internal/utils"
	"github.com/gin-gonic/gin"
)

type MasterHandler struct {
	masterRepo repository.MasterRepository
}

func NewMasterHandler(masterRepo repository.MasterRepository) *MasterHandler {
	return &MasterHandler{masterRepo: masterRepo}
}

func (h *MasterHandler) GetAll(c *gin.Context) {
	reportType := c.Query("type")

	severities, err := h.masterRepo.GetSeverities()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to fetch severities", err)
		return
	}

	apps, err := h.masterRepo.GetApps()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to fetch apps", err)
		return
	}

	assignedTo, err := h.masterRepo.GetAssignedTo()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to fetch assigned_to", err)
		return
	}

	scopes, err := h.masterRepo.GetScopes(reportType)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to fetch scopes", err)
		return
	}

	externalTeams, err := h.masterRepo.GetExternalTeams()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to fetch external teams", err)
		return
	}

	impacts, err := h.masterRepo.GetImpacts()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to fetch impacts", err)
		return
	}

	priorities, err := h.masterRepo.GetPriorities()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to fetch priorities", err)
		return
	}

	roles, err := h.masterRepo.GetRoles()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to fetch roles", err)
		return
	}

	utils.Success(c, "Master data retrieved", gin.H{
		"severities":     severities,
		"apps":           apps,
		"assigned_to":    assignedTo,
		"scopes":         scopes,
		"external_teams": externalTeams,
		"impacts":        impacts,
		"priorities":     priorities,
		"roles":		  roles,
	})
}

func (h *MasterHandler) GetExternalTeams(c *gin.Context) {
	data, err := h.masterRepo.GetExternalTeams()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to fetch external teams", err)
		return
	}
	utils.Success(c, "External teams retrieved", data)
}

func (h *MasterHandler) GetScopes(c *gin.Context) {
	reportType := c.Query("type")
	data, err := h.masterRepo.GetScopes(reportType)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to fetch scopes", err)
		return
	}
	utils.Success(c, "Scopes retrieved", data)
}