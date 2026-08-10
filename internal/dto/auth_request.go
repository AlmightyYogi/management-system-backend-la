package dto

type LoginRequest struct {
	Email		string	`json:"email" validate:"required,email"`
	Password	string	`json:"password" validate:"required,min=6"`
	LastLoginAt string  `json:"last_login_at"`
}

type RegisterRequest struct {
	Name     string  `form:"name" json:"name"`
	Email    string  `form:"email" json:"email"`
	Phone    string  `form:"phone" json:"phone"`
	Image    string  `form:"image" json:"image"`
	RoleID   uint    `form:"role_id" json:"role_id"`
	Active   *bool   `form:"active" json:"active"`
	Password string  `form:"password" json:"password,omitempty"`
}