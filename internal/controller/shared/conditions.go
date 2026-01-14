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

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// SetCondition 设置或更新 Condition。
// 如果状态未变，只更新 Reason/Message 而不更新 LastTransitionTime。
func SetCondition(conditions *[]metav1.Condition, conditionType string,
	status metav1.ConditionStatus, reason, message string, generation int64) {

	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		ObservedGeneration: generation,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}

	found := false
	for i := range *conditions {
		if (*conditions)[i].Type == conditionType {
			// 只有状态变化时才更新 LastTransitionTime
			if (*conditions)[i].Status != status {
				(*conditions)[i] = condition
			} else {
				// 状态未变，只更新 Reason 和 Message
				(*conditions)[i].Reason = reason
				(*conditions)[i].Message = message
				(*conditions)[i].ObservedGeneration = generation
			}
			found = true
			break
		}
	}

	if !found {
		*conditions = append(*conditions, condition)
	}
}

// RemoveCondition 移除指定类型的 Condition。
func RemoveCondition(conditions *[]metav1.Condition, conditionType string) {
	for i := range *conditions {
		if (*conditions)[i].Type == conditionType {
			*conditions = append((*conditions)[:i], (*conditions)[i+1:]...)
			return
		}
	}
}

// GetCondition 获取指定类型的 Condition。
func GetCondition(conditions []metav1.Condition, conditionType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

// IsConditionTrue 检查 Condition 是否为 True。
func IsConditionTrue(conditions []metav1.Condition, conditionType string) bool {
	c := GetCondition(conditions, conditionType)
	return c != nil && c.Status == metav1.ConditionTrue
}
