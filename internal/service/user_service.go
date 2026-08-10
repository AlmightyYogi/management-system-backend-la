package service

import (
	"errors"
	"time"
	"log"

	"github.com/AlmightyOggy/management-system/internal/domain"
	"github.com/AlmightyOggy/management-system/internal/dto"
	"github.com/AlmightyOggy/management-system/internal/repository"
	"github.com/AlmightyOggy/management-system/internal/utils"
	"golang.org/x/crypto/bcrypt"
)

type UserService interface {
	Register(req dto.RegisterRequest) (*domain.User, error)
	Login(req dto.LoginRequest) (*domain.User, string, error)
	GetAllUsers() ([]dto.UserRequest, error)
	UpdateUser(uuid string, req dto.UpdateUserRequest) (*domain.User, error)
	GetUserByUUID(uuidStr string) (*domain.User, error)
}

type userService struct {
	userRepo repository.UserRepository
}

func NewUserService(userRepo repository.UserRepository) UserService {
	return &userService{userRepo: userRepo}
}

func (s *userService) Register(req dto.RegisterRequest) (*domain.User, error) {
	existing, _ := s.userRepo.FindByEmail(req.Email)
	if existing != nil {
		return nil, errors.New("email already registered")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &domain.User{
		UUID: 		utils.GenerateUUID(),
		RoleID: 	req.RoleID,
		Name: 		req.Name,
		Email: 		req.Email,
		Password: 	string(hashedPassword),
		Active: 	true,
	}

	err = s.userRepo.Create(user)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (s *userService) Login(req dto.LoginRequest) (*domain.User, string, error) {
	user, err := s.userRepo.FindByEmail(req.Email)
	if err != nil {
		return nil, "", errors.New("Invalid email or password, please try again")
	}

	if !user.Active {
		return nil, "", errors.New("Akun anda sudah ditangguhkan, hubungi admin untuk request buka akun")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, "", errors.New("Invalid email or password, please try again")
	}

	now := time.Now()
	user.LastLoginAt = &now

	if err := s.userRepo.Update(user); err != nil {
		log.Printf("Warning: Failed to update last_login_at: %v", err)
	}
	
	token, err := utils.GenerateToken(user.ID, user.UUID.String(), user.Email, user.RoleID)
	if err != nil {
		return nil, "", err
	}

	return user, token, nil
}

func (s *userService) GetAllUsers() ([]dto.UserRequest, error) {
	users, err := s.userRepo.FindAll()
	if err != nil {
		return nil, err
	}

	result := make([]dto.UserRequest, 0, len(users))
	for _, u := range users {
		lastLoginStr := ""
		if u.LastLoginAt != nil {
			lastLoginStr = u.LastLoginAt.Format("2006-01-02 15:04:05")
		}

		result = append(result, dto.UserRequest{
			ID:          u.ID,
			UUID:        u.UUID.String(),
			Name:        u.Name,
			Email:       u.Email,
			Phone:       u.Phone,
			Image:       u.Image,
			RoleID:      u.RoleID,
			RoleName:    roleNameFromID(u.RoleID),
			Active:      u.Active,
			LastLoginAt: lastLoginStr,
		})
	}
	return result, nil
}

func (s *userService) GetUserByUUID(uuidStr string) (*domain.User, error) {
	user, err := s.userRepo.FindByUUID(uuidStr)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *userService) UpdateUser(uuid string, req dto.UpdateUserRequest) (*domain.User, error) {
	user, err := s.userRepo.FindByUUID(uuid)
	if err != nil {
		return nil, errors.New("user not found")
	}

	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.Phone != "" {
		user.Phone = req.Phone
	}
	if req.Image != "" {
		user.Image = req.Image
	}
	if req.RoleID != 0 {
		user.RoleID = req.RoleID
	}
	if req.Active != nil {
		user.Active = *req.Active
	}
	if req.Password != "" {
		hashed, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		user.Password = string(hashed)
	}

	err = s.userRepo.Update(user)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func roleNameFromID(id uint) string {
	switch id {
	case 1:
		return "Administrator"
	case 2:
		return "IT Agent"
	case 3:
		return "End User"
	default:
		return "End User"
	}
}