package framework

import "time"

// Requeue 时间常量
const (
	// DefaultRequeue 默认重新入队时间（5秒）。
	DefaultRequeue = 5 * time.Second
)

// 步骤状态常量
const (
	StateRunning   = "Running"
	StateSucceeded = "Succeeded"
	StateFailed    = "Failed"
	StatePassed    = "Passed"
	StatePending   = "Pending"
)

// 原因常量
const (
	ReasonSucceeded = "Succeeded"
	ReasonFailed    = "Failed"
	ReasonTimeout   = "Timeout"
)
