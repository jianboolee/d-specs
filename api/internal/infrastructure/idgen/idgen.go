// Package idgen 统一生成对外可见的短 ID。
//
// 业务主键用 20 位 nanoid（小写字母 + 数字），比自增 ID 更难被枚举，
// 也便于在 URL 与分享链接中直接暴露。
package idgen

import gonanoid "github.com/matoous/go-nanoid/v2"

const (
	// Alphabet 去掉易混淆字符（o/l/i 保留但统一小写），降低人工抄写错误
	Alphabet = "0123456789abcdefghijklmnopqrstuvwxyz"

	DefaultLen = 20

	// TokenLen 分享 token 长度，熵要足够高以防被枚举爬取
	TokenLen = 32
)

// New 生成一个业务实体 ID
func New() string {
	id, err := gonanoid.Generate(Alphabet, DefaultLen)
	if err != nil {
		// 字母表固定且合法，这里不会失败；真失败说明程序有 bug，直接 panic 更快暴露
		panic("idgen: " + err.Error())
	}
	return id
}

// Token 生成分享链接用的随机 token
func Token() string {
	token, err := gonanoid.Generate(Alphabet, TokenLen)
	if err != nil {
		panic("idgen: " + err.Error())
	}
	return token
}
