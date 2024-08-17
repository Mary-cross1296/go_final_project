package auth

import "github.com/golang-jwt/jwt/v4"

type Password struct {
	Password string `json:"password"`
}

type Token struct {
	Token string `json:"token"`
}

// Claims - структура для хранения данных токена
type Claims struct {
	jwt.StandardClaims
	PasswordHash string `json:"password_hash"`
}
