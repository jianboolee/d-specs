// Package idgen 统一生成对外的实体 ID 与分享 token。
package idgen

import (
	"github.com/google/uuid"
	gonanoid "github.com/matoous/go-nanoid/v2"
)

const (
	// Alphabet 分享 token 用的字母表
	Alphabet = "0123456789abcdefghijklmnopqrstuvwxyz"

	// TokenLen 分享 token 长度，熵要足够高以防被枚举爬取
	TokenLen = 32
)

// New 生成业务实体 ID：UUID v7（RFC 9562）。
//
// 选 v7 而不是 v4：v7 的前 48 位是毫秒时间戳，天然按创建时间有序。
// 作为主键时 B-tree 的插入位置集中在右侧而不是随机打散，页分裂更少；
// 后 74 位随机则保证 ID 不可枚举。
//
// 注意 ID 是 VARCHAR(36)（带连字符的标准写法），迁移 202609110004 已经把
// 相关列从 32 位加宽过 —— 改这里时别忘记核对列宽。
func New() string {
	id, err := uuid.NewV7()
	if err != nil {
		// 只有在随机源不可用时才会失败，属于环境级故障，直接 panic 更快暴露
		panic("idgen: " + err.Error())
	}
	return id.String()
}

// Token 生成分享链接用的随机 token。
//
// 刻意不用 UUID v7：token 是「持有即可访问」的凭证，不是实体标识 ——
// 带上时间戳既泄露创建时间，又让前 48 位可预测，对凭证来说都是负分。
// 这里要的是纯粹的随机性。
func Token() string {
	token, err := gonanoid.Generate(Alphabet, TokenLen)
	if err != nil {
		panic("idgen: " + err.Error())
	}
	return token
}
