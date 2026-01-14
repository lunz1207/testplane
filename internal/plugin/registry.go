package plugin

import (
	"encoding/json"
	"fmt"
)

// Function 统一的函数签名（断言和提取）。
// resource: CR 完整数据（由框架获取）
// params: 用户定义的参数
// 返回 Result，业务方按需使用：
//   - 断言模式：使用 Passed、Actual、Message
//   - 提取模式：使用 Value 返回提取的值
type Function func(resource, params map[string]interface{}) Result

// Registry 函数注册表。
type Registry struct {
	functions map[string]Function
}

// NewRegistry 创建注册表。
func NewRegistry() *Registry {
	return &Registry{
		functions: make(map[string]Function),
	}
}

// Register 注册函数。
func (r *Registry) Register(name string, fn Function) {
	r.functions[name] = fn
}

// Call 调用函数。
func (r *Registry) Call(name string, resource map[string]interface{}, paramsJSON []byte) (Result, error) {
	fn, ok := r.functions[name]
	if !ok {
		return Fail(fmt.Sprintf("unknown function: %s", name)), fmt.Errorf("unknown function: %s", name)
	}

	params, err := parseParams(paramsJSON)
	if err != nil {
		return Fail(fmt.Sprintf("invalid params: %v", err)), err
	}

	return fn(resource, params), nil
}

// Has 检查函数是否存在。
func (r *Registry) Has(name string) bool {
	_, ok := r.functions[name]
	return ok
}

// Names 返回所有已注册的函数名称。
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.functions))
	for name := range r.functions {
		names = append(names, name)
	}
	return names
}

// parseParams 解析参数 JSON。
func parseParams(paramsJSON []byte) (map[string]interface{}, error) {
	if len(paramsJSON) == 0 {
		return nil, nil
	}

	var params map[string]interface{}
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	return params, nil
}
