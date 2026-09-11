package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/donktech/d-specs/internal/infrastructure/auth"
	"github.com/donktech/d-specs/internal/infrastructure/idgen"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// maxPasswordBytes bcrypt 只处理前 72 字节，超长会直接报错，这里提前拦掉
const maxPasswordBytes = 72

type Service struct {
	db  *gorm.DB
	jwt *auth.JWTManager
}

func NewService(db *gorm.DB, jwt *auth.JWTManager) *Service {
	return &Service{db: db, jwt: jwt}
}

func (s *Service) Register(ctx context.Context, req RegisterRequest) (*AuthResponse, error) {
	email := normalizeEmail(req.Email)
	if len(req.Password) > maxPasswordBytes {
		return nil, ErrWeakPassword
	}

	if s.db == nil {
		return nil, errors.New("数据库不可用")
	}

	var exists int64
	if err := s.db.WithContext(ctx).Model(&User{}).
		Where("lower(email) = ? AND deleted_at IS NULL", email).
		Count(&exists).Error; err != nil {
		return nil, err
	}
	if exists > 0 {
		return nil, ErrEmailTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &User{
		ID:           idgen.New(),
		Email:        email,
		Name:         strings.TrimSpace(req.Name),
		PasswordHash: string(hash),
	}
	if user.Name == "" {
		user.Name = defaultNameFromEmail(email)
	}

	if err := s.db.WithContext(ctx).Create(user).Error; err != nil {
		// 并发注册同一邮箱时唯一索引会兜住，转成友好错误
		if isUniqueViolation(err) {
			return nil, ErrEmailTaken
		}
		return nil, err
	}

	return s.issue(user)
}

func (s *Service) Login(ctx context.Context, req LoginRequest) (*AuthResponse, error) {
	if s.db == nil {
		return nil, errors.New("数据库不可用")
	}

	var user User
	err := s.db.WithContext(ctx).
		Where("lower(email) = ? AND deleted_at IS NULL", normalizeEmail(req.Email)).
		Take(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 邮箱不存在与密码错误返回同一个错误，避免账号枚举
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}

	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		return nil, ErrInvalidCredentials
	}

	return s.issue(&user)
}

func (s *Service) Me(ctx context.Context, userID string) (*UserProfile, error) {
	if s.db == nil {
		return nil, errors.New("数据库不可用")
	}

	var user User
	err := s.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", userID).
		Take(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	profile := user.Profile()
	return &profile, nil
}

func (s *Service) issue(user *User) (*AuthResponse, error) {
	signed, err := s.jwt.Sign(user.ID)
	if err != nil {
		return nil, err
	}
	return &AuthResponse{
		Token:     signed.Value,
		ExpiresAt: signed.ExpiresAt.Format(time.RFC3339),
		User:      user.Profile(),
	}, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func defaultNameFromEmail(email string) string {
	if idx := strings.Index(email, "@"); idx > 0 {
		return email[:idx]
	}
	return email
}

// isUniqueViolation 判断是否为唯一索引冲突（PostgreSQL 23505）
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "23505")
}
