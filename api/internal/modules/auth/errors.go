package auth

import "errors"

var (
	ErrEmailTaken         = errors.New("该邮箱已被注册")
	ErrInvalidCredentials = errors.New("邮箱或密码不正确")
	ErrUserNotFound       = errors.New("用户不存在")
	ErrWeakPassword       = errors.New("密码长度需为 8-72 位")
)
