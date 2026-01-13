package plugin

import "fmt"

// Result 函数执行结果（统一断言和提取）。
// 支持两种使用模式：
// 1. 断言模式：使用 Passed、Actual、Message
// 2. 提取模式：使用 Value 返回提取的值
type Result struct {
	// Passed 断言是否通过（断言模式）。
	Passed bool
	// Actual 实际值（断言模式）。
	Actual string
	// Message 结果消息（断言模式）。
	Message string
	// Value 提取的值（提取模式）。
	Value string
}

// Pass 创建成功结果。
func Pass() Result {
	return Result{Passed: true}
}

// Fail 创建失败结果。
func Fail(msg string) Result {
	return Result{Passed: false, Message: msg}
}

// Extract 创建提取结果。
func Extract(value string) Result {
	return Result{Passed: true, Value: value}
}

// WithActual 设置实际值。
func (r Result) WithActual(actual interface{}) Result {
	r.Actual = fmt.Sprintf("%v", actual)
	return r
}

// WithMessage 设置消息。
func (r Result) WithMessage(msg string) Result {
	r.Message = msg
	return r
}

// WithValue 设置提取值。
func (r Result) WithValue(value string) Result {
	r.Value = value
	return r
}
