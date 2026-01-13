package framework

import (
	"encoding/json"
	"fmt"
)

// DecodeSpecMap 解码 RawExtension 中的 spec 数据为 map
// 统一处理空值和 JSON 解析错误
func DecodeSpecMap(raw []byte) (map[string]interface{}, error) {
	if len(raw) == 0 {
		return make(map[string]interface{}), nil
	}

	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("invalid JSON in spec: %w", err)
	}

	return m, nil
}
