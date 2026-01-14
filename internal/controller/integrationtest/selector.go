package integrationtest

import (
	"context"
	"fmt"
	"sort"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	infrav1alpha1 "github.com/lunz1207/testplane/api/v1alpha1"
)

// SelectorResult 保存选择器查找结果。
type SelectorResult struct {
	// Key 选择器标识（apiVersion/kind/name）。
	Key string
	// Resources 匹配到的所有资源。
	Resources []map[string]interface{}
	// Matched 第一个符合期望的资源（nil 表示未找到）。
	Matched map[string]interface{}
	// MatchedName 匹配资源的名称。
	MatchedName string
}

// getSelectorKey 生成 ResourceSelector 的唯一标识。
func getSelectorKey(sel infrav1alpha1.ResourceSelector) string {
	if sel.Name != "" {
		return fmt.Sprintf("%s/%s/%s", sel.APIVersion, sel.Kind, sel.Name)
	}
	// 对于 LabelSelector/AnnotationSelector，使用 Kind
	return fmt.Sprintf("%s/%s", sel.APIVersion, sel.Kind)
}

// listBySelector 按名称、标签或注解选择器查找资源。
// Name、LabelSelector 和 AnnotationSelector 互斥，只能指定其中一个。
func (r *IntegrationTestReconciler) listBySelector(
	ctx context.Context,
	tc *infrav1alpha1.IntegrationTest,
	sel infrav1alpha1.ResourceSelector,
) ([]map[string]interface{}, error) {
	log := logf.FromContext(ctx)

	// 命名空间
	ns := sel.Namespace
	if ns == "" {
		ns = tc.Namespace
	}

	// 验证互斥：Name、LabelSelector 和 AnnotationSelector 不能同时指定
	hasName := sel.Name != ""
	hasLabelSelector := len(sel.LabelSelector) > 0
	hasAnnotationSelector := len(sel.AnnotationSelector) > 0

	selectorCount := 0
	if hasName {
		selectorCount++
	}
	if hasLabelSelector {
		selectorCount++
	}
	if hasAnnotationSelector {
		selectorCount++
	}

	if selectorCount > 1 {
		return nil, fmt.Errorf("selector %s/%s: name, labelSelector and annotationSelector are mutually exclusive", sel.Kind, sel.Name)
	}

	if selectorCount == 0 {
		return nil, fmt.Errorf("selector %s: must specify one of name, labelSelector or annotationSelector", getSelectorKey(sel))
	}

	// 按名称查找
	if hasName {
		obj := &unstructured.Unstructured{}
		obj.SetAPIVersion(sel.APIVersion)
		obj.SetKind(sel.Kind)

		key := client.ObjectKey{Namespace: ns, Name: sel.Name}
		if err := r.Get(ctx, key, obj); err != nil {
			if client.IgnoreNotFound(err) == nil {
				log.Info("resource not found by name",
					"selector", getSelectorKey(sel),
					"kind", sel.Kind,
					"name", sel.Name)
				return []map[string]interface{}{}, nil
			}
			return nil, fmt.Errorf("get resource by name: %w", err)
		}

		log.Info("selector matched resource by name",
			"selector", getSelectorKey(sel),
			"kind", sel.Kind,
			"name", sel.Name)

		return []map[string]interface{}{obj.Object}, nil
	}

	// 按标签选择器查找
	if hasLabelSelector {
		list := &unstructured.UnstructuredList{}
		list.SetAPIVersion(sel.APIVersion)
		list.SetKind(sel.Kind)

		opts := []client.ListOption{
			client.InNamespace(ns),
			client.MatchingLabels(sel.LabelSelector),
		}

		if err := r.List(ctx, list, opts...); err != nil {
			return nil, fmt.Errorf("list resources: %w", err)
		}

		log.Info("selector matched resources by label",
			"selector", getSelectorKey(sel),
			"kind", sel.Kind,
			"count", len(list.Items))

		results := make([]map[string]interface{}, 0, len(list.Items))
		for _, item := range list.Items {
			results = append(results, item.Object)
		}

		return results, nil
	}

	// 按注解选择器查找
	list := &unstructured.UnstructuredList{}
	list.SetAPIVersion(sel.APIVersion)
	list.SetKind(sel.Kind)

	opts := []client.ListOption{client.InNamespace(ns)}

	if err := r.List(ctx, list, opts...); err != nil {
		return nil, fmt.Errorf("list resources: %w", err)
	}

	// 客户端过滤注解
	results := make([]map[string]interface{}, 0)
	for _, item := range list.Items {
		if matchAnnotations(item.GetAnnotations(), sel.AnnotationSelector) {
			results = append(results, item.Object)
		}
	}

	log.Info("selector matched resources by annotation",
		"selector", getSelectorKey(sel),
		"kind", sel.Kind,
		"count", len(results))

	return results, nil
}

// matchAnnotations 检查资源的注解是否包含所有指定的注解。
func matchAnnotations(annotations, selector map[string]string) bool {
	for key, value := range selector {
		if annotations[key] != value {
			return false
		}
	}
	return true
}

// findMatchingResource 遍历资源，找到第一个符合期望的。
// 资源列表会按名称排序，确保选择的确定性。
func (r *IntegrationTestReconciler) findMatchingResource(
	ctx context.Context,
	sel infrav1alpha1.ResourceSelector,
	resources []map[string]interface{},
	expectations []infrav1alpha1.Expectation,
) *SelectorResult {
	log := logf.FromContext(ctx)

	result := &SelectorResult{
		Key:       getSelectorKey(sel),
		Resources: resources,
	}

	if len(resources) == 0 {
		return result
	}

	// 按名称排序，确保选择的确定性
	sortResourcesByName(resources)

	if len(expectations) == 0 {
		// 没有期望，返回第一个资源（按名称排序后）
		result.Matched = resources[0]
		result.MatchedName = getResourceName(resources[0])
		return result
	}

	// 遍历资源，找到第一个符合所有期望的
	for _, res := range resources {
		name := getResourceName(res)
		allPassed := true

		for _, exp := range expectations {
			passed := r.runSingleExpectation(ctx, exp, res)
			if !passed {
				log.V(1).Info("expectation not passed", "resource", name, "expect", getExpectName(exp))
				allPassed = false
				break
			}
		}

		if allPassed {
			log.Info("found matching resource", "selector", getSelectorKey(sel), "resource", name)
			result.Matched = res
			result.MatchedName = name
			return result
		}
	}

	log.Info("no matching resource found", "selector", getSelectorKey(sel), "total", len(resources))
	return result
}

// sortResourcesByName 按资源名称排序。
func sortResourcesByName(resources []map[string]interface{}) {
	sort.Slice(resources, func(i, j int) bool {
		return getResourceName(resources[i]) < getResourceName(resources[j])
	})
}

// runSingleExpectation 执行单个期望检查。
// 支持声明式和函数式两种模式。
func (r *IntegrationTestReconciler) runSingleExpectation(
	ctx context.Context,
	exp infrav1alpha1.Expectation,
	res map[string]interface{},
) bool {
	log := logf.FromContext(ctx)

	// 使用 ExpectationRunner 统一处理
	condition := &infrav1alpha1.StepCondition{
		AllOf: []infrav1alpha1.Expectation{exp},
	}
	results, err := r.runExpectations(condition, res)
	if err != nil {
		log.V(1).Info("expectation error", "expect", getExpectName(exp), "error", err)
		return false
	}
	return results.Passed()
}

// getExpectName 获取期望的名称（用于日志）。
func getExpectName(exp infrav1alpha1.Expectation) string {
	return exp.Function
}

// gatherSelectorStates 收集所有选择器的状态。
func (r *IntegrationTestReconciler) gatherSelectorStates(
	ctx context.Context,
	tc *infrav1alpha1.IntegrationTest,
	selectors []infrav1alpha1.ResourceSelector,
	expectations []infrav1alpha1.Expectation,
) (map[string]*SelectorResult, error) {
	results := make(map[string]*SelectorResult)

	for _, sel := range selectors {
		resources, err := r.listBySelector(ctx, tc, sel)
		if err != nil {
			return nil, fmt.Errorf("selector %s: %w", getSelectorKey(sel), err)
		}

		result := r.findMatchingResource(ctx, sel, resources, expectations)
		results[getSelectorKey(sel)] = result
	}

	return results, nil
}

// getResourceName 从资源对象中获取名称。
func getResourceName(res map[string]interface{}) string {
	if meta, ok := res["metadata"].(map[string]interface{}); ok {
		if name, ok := meta["name"].(string); ok {
			return name
		}
	}
	return ""
}

// buildResourceKey 生成 apiVersion/kind/name 格式的 key。
func buildResourceKey(res map[string]interface{}) string {
	apiVersion, _ := res["apiVersion"].(string)
	kind, _ := res["kind"].(string)
	name := getResourceName(res)
	if apiVersion == "" || kind == "" || name == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s/%s", apiVersion, kind, name)
}

// selectorResultsToState 将 SelectorResult 转为期望检查可用的状态 map。
func selectorResultsToState(results map[string]*SelectorResult) map[string]interface{} {
	state := make(map[string]interface{})
	for _, result := range results {
		if result == nil || result.Matched == nil {
			continue
		}
		key := buildResourceKey(result.Matched)
		if key == "" {
			key = result.Key
		}
		state[key] = result.Matched
	}
	return state
}
