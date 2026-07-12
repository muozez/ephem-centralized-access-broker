package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	keyID      = "ephem-default-key-id"
	keyOnce    sync.Once
)

type Claims struct {
	Email string   `json:"email"`
	Name  string   `json:"name"`
	Roles []string `json:"roles"`
	jwt.RegisteredClaims
}

// GetPublicKey returns the RSA Public Key
func GetPublicKey() *rsa.PublicKey {
	ensureKeys()
	return publicKey
}

// GetKeyID returns the active Key ID
func GetKeyID() string {
	return keyID
}

func ensureKeys() {
	keyOnce.Do(func() {
		privPath := os.Getenv("RSA_PRIVATE_KEY_PATH")
		if privPath != "" {
			privBytes, err := os.ReadFile(privPath)
			if err != nil {
				panic(fmt.Sprintf("failed to read private key: %v", err))
			}
			block, _ := pem.Decode(privBytes)
			if block == nil {
				panic("failed to decode PEM block containing private key")
			}
			privKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
			if err != nil {
				// Try PKCS8 format
				pkcs8Key, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
				if err2 != nil {
					panic(fmt.Sprintf("failed to parse private key: %v (PKCS1) or %v (PKCS8)", err, err2))
				}
				var ok bool
				privKey, ok = pkcs8Key.(*rsa.PrivateKey)
				if !ok {
					panic("private key is not an RSA key")
				}
			}
			privateKey = privKey
			publicKey = &privateKey.PublicKey
			keyID = "ephem-loaded-key-id"
			return
		}

		// Fallback to dynamic generation for development
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			panic(fmt.Sprintf("failed to generate RSA key: %v", err))
		}
		privateKey = key
		publicKey = &key.PublicKey
	})
}

// GenerateToken creates a signed RS256 JWT token for a user
func GenerateToken(userID string, email string, name string, roles []string) (string, error) {
	ensureKeys()
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

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = keyID
	return token.SignedString(privateKey)
}

// VerifyToken validates the JWT token and returns its claims
func VerifyToken(tokenStr string) (*Claims, error) {
	ensureKeys()
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return publicKey, nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}
