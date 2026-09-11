package contextx

import (
	"context"
	"errors"
	"testing"
)

func TestUserID(t *testing.T) {
	ctx := WithUserID(context.Background(), "abc123")

	id, ok := UserID(ctx)
	if !ok || id != "abc123" {
		t.Fatalf("UserID() = %q, %v; want abc123, true", id, ok)
	}

	id, err := RequireUserID(ctx)
	if err != nil || id != "abc123" {
		t.Fatalf("RequireUserID() = %q, %v", id, err)
	}

	_, ok = UserID(context.Background())
	if ok {
		t.Fatal("expected no user in background context")
	}

	_, err = RequireUserID(context.Background())
	if !errors.Is(err, ErrNoUser) {
		t.Fatalf("RequireUserID() err = %v, want ErrNoUser", err)
	}
}
