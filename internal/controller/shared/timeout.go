/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package shared

import "time"

// 默认超时常量
const (
	// DefaultStepTimeout 默认步骤超时（10 分钟）
	DefaultStepTimeout = 10 * time.Minute
	// DefaultWaitConditionTimeout 默认等待条件超时（5 分钟）
	DefaultWaitConditionTimeout = 5 * time.Minute
	// DefaultExpectationTimeout 默认断言超时（5 分钟）
	DefaultExpectationTimeout = 5 * time.Minute
	// DefaultReadyConditionTimeout 默认就绪条件超时（5 分钟）
	DefaultReadyConditionTimeout = 5 * time.Minute
)

// GetTimeoutDuration 从 int32 秒获取 Duration，如果为 0 或负数返回默认值。
func GetTimeoutDuration(seconds int32, defaultDuration time.Duration) time.Duration {
	if seconds <= 0 {
		return defaultDuration
	}
	return time.Duration(seconds) * time.Second
}

// CalculateDeadline 计算截止时间。
func CalculateDeadline(start time.Time, timeout time.Duration) time.Time {
	return start.Add(timeout)
}
