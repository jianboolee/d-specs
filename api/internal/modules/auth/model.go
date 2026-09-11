package auth

import "time"

// User 用户
type User struct {
	ID           string     `gorm:"column:id;primaryKey" json:"id"`
	Email        string     `gorm:"column:email" json:"email"`
	Name         string     `gorm:"column:name" json:"name"`
	PasswordHash string     `gorm:"column:password_hash" json:"-"`
	CreatedAt    time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt    *time.Time `gorm:"column:deleted_at" json:"-"`
}

func (User) TableName() string { return "users" }

// ==================== 请求 ====================

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email,max=255"`
	Password string `json:"password" binding:"required,min=8,max=72"`
	Name     string `json:"name" binding:"max=80"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email,max=255"`
	Password string `json:"password" binding:"required,max=72"`
}

// ==================== 响应 ====================

type UserProfile struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

func (u *User) Profile() UserProfile {
	return UserProfile{ID: u.ID, Email: u.Email, Name: u.Name}
}

type AuthResponse struct {
	Token     string      `json:"token"`
	ExpiresAt string      `json:"expires_at"`
	User      UserProfile `json:"user"`
}
