package utils

import "github.com/google/uuid"

func GenerateUUID() uuid.UUID {
	return uuid.New()
}

func GenerateUUIDString() string {
	return uuid.New().String()
}