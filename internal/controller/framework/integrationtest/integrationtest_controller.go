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

package integrationtest

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
	"github.com/lunz1207/testplane/internal/controller/framework"
	"github.com/lunz1207/testplane/internal/controller/framework/plugin"
	"github.com/lunz1207/testplane/internal/controller/framework/resource"
)

const (
	integrationTestFinalizer  = "infra.testplane.io/integrationtest-finalizer"
	integrationTestFieldOwner = "integrationtest-controller"

	defaultStepTimeout = 10 * time.Minute
	defaultRequeue     = 5 * time.Second
)

// IntegrationTestReconciler reconciles an IntegrationTest object.
type IntegrationTestReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	PluginRegistry  *plugin.Registry
	APIReader       client.Reader        // 用于 waitResourcesConverge 绕过缓存检查收敛状态
	Recorder        record.EventRecorder // 事件记录器
	ResourceManager *resource.Manager    // 资源管理器
}

// +kubebuilder:rbac:groups=infra.testplane.io,resources=integrationtests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infra.testplane.io,resources=integrationtests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infra.testplane.io,resources=integrationtests/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// 需要操作任意资源用于测试。
// +kubebuilder:rbac:groups=*,resources=*,verbs=get;list;watch;create;update;patch;delete

func (r *IntegrationTestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var it infrav1alpha1.IntegrationTest
	if err := r.Get(ctx, req.NamespacedName, &it); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	r.ensureRegistry()
	r.ensureResourceManager()

	if !it.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, &it)
	}

	if !controllerutil.ContainsFinalizer(&it, integrationTestFinalizer) {
		return framework.EnsureFinalizer(ctx, r.Client, &it, integrationTestFinalizer)
	}

	res, err := r.reconcileNormal(ctx, &it)
	if err != nil {
		log.Error(err, "reconcile failed")
	}
	return res, err
}

// handleDeletion 处理删除：移除 finalizer。
// 资源通过 ownerRef 关联，GC 会自动清理。
func (r *IntegrationTestReconciler) handleDeletion(ctx context.Context, it *infrav1alpha1.IntegrationTest) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(it, integrationTestFinalizer) {
		return ctrl.Result{}, nil
	}

	// 移除 finalizer，资源通过 ownerRef 由 GC 自动清理
	controllerutil.RemoveFinalizer(it, integrationTestFinalizer)
	if err := r.Update(ctx, it); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("integrationtest deleted successfully")
	return ctrl.Result{}, nil
}

func (r *IntegrationTestReconciler) reconcileNormal(ctx context.Context, it *infrav1alpha1.IntegrationTest) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// 初始化状态（如需要）
	if it.Status.Phase == "" {
		return r.initializeTest(ctx, it)
	}

	log.Info("reconcile integrationtest", "phase", it.Status.Phase, "mode", it.Spec.Mode)

	if isTerminalPhase(it.Status.Phase) {
		return ctrl.Result{}, nil
	}

	// 检测运行中的 spec 变更并忽略
	if r.detectAndIgnoreSpecChange(ctx, it) {
		log.Info("spec changed while running, ignoring", "observedGeneration", it.Status.ObservedGeneration, "currentGeneration", it.Generation)
		if err := r.patchStatus(ctx, it, it.Status); err != nil {
			return ctrl.Result{}, err
		}
	}

	// 执行测试逻辑（子函数负责各自的状态持久化）
	return r.executeTest(ctx, it)
}

// SetupWithManager wires the controller.
func (r *IntegrationTestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorderFor("integrationtest")
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1alpha1.IntegrationTest{}).
		Named("integrationtest").
		Complete(r)
}

func (r *IntegrationTestReconciler) ensureRegistry() {
	if r.PluginRegistry == nil {
		r.PluginRegistry = plugin.DefaultRegistry
	}
}

func (r *IntegrationTestReconciler) ensureResourceManager() {
	if r.ResourceManager == nil {
		r.ResourceManager = resource.NewManager(r.Client, r.Scheme, integrationTestFieldOwner, r.APIReader)
	}
}
