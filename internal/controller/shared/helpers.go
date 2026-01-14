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
	infrav1alpha1 "github.com/lunz1207/testplane/api/v1alpha1"
)

// ToExpectationResultSummary 将 ExpectationResult 转换为 ExpectationResultSummary。
func ToExpectationResultSummary(r *infrav1alpha1.ExpectationResult) infrav1alpha1.ExpectationResultSummary {
	msg := r.Message
	if len(msg) > 256 {
		msg = msg[:253] + "..."
	}
	return infrav1alpha1.ExpectationResultSummary{
		Expect:  r.Expect,
		Passed:  r.Passed,
		Actual:  r.Actual,
		Message: msg,
	}
}

// ToExpectationResultSummaries 将 ExpectationResult 切片转换为摘要切片。
func ToExpectationResultSummaries(results []infrav1alpha1.ExpectationResult) []infrav1alpha1.ExpectationResultSummary {
	if len(results) == 0 {
		return nil
	}
	summaries := make([]infrav1alpha1.ExpectationResultSummary, len(results))
	for i := range results {
		summaries[i] = ToExpectationResultSummary(&results[i])
	}
	return summaries
}
