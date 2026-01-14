package integrationtest

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1alpha1 "github.com/lunz1207/testplane/api/v1alpha1"
)

var _ = Describe("IntegrationTest Controller", func() {
	Context("When reconciling an IntegrationTest", func() {
		const resourceName = "integrationtest-sample"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		It("should reconcile without error", func() {
			scheme := runtime.NewScheme()
			Expect(infrav1alpha1.AddToScheme(scheme)).To(Succeed())

			it := &infrav1alpha1.IntegrationTest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: infrav1alpha1.IntegrationTestSpec{
					Mode:  infrav1alpha1.IntegrationTestModeSequential,
					Steps: []infrav1alpha1.TestStep{}, // 简化：无步骤
				},
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(it).
				Build()

			controllerReconciler := &IntegrationTestReconciler{
				Client:         fakeClient,
				Scheme:         scheme,
				PluginRegistry: nil, // Reconcile 中会初始化
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
