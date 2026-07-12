package auth

import (
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var jwtSecretKey = []byte("ephem_jwt_secret_key_change_me_in_production")

func init() {
	if secret := os.Getenv("JWT_SECRET"); secret != "" {
		jwtSecretKey = []byte(secret)
	}
}

type Claims struct {
	Email string   `json:"email"`
	Name  string   `json:"name"`
	Roles []string `json:"roles"`
	jwt.RegisteredClaims
}

// GenerateToken creates a signed JWT token for a user
func GenerateToken(userID string, email string, name string, roles []string) (string, error) {
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		Email: email,
		Name:  name,
		Roles: roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    "ephem-api",
			Audience:  jwt.ClaimStrings{"ephem-cli"},
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecretKey)
}

// VerifyToken validates the JWT token and returns its claims
func VerifyToken(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return jwtSecretKey, nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}
