package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin/binding"
	"github.com/AlmightyOggy/management-system/internal/dto"
	"github.com/AlmightyOggy/management-system/internal/service"
	"github.com/AlmightyOggy/management-system/internal/utils"
	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	userService service.UserService
}

func NewUserHandler(userService service.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

func (h *UserHandler) Register(c *gin.Context) {
	if err := c.Request.ParseMultipartForm(10 << 20); err != nil {
		utils.Error(c, http.StatusBadRequest, "Invalid form data", err)
		return
	}

	var req dto.RegisterRequest

	if err := c.ShouldBindWith(&req, binding.Form); err != nil {
		utils.ValidationError(c, err.Error())
		return
	}

	uploadDir := filepath.Join("storage", "public", "user_images")
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to create upload directory", err)
		return
	}

	if file, err := c.FormFile("image"); err == nil {
		filename := time.Now().Format("20060102150405") + "_" + file.Filename
		dst := filepath.Join(uploadDir, filename)

		if err := c.SaveUploadedFile(file, dst); err != nil {
			utils.Error(c, http.StatusInternalServerError, "Failed to save image", err)
			return
		}

		req.Image = "user_images/" + filename
	}

	user, err := h.userService.Register(req)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	utils.Created(c, "User registered successfully", gin.H{
		"id":    user.ID,
		"uuid":  user.UUID,
		"name":  user.Name,
		"email": user.Email,
	})
}

func (h *UserHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := utils.ValidateRequest(c, &req); err != nil {
		utils.ValidationError(c, err.Error())
		return
	}

	user, token, err := h.userService.Login(req)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, err.Error(), nil)
		return
	}

	utils.Success(c, "Login successful", gin.H{
		"user": gin.H{
			"id":            user.ID,
			"uuid":          user.UUID,
			"name":          user.Name,
			"email":         user.Email,
			"phone":         user.Phone,
			"image":         user.Image,
			"role_id":       user.RoleID,
			"active":        user.Active,
			"last_login_at": user.LastLoginAt,
		},
		"token": token,
	})
}

func (h *UserHandler) GetAllUsers(c *gin.Context) {
	users, err := h.userService.GetAllUsers()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	utils.Success(c, "Users retrieved", users)
}

func (h *UserHandler) GetUserByUUID(c *gin.Context) {
	uuid := c.Param("uuid")
	if uuid == "" {
		utils.BadRequest(c, "UUID is required")
		return
	}

	user, err := h.userService.GetUserByUUID(uuid)
	if err != nil {
		if err.Error() == "user not found" {
			utils.NotFound(c, "User not found")
			return
		}
		utils.Error(c, http.StatusInternalServerError, "Failed to fetch user", err)
		return
	}

	utils.Success(c, "User retrieved successfully", user)
}

func (h *UserHandler) UpdateUser(c *gin.Context) {
	uuid := c.Param("uuid")
	if uuid == "" {
		utils.BadRequest(c, "UUID is required")
		return
	}

	if err := c.Request.ParseMultipartForm(10 << 20); err != nil {
		utils.Error(c, http.StatusBadRequest, "Invalid form data", err)
		return
	}

	var req dto.UpdateUserRequest

	if err := c.ShouldBindWith(&req, binding.Form); err != nil {
		utils.ValidationError(c, err.Error())
		return
	}

	if file, err := c.FormFile("image"); err == nil {
		uploadDir := filepath.Join("storage", "public", "user_images")
		if err := os.MkdirAll(uploadDir, 0755); err != nil {
			utils.Error(c, http.StatusInternalServerError, "Failed to create directory", err)
			return
		}

		filename := time.Now().Format("20060102150405") + "_" + file.Filename
		dst := filepath.Join(uploadDir, filename)

		if err := c.SaveUploadedFile(file, dst); err != nil {
			utils.Error(c, http.StatusInternalServerError, "Failed to save image", err)
			return
		}

		req.Image = "user_images/" + filename
	}

	user, err := h.userService.UpdateUser(uuid, req)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	utils.Success(c, "User updated successfully", user)
}