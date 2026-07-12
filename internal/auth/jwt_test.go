package auth

import (
	"testing"
)

func TestJWTGenerationAndVerification(t *testing.T) {
	userID := "user-123"
	email := "test@example.com"
	name := "Test User"
	roles := []string{"developer", "admin"}

	token, err := GenerateToken(userID, email, name, roles)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	if token == "" {
		t.Fatalf("Generated token is empty")
	}

	claims, err := VerifyToken(token)
	if err != nil {
		t.Fatalf("Failed to verify token: %v", err)
	}

	if claims.Subject != userID {
		t.Errorf("Expected subject %q, got %q", userID, claims.Subject)
	}

	if claims.Email != email {
		t.Errorf("Expected email %q, got %q", email, claims.Email)
	}

	if claims.Name != name {
		t.Errorf("Expected name %q, got %q", name, claims.Name)
	}

	if len(claims.Roles) != 2 || claims.Roles[0] != "developer" || claims.Roles[1] != "admin" {
		t.Errorf("Expected roles %v, got %v", roles, claims.Roles)
	}
}

func TestVerifyInvalidToken(t *testing.T) {
	_, err := VerifyToken("invalid-token-string")
	if err == nil {
		t.Error("Expected error when verifying invalid token, got nil")
	}
}
