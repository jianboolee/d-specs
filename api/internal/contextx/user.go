package contextx

import (
	"context"
	"errors"
)

var ErrNoUser = errors.New("no user in context")

type userIDKey struct{}

// WithUserID 将当前登录用户 ID 写入 context（由 Auth 中间件调用）
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey{}, userID)
}

// UserID 从 context 读取当前登录用户 ID
func UserID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(userIDKey{}).(string)
	return id, ok && id != ""
}

// RequireUserID 读取当前登录用户 ID，未登录返回 ErrNoUser
func RequireUserID(ctx context.Context) (string, error) {
	id, ok := UserID(ctx)
	if !ok {
		return "", ErrNoUser
	}
	return id, nil
}
