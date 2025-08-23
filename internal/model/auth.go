package model

import "github.com/golang-jwt/jwt/v5"

type Auth struct {
	ID   int
	Role int
	jwt.RegisteredClaims
}
