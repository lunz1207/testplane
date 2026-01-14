package shared

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// EnsureFinalizer 确保 finalizer 已添加到对象
func EnsureFinalizer(ctx context.Context, c client.Client, obj client.Object, finalizer string) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(obj, finalizer) {
		return ctrl.Result{}, nil
	}

	controllerutil.AddFinalizer(obj, finalizer)
	if err := c.Update(ctx, obj); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: DefaultRequeue}, nil
}

// HandleDeletion 处理对象删除，移除 finalizer
func HandleDeletion(ctx context.Context, c client.Client, obj client.Object, finalizer string) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(obj, finalizer) {
		return ctrl.Result{}, nil
	}

	controllerutil.RemoveFinalizer(obj, finalizer)
	if err := c.Update(ctx, obj); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}
