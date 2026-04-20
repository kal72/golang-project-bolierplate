package auth

import (
	"context"
	"fmt"
	"golang-project-boilerplate/internal/config"
	"golang-project-boilerplate/internal/model"
	"golang-project-boilerplate/internal/shared/errorhandler"

	"github.com/golang-jwt/jwt/v5"
)

type AuthUsecase struct {
	Config *config.Config
}

func NewAuthUsecase(config *config.Config) AuthUsecaseContract {
	return &AuthUsecase{Config: config}
}

func (uc *AuthUsecase) Verify(ctx context.Context, token string) (data *model.Auth, errData *model.ErrorData) {
	jwtToken, err := jwt.ParseWithClaims(token, data, func(jwtToken *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := jwtToken.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", jwtToken.Header["alg"])
		}
		return []byte(uc.Config.Jwt.Secret), nil
	})

	if err != nil || !jwtToken.Valid {
		errData = errorhandler.ErrorInvalidToken(err)
		return
	}

	return
}
