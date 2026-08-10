package utils

import (
	"time"

	"github.com/AlmightyOggy/management-system/internal/config"
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID		uint	`json:"user_id"`
	UUID		string	`json:"uuid"`
	Email		string	`json:"email"`
	RoleID		uint	`json:"role_id"`
	IsAdmin		bool	`json:"is_admin"`
	IsViewer	bool	`json:"is_viewer"`
	jwt.RegisteredClaims
}

func GenerateToken(userID uint, uuid, email string, roleID uint) (string, error) {
	cfg := config.GetConfig()

	claims := Claims{
		UserID: 	userID,
		UUID: 		uuid,
		Email: 		email,
		RoleID: 	roleID,
		IsAdmin: 	roleID == 1,
		IsViewer: 	roleID == 3,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: 	jwt.NewNumericDate(time.Now().Add(cfg.JWT.ExpiresIn)),
			IssuedAt: 	jwt.NewNumericDate(time.Now()),
			ID: 		uuid,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.JWT.Secret))
}

func ParseToken(tokenString string) (*Claims, error) {
	cfg := config.GetConfig()

	token, err := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(token *jwt.Token) (interface{}, error) {

			if token.Method != jwt.SigningMethodHS256 {
				return nil, jwt.ErrTokenSignatureInvalid
			}

			return []byte(cfg.JWT.Secret), nil
		},
	)

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, jwt.ErrTokenInvalidClaims
	}

	return claims, nil
}