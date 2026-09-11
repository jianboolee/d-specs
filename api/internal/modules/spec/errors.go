package spec

import "errors"

var (
	// ErrTableNotFound 配置表不存在，或不属于当前用户
	// 两种情况合并返回同一个错误，避免通过错误信息探测他人表是否存在
	ErrTableNotFound = errors.New("配置表不存在")

	// ErrRevisionConflict 并发写入冲突：客户端的 base_revision 已过期
	ErrRevisionConflict = errors.New("配置表已被其他操作修改，请刷新后重试")

	// ErrRevisionNotFound 指定的版本不存在
	ErrRevisionNotFound = errors.New("指定的版本不存在")

	// ErrOpsRejected ops 校验未通过
	ErrOpsRejected = errors.New("变更未通过校验")

	// ErrNothingToApply 提交了空变更
	ErrNothingToApply = errors.New("没有需要应用的变更")

	// ErrTitleRequired 标题为空
	ErrTitleRequired = errors.New("配置表标题不能为空")
)
