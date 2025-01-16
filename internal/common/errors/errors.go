package errors

import "fmt"

// 定义错误类型
type Error struct {
	Code    int
	Message string
}

func (e *Error) Error() string {
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// 预定义错误
var (
	ErrInvalidConfig = &Error{1001, "无效的配置"}
	ErrInvalidInput  = &Error{1002, "无效的输入"}
	ErrAPIRequest    = &Error{2001, "API请求失败"}
)
