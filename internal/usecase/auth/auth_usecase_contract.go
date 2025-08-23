package auth

import (
	"context"
	"golang-project-boilerplate/internal/model"
)

type AuthUsecaseContract interface {
	Verify(ctx context.Context, token string) (data *model.Auth, errData *model.ErrorData)
}
