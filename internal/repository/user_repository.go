package repository

import (
	"github.com/AlmightyOggy/management-system/internal/domain"
	"gorm.io/gorm"
)

type UserRepository interface {
	BaseRepository[domain.User]
	FindByEmail(email string) (*domain.User, error)
	FindByUUID(uuid string) (*domain.User, error)
	FindAll() ([]domain.User, error)
}

type userRepository struct {
	BaseRepository[domain.User]
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{
		BaseRepository: NewBaseRepository[domain.User](db),
		db:             db,
	}
}

func (r *userRepository) FindByEmail(email string) (*domain.User, error) {
	var user domain.User
	err := r.db.Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindByUUID(uuid string) (*domain.User, error) {
	var user domain.User
	err := r.db.Where("uuid = ?", uuid).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindAll() ([]domain.User, error) {
	var users []domain.User
	err := r.db.Order("created_at desc").Find(&users).Error
	if err != nil {
		return nil, err
	}
	return users, nil
}