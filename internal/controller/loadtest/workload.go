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

package loadtest

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	infrav1alpha1 "github.com/lunz1207/testplane/api/v1alpha1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const annotationInjectPrefix = "testplane.io/inject-"

// applyWorkload 应用 workload 资源（创建或更新）。
func (r *LoadTestReconciler) applyWorkload(ctx context.Context, lt *infrav1alpha1.LoadTest) error {
	log := logf.FromContext(ctx)

	specs, err := r.expandResources(lt, lt.Spec.Workload.Resources)
	if err != nil {
		return fmt.Errorf("expand workload resources: %w", err)
	}

	// 将提取的值注入到 Pod template annotations
	for i := range specs {
		if err := injectAnnotationsToWorkload(specs[i].Object, lt.Status.InjectedValues); err != nil {
			return fmt.Errorf("inject annotations to workload: %w", err)
		}
	}

	if err := r.applyResources(ctx, lt, specs); err != nil {
		return fmt.Errorf("apply workload resources: %w", err)
	}

	log.Info("workload resources applied", "count", len(specs))
	return nil
}

// injectAnnotationsToWorkload 将提取的值注入到 workload 资源的 Pod template annotations 中。
// 支持 Deployment、DaemonSet、StatefulSet、Job、Pod 等资源类型。
// 用户可通过 Downward API 引用这些 annotations 作为环境变量。
func injectAnnotationsToWorkload(obj *unstructured.Unstructured, values map[string]string) error {
	if len(values) == 0 {
		return nil
	}

	// 转换 values 为 annotations (TARGET_URL → testplane.io/inject-target-url)
	annotations := make(map[string]string, len(values))
	for name, value := range values {
		key := annotationInjectPrefix + toKebabCase(name)
		annotations[key] = value
	}

	// 根据资源类型确定 annotations 路径
	kind := obj.GetKind()
	var annotationPath []string

	switch kind {
	case "Deployment", "DaemonSet", "StatefulSet", "ReplicaSet":
		annotationPath = []string{"spec", "template", "metadata", "annotations"}
	case "Job":
		annotationPath = []string{"spec", "template", "metadata", "annotations"}
	case "CronJob":
		annotationPath = []string{"spec", "jobTemplate", "spec", "template", "metadata", "annotations"}
	case "Pod":
		annotationPath = []string{"metadata", "annotations"}
	default:
		// 尝试通用的 spec.template.metadata.annotations 路径
		annotationPath = []string{"spec", "template", "metadata", "annotations"}
	}

	// 获取现有 annotations
	existing, _, _ := unstructured.NestedStringMap(obj.Object, annotationPath...)
	if existing == nil {
		existing = make(map[string]string)
	}

	// 合并 annotations
	for k, v := range annotations {
		existing[k] = v
	}

	// 设置 annotations
	if err := unstructured.SetNestedStringMap(obj.Object, existing, annotationPath...); err != nil {
		return fmt.Errorf("set annotations at %v: %w", annotationPath, err)
	}

	return nil
}

// toKebabCase 将字符串转换为 kebab-case。
// 例如：TARGET_URL → target-url, MyVariable → my-variable
func toKebabCase(s string) string {
	// 替换下划线为连字符
	s = strings.ReplaceAll(s, "_", "-")

	// 在大写字母前插入连字符（驼峰转换）
	re := regexp.MustCompile("([a-z])([A-Z])")
	s = re.ReplaceAllString(s, "${1}-${2}")

	return strings.ToLower(s)
}
