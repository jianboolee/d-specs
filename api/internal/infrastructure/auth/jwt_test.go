package auth

import (
	"testing"
	"time"
)

func TestJWTManager_VerifyAllowExpired(t *testing.T) {
	m := NewJWTManager("test-secret", 2*time.Second)

	signed, err := m.Sign("user-1")
	if err != nil {
		t.Fatalf("Sign() err = %v", err)
	}

	claims, err := m.Verify(signed.Value)
	if err != nil {
		t.Fatalf("Verify() err = %v", err)
	}
	if claims.UserID != "user-1" {
		t.Fatalf("UserID = %q, want user-1", claims.UserID)
	}

	time.Sleep(3 * time.Second)

	if _, err := m.Verify(signed.Value); err == nil {
		t.Fatal("Verify() should fail on expired token")
	}

	claims, err = m.VerifyAllowExpired(signed.Value)
	if err != nil {
		t.Fatalf("VerifyAllowExpired() err = %v", err)
	}
	if claims.UserID != "user-1" {
		t.Fatalf("UserID = %q, want user-1", claims.UserID)
	}
}
