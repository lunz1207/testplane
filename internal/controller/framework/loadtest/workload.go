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

	infrav1alpha1 "github.com/lunz1207/testplane/api/v1alpha1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// applyWorkload 应用 workload 资源（创建或更新）。
func (r *LoadTestReconciler) applyWorkload(ctx context.Context, lt *infrav1alpha1.LoadTest) error {
	log := logf.FromContext(ctx)

	specs, err := r.expandResources(lt, &lt.Spec.Workload.Resources, lt.Status.InjectedValues)
	if err != nil {
		return fmt.Errorf("expand workload resources: %w", err)
	}

	if err := r.applyResources(ctx, lt, specs); err != nil {
		return fmt.Errorf("apply workload resources: %w", err)
	}

	log.Info("workload resources applied", "count", len(specs))
	return nil
}
