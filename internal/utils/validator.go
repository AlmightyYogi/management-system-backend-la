package utils

import (
	"github.com/go-playground/validator/v10"
	"github.com/gin-gonic/gin"
)

var validate = validator.New()

func ValidateStruct(s interface{}) error {
	return validate.Struct(s)
}

func ValidateRequest(c *gin.Context, req interface{}) error {
	if err := c.ShouldBindJSON(req); err != nil {
		return err
	}
	return ValidateStruct(req)
}