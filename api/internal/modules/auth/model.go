package auth

// UserProfile 透给前端的用户资料。
//
// 字段全部来自 d-auth 的脱敏视图（/v1/users/:id），Phone 是掩码形态
// （138****8000）。按产品要求不展示 email，这里没有它。
// 昵称/头像取不到时为空串——展示降级，不影响登录判定（授权用 JWT 的 sub）。
type UserProfile struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Phone     string `json:"phone,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
}
