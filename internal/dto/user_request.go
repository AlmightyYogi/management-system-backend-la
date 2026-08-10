package dto

type UserRequest struct {
	ID          uint       `json:"id"`
	UUID        string     `json:"uuid"`
	Name        string     `json:"name"`
	Email       string     `json:"email"`
	Phone       string     `json:"phone"`
	Image       string     `json:"image"`
	RoleID      uint       `json:"role_id"`
	RoleName    string     `json:"role_name"`
	Active      bool       `json:"active"`
	LastLoginAt string     `json:"last_login_at"`
}

type UpdateUserRequest struct {
	Name     string  `form:"name" json:"name"`
	Email    string  `form:"email" json:"email"`
	Phone    string  `form:"phone" json:"phone"`
	Image    string  `form:"image" json:"image"`
	RoleID   uint    `form:"role_id" json:"role_id"`
	Active   *bool   `form:"active" json:"active"`
	Password string  `form:"password" json:"password,omitempty"`
}