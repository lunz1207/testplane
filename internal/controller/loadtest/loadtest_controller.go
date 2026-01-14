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
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	infrav1alpha1 "github.com/lunz1207/testplane/api/v1alpha1"
	"github.com/lunz1207/testplane/internal/controller/shared"
	"github.com/lunz1207/testplane/internal/controller/shared/logging"
	"github.com/lunz1207/testplane/internal/controller/shared/resource"
	"github.com/lunz1207/testplane/internal/plugin"
)

const (
	loadTestFinalizer  = "infra.testplane.io/loadtest-finalizer"
	loadTestFieldOwner = "loadtest-controller"

	defaultRequeue = 5 * time.Second
)

// LoadTestReconciler reconciles a LoadTest object.
type LoadTestReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	PluginRegistry  *plugin.Registry
	APIReader       client.Reader // 用于 waitResourcesConverge 绕过缓存检查收敛
	Recorder        record.EventRecorder
	ResourceManager *resource.Manager
}

// +kubebuilder:rbac:groups=infra.testplane.io,resources=loadtests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infra.testplane.io,resources=loadtests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infra.testplane.io,resources=loadtests/finalizers,verbs=update
// 需要操作任意资源用于负载测试。
// +kubebuilder:rbac:groups=*,resources=*,verbs=get;list;watch;create;update;patch;delete

func (r *LoadTestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	baseLog := logf.FromContext(ctx)

	var lt infrav1alpha1.LoadTest
	if err := r.Get(ctx, req.NamespacedName, &lt); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 添加资源上下文到 logger
	log := logging.WithKindName(baseLog, "LoadTest", lt.Namespace, lt.Name)
	ctx = logf.IntoContext(ctx, log)

	r.ensurePluginRegistry()
	r.ensureResourceManager()

	// 处理删除
	if !lt.DeletionTimestamp.IsZero() {
		return shared.HandleDeletion(ctx, r.Client, &lt, loadTestFinalizer)
	}

	// 添加 finalizer
	if !controllerutil.ContainsFinalizer(&lt, loadTestFinalizer) {
		return shared.EnsureFinalizer(ctx, r.Client, &lt, loadTestFinalizer)
	}

	res, err := r.reconcileNormal(ctx, &lt)
	if err != nil {
		log.Error(err, "reconcile failed")
	}
	return res, err
}

func (r *LoadTestReconciler) reconcileNormal(ctx context.Context, lt *infrav1alpha1.LoadTest) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// 初始化状态
	if lt.Status.Phase == "" {
		return r.initializeLoadTest(ctx, lt)
	}

	logging.Reconciling(log, string(lt.Status.Phase))

	// 检测 spec 变更（Generation 变化）
	if lt.Generation > lt.Status.ObservedGeneration {
		return r.handleSpecChange(ctx, lt)
	}

	// 根据阶段分发处理
	switch lt.Status.Phase {
	case infrav1alpha1.LoadTestPending:
		return r.reconcilePending(ctx, lt)
	case infrav1alpha1.LoadTestInitializing:
		return r.reconcileInitializing(ctx, lt)
	case infrav1alpha1.LoadTestRunning:
		return r.reconcileRunning(ctx, lt)
	case infrav1alpha1.LoadTestSucceeded, infrav1alpha1.LoadTestFailed:
		return r.reconcileTerminal(ctx, lt)
	}

	return ctrl.Result{}, nil
}

// handleSpecChange 处理 LoadTest spec 变更。
// - target 变更：使用 Server-Side Apply 更新
// - workload 变更：在 Running 阶段重新部署
func (r *LoadTestReconciler) handleSpecChange(ctx context.Context, lt *infrav1alpha1.LoadTest) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	logging.SpecChanged(log, lt.Generation, lt.Status.ObservedGeneration)

	// 1. 重新应用 target 资源（如有模板）
	// applyAndResolveTarget 内部会通过 hash 检查避免重复 apply
	if len(lt.Spec.Target.Resource.Manifest.Raw) > 0 {
		if _, err := r.applyAndResolveTarget(ctx, lt); err != nil {
			log.Error(err, "failed to apply target")
			return r.setFailed(ctx, lt, "TargetApplyFailed", err.Error())
		}
	}

	// 2. 只在 Running 阶段处理 workload 变更
	if lt.Status.Phase == infrav1alpha1.LoadTestRunning {
		// 重新解析 env injection
		if err := r.resolveAndUpdateEnvInjection(ctx, lt, "re-resolved values"); err != nil {
			return ctrl.Result{}, err
		}

		// 重新应用 workload
		log.Info("reapplying workload due to spec change")
		if err := r.applyWorkload(ctx, lt); err != nil {
			log.Error(err, "failed to reapply workload")
			return r.setFailed(ctx, lt, "WorkloadApplyFailed", err.Error())
		}
	}

	// 更新 ObservedGeneration
	lt.Status.ObservedGeneration = lt.Generation
	if err := shared.PatchStatusMerge(ctx, r.Client, lt); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{Requeue: true}, nil
}

// SetupWithManager wires the controller.
func (r *LoadTestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorderFor("loadtest")
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1alpha1.LoadTest{}).
		Named("loadtest").
		Complete(r)
}
