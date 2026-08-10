package repository

import "gorm.io/gorm"

type BaseRepository[T any] interface {
	FindAll() ([]T, error)
	FindByID(id uint) (*T, error)
	FindByUUID(uuid string) (*T, error)
	Create(entity *T) error
	Update(entity *T) error
	Delete(id uint) error
	GetByID(id uint) (*T, error)
}

type baseRepository[T any] struct {
	db *gorm.DB
}

func NewBaseRepository[T any](db *gorm.DB) BaseRepository[T] {
	return &baseRepository[T]{db: db}
}

func (r *baseRepository[T]) FindAll() ([]T, error) {
	var entitites []T
	err := r.db.Find(&entitites).Error
	return entitites, err
}

func (r *baseRepository[T]) FindByID(id uint) (*T, error) {
	var entity T
	err := r.db.First(&entity, id).Error
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

func (r *baseRepository[T]) FindByUUID(uuid string) (*T, error) {
	var entity T
	err := r.db.Where("uuid = ?", uuid).First(&entity).Error
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

func (r *baseRepository[T]) Create(entity *T) error {
	return r.db.Create(entity).Error
}

func (r *baseRepository[T]) Update(entity *T) error {
	return r.db.Save(entity).Error
}

func (r *baseRepository[T]) Delete(id uint) error {
	var entity T
	return r.db.Delete(&entity, id).Error
}

func (r *baseRepository[T]) GetByID(id uint) (*T, error) {
	var model T
	err := r.db.First(&model, id).Error
	if err != nil {
		return nil, err
	}
	return &model, nil
}