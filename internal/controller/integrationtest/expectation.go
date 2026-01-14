package integrationtest

import (
	infrav1alpha1 "github.com/lunz1207/testplane/api/v1alpha1"
	"github.com/lunz1207/testplane/internal/controller/shared"
)

// runExpectations 执行一组期望检查（委托给 shared.ExpectationRunner）。
func (r *IntegrationTestReconciler) runExpectations(expectations *infrav1alpha1.StepCondition, state map[string]interface{}) (shared.ExpectationResults, error) {
	runner := shared.NewExpectationRunner(r.PluginRegistry)
	return runner.RunStepCondition(expectations, state)
}
