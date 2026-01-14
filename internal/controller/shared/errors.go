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

import (
	"fmt"
	"time"
)

// TransientError 临时错误，应该重试。
// 例如：资源未收敛、网络超时等。
type TransientError struct {
	Err          error
	RequeueAfter time.Duration
}

func (e *TransientError) Error() string {
	return fmt.Sprintf("transient error (requeue after %v): %v", e.RequeueAfter, e.Err)
}

func (e *TransientError) Unwrap() error {
	return e.Err
}

// NewTransientError 创建临时错误。
func NewTransientError(err error, requeueAfter time.Duration) *TransientError {
	return &TransientError{
		Err:          err,
		RequeueAfter: requeueAfter,
	}
}

// PermanentError 永久错误，不应重试，应标记失败。
// 例如：断言失败、manifest 无效等。
type PermanentError struct {
	Err    error
	Reason string
}

func (e *PermanentError) Error() string {
	return fmt.Sprintf("permanent error (%s): %v", e.Reason, e.Err)
}

func (e *PermanentError) Unwrap() error {
	return e.Err
}

// NewPermanentError 创建永久错误。
func NewPermanentError(err error, reason string) *PermanentError {
	return &PermanentError{
		Err:    err,
		Reason: reason,
	}
}

// 常见错误原因常量（补充 constants.go 中的定义）。
const (
	ReasonAssertionFailed  = "AssertionFailed"
	ReasonManifestInvalid  = "ManifestInvalid"
	ReasonResourceNotFound = "ResourceNotFound"
	ReasonWebhookFailed    = "WebhookFailed"
)

// 常见重试间隔常量。
const (
	ShortRequeueAfter = 2 * time.Second
	LongRequeueAfter  = 30 * time.Second
)
