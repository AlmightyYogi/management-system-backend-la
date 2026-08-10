package handler

import (
	"net/http"

	"github.com/AlmightyOggy/management-system/internal/dto"
	"github.com/AlmightyOggy/management-system/internal/service"
	"github.com/AlmightyOggy/management-system/internal/utils"
	"github.com/gin-gonic/gin"
)

type CommentHandler struct {
	commentService service.CommentService
}

func NewCommentHandler(commentService service.CommentService) *CommentHandler {
	return &CommentHandler{commentService: commentService}
}

func (h *CommentHandler) GetComments(c *gin.Context) {
	reportUUID := c.Param("uuid")
	comments, err := h.commentService.GetComments(reportUUID)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Gagal memuat komentar", nil)
		return
	}
	utils.Success(c, "Comments retrieved", comments)
}

func (h *CommentHandler) AddComment(c *gin.Context) {
	reportUUID := c.Param("uuid")

	var req dto.CreateCommentRequest
	if err := utils.ValidateRequest(c, &req); err != nil {
		utils.ValidationError(c, err.Error())
		return
	}

	entry, err := h.commentService.AddComment(reportUUID, req)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Gagal menyimpan komentar", nil)
		return
	}

	utils.Created(c, "Comment added", entry)
}

func (h *CommentHandler) DeleteComment(c *gin.Context) {
	reportUUID := c.Param("uuid")
	commentID := c.Param("commentId")

	requesterID := c.Query("requester_id")
	isAdmin := c.Query("is_admin") == "true"

	if err := h.commentService.DeleteComment(reportUUID, commentID, requesterID, isAdmin); err != nil {
		utils.Error(c, http.StatusForbidden, err.Error(), nil)
		return
	}

	utils.Success(c, "Comment deleted", nil)
}