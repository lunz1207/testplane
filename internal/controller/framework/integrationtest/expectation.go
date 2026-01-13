package integrationtest

import (
	infrav1alpha1 "github.com/lunz1207/testplane/api/v1alpha1"
	"github.com/lunz1207/testplane/internal/controller/framework"
)

// runExpectations 执行一组期望检查（委托给 framework.ExpectationRunner）。
func (r *IntegrationTestReconciler) runExpectations(expectations *infrav1alpha1.WaitCondition, state map[string]interface{}) (framework.ExpectationResults, error) {
	runner := framework.NewExpectationRunner(r.PluginRegistry)
	return runner.RunWaitCondition(expectations, state)
}
