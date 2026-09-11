package response

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// fieldLabels 把结构体字段名映射成用户能看懂的中文名。
// 未登记的字段回退用字段名本身，不至于漏提示。
var fieldLabels = map[string]string{
	"Email":       "邮箱",
	"Password":    "密码",
	"Name":        "姓名",
	"Title":       "标题",
	"Description": "描述",
	"Industry":    "行业",
	"ItemIDs":     "参数项",
	"Value":       "参数值",
	"Keyword":     "关键词",
}

// BindError 把 Gin 的绑定/校验错误转成可直接展示给用户的中文提示。
//
// 默认的 err.Error() 会输出 Validator 的内部信息
// （如 "Field validation for 'Password' failed on the 'min' tag"），
// 既不可读也不该暴露给前端。
func BindError(c *gin.Context, err error) {
	var verrs validator.ValidationErrors
	if !asValidationErrors(err, &verrs) || len(verrs) == 0 {
		Error(c, http.StatusBadRequest, 400, "请求参数格式不正确")
		return
	}

	// 一次只提示第一个问题：表单类接口逐个改比一次性列一堆更好用
	fe := verrs[0]
	label := fieldLabel(fe.Field())

	var message string
	switch fe.Tag() {
	case "required":
		message = label + "不能为空"
	case "email":
		message = label + "格式不正确"
	case "min":
		message = fmt.Sprintf("%s长度不能少于 %s", label, fe.Param())
	case "max":
		message = fmt.Sprintf("%s长度不能超过 %s", label, fe.Param())
	case "len":
		message = fmt.Sprintf("%s长度必须为 %s", label, fe.Param())
	case "oneof":
		message = fmt.Sprintf("%s取值不合法，可选值：%s", label, fe.Param())
	case "gte":
		message = fmt.Sprintf("%s不能小于 %s", label, fe.Param())
	case "lte":
		message = fmt.Sprintf("%s不能大于 %s", label, fe.Param())
	case "url":
		message = label + "必须是合法的链接"
	case "numeric", "number":
		message = label + "必须是数字"
	default:
		message = label + "不合法"
	}

	Error(c, http.StatusBadRequest, 400, message)
}

func fieldLabel(field string) string {
	if label, ok := fieldLabels[field]; ok {
		return label
	}
	return field
}

// asValidationErrors 做类型断言，抽出来是为了避免在上层重复引入 validator 依赖
func asValidationErrors(err error, target *validator.ValidationErrors) bool {
	if verrs, ok := err.(validator.ValidationErrors); ok {
		*target = verrs
		return true
	}
	return false
}
