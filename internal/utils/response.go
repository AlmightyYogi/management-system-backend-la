package utils

import (
	"github.com/gin-gonic/gin"
)

type Response struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type ErrorResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

type PaginatedResponse struct {
	Status     string      `json:"status"`
	Message    string      `json:"message"`
	Data       interface{} `json:"data"`
	Pagination Pagination  `json:"pagination"`
}

type Pagination struct {
	CurrentPage int `json:"current_page"`
	PerPage     int `json:"per_page"`
	Total       int `json:"total"`
	TotalPage   int `json:"total_pages"`
}

func Success(c *gin.Context, message string, data interface{}) {
	c.JSON(200, Response{
		Status:  "success",
		Message: message,
		Data:    data,
	})
}

func Created(c *gin.Context, message string, data interface{}) {
	c.JSON(201, Response{
		Status:  "success",
		Message: message,
		Data:    data,
	})
}

func Error(c *gin.Context, statusCode int, message string, err error) {
	errorMsg := ""
	if err != nil {
		errorMsg = err.Error()
	}

	c.JSON(statusCode, ErrorResponse{
		Status:  "error",
		Message: message,
		Error:   errorMsg,
	})
}

func ValidationError(c *gin.Context, message string) {
	c.JSON(422, ErrorResponse{
		Status:  "error",
		Message: message,
	})
}

func BadRequest(c *gin.Context, message string) {
	Error(c, 400, message, nil)
}

func Unauthorized(c *gin.Context, message string) {
	Error(c, 401, message, nil)
}

func Forbidden(c *gin.Context, message string) {
	Error(c, 403, message, nil)
}

func NotFound(c *gin.Context, message string) {
	Error(c, 404, message, nil)
}