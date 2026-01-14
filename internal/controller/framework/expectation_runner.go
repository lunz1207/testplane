package framework

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	infrav1alpha1 "github.com/lunz1207/testplane/api/v1alpha1"
	"github.com/lunz1207/testplane/internal/controller/framework/plugin"
	"k8s.io/apimachinery/pkg/runtime"
)

// normalizeParams 确保 RawExtension 不为 null，为空时返回空对象 {}。
func normalizeParams(params runtime.RawExtension) runtime.RawExtension {
	if len(params.Raw) == 0 {
		return runtime.RawExtension{Raw: []byte("{}")}
	}
	return params
}

// ExpectationRunner 统一的期望执行器。
type ExpectationRunner struct {
	Registry   *plugin.Registry
	HTTPClient *http.Client
}

// NewExpectationRunner 创建期望执行器。
func NewExpectationRunner(registry *plugin.Registry) *ExpectationRunner {
	return &ExpectationRunner{
		Registry: registry,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ExpectationResults 包含 allOf 和 anyOf 的检查结果。
type ExpectationResults struct {
	AllOf []infrav1alpha1.ExpectationResult
	AnyOf []infrav1alpha1.ExpectationResult
}

// All 返回所有结果（用于状态记录）。
func (r ExpectationResults) All() []infrav1alpha1.ExpectationResult {
	results := make([]infrav1alpha1.ExpectationResult, 0, len(r.AllOf)+len(r.AnyOf))
	results = append(results, r.AllOf...)
	results = append(results, r.AnyOf...)
	return results
}

// Passed 检查期望是否满足：allOf 全部通过 && anyOf 任一通过（如果有）。
func (r ExpectationResults) Passed() bool {
	// allOf: 全部必须通过
	for _, result := range r.AllOf {
		if !result.Passed {
			return false
		}
	}

	// anyOf: 任一通过即可（如果没有 anyOf 则视为通过）
	if len(r.AnyOf) > 0 {
		anyPassed := false
		for _, result := range r.AnyOf {
			if result.Passed {
				anyPassed = true
				break
			}
		}
		if !anyPassed {
			return false
		}
	}

	return true
}

// RunStepCondition 执行 StepCondition 期望检查（用于 IntegrationTest）。
func (runner *ExpectationRunner) RunStepCondition(
	condition *infrav1alpha1.StepCondition,
	state map[string]interface{},
) (ExpectationResults, error) {
	if condition == nil {
		return ExpectationResults{}, nil
	}
	return runner.runExpectations(condition.AllOf, condition.AnyOf, state)
}

// RunReadyCondition 执行 ReadyCondition 期望检查（用于 LoadTest Target）。
func (runner *ExpectationRunner) RunReadyCondition(
	condition *infrav1alpha1.ReadyCondition,
	state map[string]interface{},
) (ExpectationResults, error) {
	if condition == nil {
		return ExpectationResults{}, nil
	}
	return runner.runExpectations(condition.AllOf, condition.AnyOf, state)
}

// RunHealthCheck 执行 HealthCheck 期望检查（用于 LoadTest 运行期）。
func (runner *ExpectationRunner) RunHealthCheck(
	healthCheck *infrav1alpha1.HealthCheck,
	state map[string]interface{},
) (ExpectationResults, error) {
	if healthCheck == nil {
		return ExpectationResults{}, nil
	}
	return runner.runExpectations(healthCheck.AllOf, healthCheck.AnyOf, state)
}

// runExpectations 执行期望检查（allOf + anyOf）。
func (runner *ExpectationRunner) runExpectations(
	allOf []infrav1alpha1.Expectation,
	anyOf []infrav1alpha1.Expectation,
	state map[string]interface{},
) (ExpectationResults, error) {
	var results ExpectationResults

	// 执行 allOf
	results.AllOf = make([]infrav1alpha1.ExpectationResult, 0, len(allOf))
	for _, exp := range allOf {
		result, err := runner.runExpectation(exp, state)
		if err != nil {
			return results, err
		}
		results.AllOf = append(results.AllOf, result)
	}

	// 执行 anyOf
	results.AnyOf = make([]infrav1alpha1.ExpectationResult, 0, len(anyOf))
	for _, exp := range anyOf {
		result, err := runner.runExpectation(exp, state)
		if err != nil {
			return results, err
		}
		results.AnyOf = append(results.AnyOf, result)
	}

	return results, nil
}

// runExpectation 执行单个期望检查。
// 支持两种模式：
// 1. 内置函数：Function + Params（可选）
// 2. Webhook：Function + Webhook + Params（可选）
// 断言的资源由调用方在 state 中提供。
func (runner *ExpectationRunner) runExpectation(
	exp infrav1alpha1.Expectation,
	state map[string]interface{},
) (infrav1alpha1.ExpectationResult, error) {
	// 有 Webhook → 调用外部服务
	if exp.Webhook != "" {
		return runner.runWebhook(exp)
	}

	// 无 Webhook → 调用内置函数
	payload := SelectStateForExpectation(state)

	return runner.runFunction(exp, payload)
}

// runFunction 执行内置函数断言。
func (runner *ExpectationRunner) runFunction(
	exp infrav1alpha1.Expectation,
	resource map[string]interface{},
) (infrav1alpha1.ExpectationResult, error) {
	result, err := runner.Registry.Call(exp.Function, resource, exp.Params.Raw)
	if err != nil {
		return infrav1alpha1.ExpectationResult{
			Expect:  exp.Function,
			Params:  normalizeParams(exp.Params),
			Passed:  false,
			Message: err.Error(),
		}, err
	}

	out := infrav1alpha1.ExpectationResult{
		Expect: exp.Function,
		Params: normalizeParams(exp.Params),
		Passed: result.Passed,
	}
	if !result.Passed {
		out.Actual = result.Actual
		out.Message = result.Message
	}

	return out, nil
}

// WebhookRequest Webhook 请求结构。
type WebhookRequest struct {
	Function string                 `json:"function"`
	Params   map[string]interface{} `json:"params,omitempty"`
}

// WebhookResponse Webhook 响应结构。
type WebhookResponse struct {
	Passed  bool   `json:"passed"`
	Actual  string `json:"actual,omitempty"`
	Message string `json:"message,omitempty"`
}

// runWebhook 调用 Webhook 执行断言。
// 请求格式：{ function, params }
// 响应格式：{ passed, actual, message }
func (runner *ExpectationRunner) runWebhook(
	exp infrav1alpha1.Expectation,
) (infrav1alpha1.ExpectationResult, error) {
	webhookURL := exp.Webhook

	// 解析参数
	var params map[string]interface{}
	if len(exp.Params.Raw) > 0 {
		if err := json.Unmarshal(exp.Params.Raw, &params); err != nil {
			return infrav1alpha1.ExpectationResult{
				Expect:  exp.Function,
				Params:  normalizeParams(exp.Params),
				Passed:  false,
				Message: fmt.Sprintf("invalid params: %v", err),
			}, err
		}
	}

	// 构建请求
	reqBody := WebhookRequest{
		Function: exp.Function,
		Params:   params,
	}
	reqData, err := json.Marshal(reqBody)
	if err != nil {
		return infrav1alpha1.ExpectationResult{
			Expect:  exp.Function,
			Params:  normalizeParams(exp.Params),
			Passed:  false,
			Message: fmt.Sprintf("marshal request failed: %v", err),
		}, err
	}

	// 调用 Webhook
	resp, err := runner.HTTPClient.Post(webhookURL, "application/json", bytes.NewReader(reqData))
	if err != nil {
		return infrav1alpha1.ExpectationResult{
			Expect:  exp.Function,
			Params:  normalizeParams(exp.Params),
			Passed:  false,
			Message: fmt.Sprintf("webhook call failed: %v", err),
		}, err
	}
	defer func() { _ = resp.Body.Close() }()

	// 读取响应
	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return infrav1alpha1.ExpectationResult{
			Expect:  exp.Function,
			Params:  normalizeParams(exp.Params),
			Passed:  false,
			Message: fmt.Sprintf("read response failed: %v", err),
		}, err
	}

	if resp.StatusCode != http.StatusOK {
		return infrav1alpha1.ExpectationResult{
			Expect:  exp.Function,
			Params:  normalizeParams(exp.Params),
			Passed:  false,
			Message: fmt.Sprintf("webhook returned status %d: %s", resp.StatusCode, string(respData)),
		}, fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	// 解析响应
	var webhookResp WebhookResponse
	if err := json.Unmarshal(respData, &webhookResp); err != nil {
		return infrav1alpha1.ExpectationResult{
			Expect:  exp.Function,
			Params:  normalizeParams(exp.Params),
			Passed:  false,
			Message: fmt.Sprintf("invalid webhook response: %v", err),
		}, err
	}

	return infrav1alpha1.ExpectationResult{
		Expect:  exp.Function,
		Params:  normalizeParams(exp.Params),
		Passed:  webhookResp.Passed,
		Actual:  webhookResp.Actual,
		Message: webhookResp.Message,
	}, nil
}

// SelectStateForExpectation 选择最适合期望使用的对象。
func SelectStateForExpectation(state map[string]interface{}) map[string]interface{} {
	if len(state) == 1 {
		return unwrapSingleState(state)
	}
	for _, v := range state {
		if m, ok := v.(map[string]interface{}); ok {
			if _, hasStatus := m["status"]; hasStatus {
				return m
			}
			if _, hasSpec := m["spec"]; hasSpec {
				return m
			}
		}
	}
	return state
}

// unwrapSingleState 将仅包含一个资源的状态下钻为资源对象本身。
func unwrapSingleState(state map[string]interface{}) map[string]interface{} {
	if len(state) == 1 {
		for _, v := range state {
			if m, ok := v.(map[string]interface{}); ok {
				return m
			}
		}
	}
	return state
}
