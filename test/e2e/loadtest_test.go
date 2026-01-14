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

//nolint:goconst // test files use string literals for readability
package e2e

import (
	"time"

	. "github.com/onsi/ginkgo/v2" //nolint:revive,staticcheck
	. "github.com/onsi/gomega"    //nolint:revive,staticcheck
)

var _ = Describe("LoadTest Controller", Ordered, func() {
	const testNamespace = "default"

	// Cleanup after all tests
	AfterAll(func() {
		cleanupResources(testNamespace)
	})

	Context("Basic Lifecycle", func() {
		const testName = "e2e-loadtest-basic"
		const testFile = testDataDir + "/loadtest/basic_loadtest.yaml"

		AfterEach(func() {
			By("cleaning up test resources")
			_ = deleteYAML(testFile)
			waitForResourceNotExists("loadtest", testName, testNamespace, 1*time.Minute)
		})

		It("should transition through phases correctly", func() {
			By("creating the LoadTest CR")
			Expect(applyYAML(testFile)).To(Succeed(), "Failed to apply LoadTest")

			By("verifying the test starts and enters Initializing phase")
			waitForLoadTestPhaseIn(testName, testNamespace,
				[]string{"Pending", "Initializing", "Running"}, 30*time.Second)

			By("waiting for test to reach Running phase")
			waitForLoadTestPhase(testName, testNamespace, "Running", 3*time.Minute)

			By("verifying startTime is set")
			startTime, err := getLoadTestField(testName, testNamespace, ".status.startTime")
			Expect(err).NotTo(HaveOccurred())
			Expect(startTime).NotTo(BeEmpty(), "startTime should be set")

			By("verifying test is in Running phase")
			phase, err := getLoadTestPhase(testName, testNamespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(phase).To(Equal("Running"))
		})

		It("should create target resources", func() {
			By("creating the LoadTest CR")
			Expect(applyYAML(testFile)).To(Succeed())

			By("waiting for test to reach Running phase")
			waitForLoadTestPhase(testName, testNamespace, "Running", 3*time.Minute)

			By("verifying target deployment is created")
			waitForResourceExists("deployment", "e2e-loadtest-target", testNamespace, 1*time.Minute)
		})

		It("should create workload resources after target is ready", func() {
			By("creating the LoadTest CR")
			Expect(applyYAML(testFile)).To(Succeed())

			By("waiting for test to reach Running phase")
			waitForLoadTestPhase(testName, testNamespace, "Running", 3*time.Minute)

			By("verifying workload deployment is created")
			waitForResourceExists("deployment", "e2e-loadtest-workload", testNamespace, 1*time.Minute)
		})
	})

	Context("Environment Injection", func() {
		const testName = "e2e-loadtest-env-injection"
		const testFile = testDataDir + "/loadtest/env_injection.yaml"

		AfterEach(func() {
			_ = deleteYAML(testFile)
			waitForResourceNotExists("loadtest", testName, testNamespace, 1*time.Minute)
		})

		It("should inject environment variables into workload", func() {
			By("creating the LoadTest with envInjection")
			Expect(applyYAML(testFile)).To(Succeed())

			By("waiting for test to reach Running phase")
			waitForLoadTestPhase(testName, testNamespace, "Running", 3*time.Minute)

			By("verifying injectedValues are populated in status")
			Eventually(func(g Gomega) {
				values, err := getLoadTestField(testName, testNamespace, ".status.injectedValues")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(values).NotTo(BeEmpty(), "injectedValues should be populated")
			}, 1*time.Minute, 5*time.Second).Should(Succeed())

			By("verifying the specific injected variable exists")
			injectedValue, err := getLoadTestField(testName, testNamespace, ".status.injectedValues.TARGET_URL")
			Expect(err).NotTo(HaveOccurred())
			Expect(injectedValue).NotTo(BeEmpty(), "TARGET_URL should be injected")
		})
	})

	Context("Ready Condition", func() {
		const testName = "e2e-loadtest-ready-condition"
		const testFile = testDataDir + "/loadtest/ready_condition.yaml"

		AfterEach(func() {
			_ = deleteYAML(testFile)
			waitForResourceNotExists("loadtest", testName, testNamespace, 1*time.Minute)
		})

		It("should wait for target readyCondition before deploying workload", func() {
			By("creating LoadTest with readyCondition")
			Expect(applyYAML(testFile)).To(Succeed())

			By("verifying test enters Initializing phase first")
			waitForLoadTestPhase(testName, testNamespace, "Initializing", 30*time.Second)

			By("verifying readyConditionStatus is tracked")
			Eventually(func(g Gomega) {
				status, err := getLoadTestField(testName, testNamespace, ".status.readyConditionStatus")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(status).NotTo(BeEmpty())
			}, 1*time.Minute, 5*time.Second).Should(Succeed())

			By("waiting for test to reach Running phase")
			waitForLoadTestPhase(testName, testNamespace, "Running", 3*time.Minute)
		})
	})

	Context("Periodic Expectations", func() {
		const testName = "e2e-loadtest-expectations"
		const testFile = testDataDir + "/loadtest/expectations.yaml"

		AfterEach(func() {
			_ = deleteYAML(testFile)
			waitForResourceNotExists("loadtest", testName, testNamespace, 1*time.Minute)
		})

		It("should perform periodic health checks", func() {
			By("creating LoadTest with healthCheck")
			Expect(applyYAML(testFile)).To(Succeed())

			By("waiting for test to reach Running phase")
			waitForLoadTestPhase(testName, testNamespace, "Running", 3*time.Minute)

			By("verifying checkCount increases over time")
			Eventually(func(g Gomega) {
				count, err := getLoadTestField(testName, testNamespace, ".status.healthCheckStatus.checkCount")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(count).NotTo(Equal("0"), "checkCount should increase")
			}, 2*time.Minute, 10*time.Second).Should(Succeed())

			By("verifying passCount is tracked")
			passCount, err := getLoadTestField(testName, testNamespace, ".status.healthCheckStatus.passCount")
			Expect(err).NotTo(HaveOccurred())
			Expect(passCount).NotTo(BeEmpty())
		})

		It("should track lastCheckTime", func() {
			By("creating LoadTest with healthCheck")
			Expect(applyYAML(testFile)).To(Succeed())

			By("waiting for test to reach Running phase")
			waitForLoadTestPhase(testName, testNamespace, "Running", 3*time.Minute)

			By("verifying lastCheckTime is updated")
			Eventually(func(g Gomega) {
				lastCheck, err := getLoadTestField(testName, testNamespace, ".status.healthCheckStatus.lastCheckTime")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(lastCheck).NotTo(BeEmpty(), "lastCheckTime should be set")
			}, 2*time.Minute, 10*time.Second).Should(Succeed())
		})
	})

	Context("Failure Threshold", func() {
		const testName = "e2e-loadtest-failure-threshold"
		const testFile = testDataDir + "/loadtest/failure_threshold.yaml"

		AfterEach(func() {
			_ = deleteYAML(testFile)
			waitForResourceNotExists("loadtest", testName, testNamespace, 1*time.Minute)
		})

		It("should fail when consecutive failures exceed threshold", func() {
			By("creating LoadTest with failing healthCheck")
			Expect(applyYAML(testFile)).To(Succeed())

			By("waiting for test to reach Running phase first")
			waitForLoadTestPhase(testName, testNamespace, "Running", 3*time.Minute)

			By("waiting for test to fail due to healthCheck failures")
			waitForLoadTestPhase(testName, testNamespace, "Failed", 3*time.Minute)

			By("verifying consecutiveFailures reached threshold")
			failures, err := getLoadTestField(testName, testNamespace, ".status.healthCheckStatus.consecutiveFailures")
			Expect(err).NotTo(HaveOccurred())
			Expect(failures).NotTo(Equal("0"))

			By("verifying completionTime is set")
			completionTime, err := getLoadTestField(testName, testNamespace, ".status.completionTime")
			Expect(err).NotTo(HaveOccurred())
			Expect(completionTime).NotTo(BeEmpty())
		})
	})

	Context("Resource Cleanup", func() {
		const testName = "e2e-loadtest-cleanup"
		const testFile = testDataDir + "/loadtest/cleanup.yaml"
		const targetDeployment = "e2e-loadtest-cleanup-target"
		const workloadDeployment = "e2e-loadtest-cleanup-workload"

		It("should cleanup target and workload when CR is deleted", func() {
			By("creating the LoadTest")
			Expect(applyYAML(testFile)).To(Succeed())

			By("waiting for Running phase")
			waitForLoadTestPhase(testName, testNamespace, "Running", 3*time.Minute)

			By("verifying target deployment exists")
			waitForResourceExists("deployment", targetDeployment, testNamespace, 1*time.Minute)

			By("verifying workload deployment exists")
			waitForResourceExists("deployment", workloadDeployment, testNamespace, 1*time.Minute)

			By("deleting the LoadTest CR")
			Expect(deleteYAML(testFile)).To(Succeed())

			By("verifying LoadTest is deleted")
			waitForResourceNotExists("loadtest", testName, testNamespace, 1*time.Minute)

			By("verifying target deployment is cleaned up")
			waitForResourceNotExists("deployment", targetDeployment, testNamespace, 1*time.Minute)

			By("verifying workload deployment is cleaned up")
			waitForResourceNotExists("deployment", workloadDeployment, testNamespace, 1*time.Minute)
		})
	})

	Context("Conditions", func() {
		const testName = "e2e-loadtest-basic"
		const testFile = testDataDir + "/loadtest/basic_loadtest.yaml"

		AfterEach(func() {
			_ = deleteYAML(testFile)
			waitForResourceNotExists("loadtest", testName, testNamespace, 1*time.Minute)
		})

		It("should update conditions during lifecycle", func() {
			By("creating the LoadTest")
			Expect(applyYAML(testFile)).To(Succeed())

			By("waiting for Running phase")
			waitForLoadTestPhase(testName, testNamespace, "Running", 3*time.Minute)

			By("verifying conditions are set")
			conditions, err := getLoadTestField(testName, testNamespace, ".status.conditions")
			Expect(err).NotTo(HaveOccurred())
			Expect(conditions).NotTo(BeEmpty(), "Conditions should be set")

			By("verifying Ready condition is True")
			readyCondition, err := getLoadTestField(testName, testNamespace,
				".status.conditions[?(@.type==\"Ready\")].status")
			Expect(err).NotTo(HaveOccurred())
			Expect(readyCondition).To(Equal("True"))

			By("verifying TargetReady condition is True")
			targetReadyCondition, err := getLoadTestField(testName, testNamespace,
				".status.conditions[?(@.type==\"TargetReady\")].status")
			Expect(err).NotTo(HaveOccurred())
			Expect(targetReadyCondition).To(Equal("True"))
		})
	})

	Context("ObservedGeneration", func() {
		const testName = "e2e-loadtest-basic"
		const testFile = testDataDir + "/loadtest/basic_loadtest.yaml"

		AfterEach(func() {
			_ = deleteYAML(testFile)
			waitForResourceNotExists("loadtest", testName, testNamespace, 1*time.Minute)
		})

		It("should track observedGeneration", func() {
			By("creating the LoadTest")
			Expect(applyYAML(testFile)).To(Succeed())

			By("waiting for test to start")
			waitForLoadTestPhaseIn(testName, testNamespace,
				[]string{"Initializing", "Running"}, 1*time.Minute)

			By("verifying observedGeneration is set")
			Eventually(func(g Gomega) {
				obsGen, err := getLoadTestField(testName, testNamespace, ".status.observedGeneration")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(obsGen).NotTo(BeEmpty())
				g.Expect(obsGen).NotTo(Equal("0"))
			}, 30*time.Second, 2*time.Second).Should(Succeed())
		})
	})

	Context("Phase Transitions", func() {
		const testName = "e2e-loadtest-basic"
		const testFile = testDataDir + "/loadtest/basic_loadtest.yaml"

		AfterEach(func() {
			_ = deleteYAML(testFile)
			waitForResourceNotExists("loadtest", testName, testNamespace, 1*time.Minute)
		})

		It("should transition Pending -> Initializing -> Running", func() {
			By("creating the LoadTest")
			Expect(applyYAML(testFile)).To(Succeed())

			By("waiting for Initializing or Running phase")
			phase := waitForLoadTestPhaseIn(testName, testNamespace,
				[]string{"Initializing", "Running"}, 1*time.Minute)

			if phase == "Initializing" {
				By("waiting for Running phase after Initializing")
				waitForLoadTestPhase(testName, testNamespace, "Running", 3*time.Minute)
			}

			By("verifying final phase is Running")
			finalPhase, err := getLoadTestPhase(testName, testNamespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(finalPhase).To(Equal("Running"))
		})
	})

	Context("Events", func() {
		const testName = "e2e-loadtest-basic"
		const testFile = testDataDir + "/loadtest/basic_loadtest.yaml"

		AfterEach(func() {
			_ = deleteYAML(testFile)
			waitForResourceNotExists("loadtest", testName, testNamespace, 1*time.Minute)
		})

		It("should emit events during test lifecycle", func() {
			By("creating the LoadTest")
			Expect(applyYAML(testFile)).To(Succeed())

			By("waiting for Running phase")
			waitForLoadTestPhase(testName, testNamespace, "Running", 3*time.Minute)

			By("verifying events were emitted")
			Eventually(func(g Gomega) {
				events := getEvents(testNamespace)
				g.Expect(events).To(Or(
					ContainSubstring("LoadTest"),
					ContainSubstring(testName),
				), "Events should contain LoadTest references")
			}, 30*time.Second, 2*time.Second).Should(Succeed())
		})
	})

	Context("Target Selector", func() {
		const testName = "e2e-loadtest-selector"
		const testFile = testDataDir + "/loadtest/target_selector.yaml"
		const targetDeployment = "e2e-lt-selector-target"
		const workloadPod = "e2e-lt-selector-workload"

		AfterEach(func() {
			// Delete resources without waiting - cleanup will handle remaining
			_ = deleteYAML(testFile)
			deleteResource("pod", workloadPod, testNamespace)
			waitForResourceNotExists("loadtest", testName, testNamespace, 1*time.Minute)
			waitForResourceNotExists("deployment", targetDeployment, testNamespace, 1*time.Minute)
			// Pod deletion is async, don't block on it
		})

		It("should use existing resource as target via selector", func() {
			By("creating LoadTest with target selector")
			Expect(applyYAML(testFile)).To(Succeed())

			By("waiting for test to reach Running phase")
			waitForLoadTestPhase(testName, testNamespace, "Running", 3*time.Minute)

			By("verifying target was resolved from selector")
			// The pre-existing deployment should be used as target
			waitForResourceExists("deployment", targetDeployment, testNamespace, 30*time.Second)

			By("verifying workload pod is created")
			waitForResourceExists("pod", workloadPod, testNamespace, 1*time.Minute)

			By("verifying injectedValues contains TARGET_NAME")
			Eventually(func(g Gomega) {
				targetName, err := getLoadTestField(testName, testNamespace, ".status.injectedValues.TARGET_NAME")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(targetName).To(Equal(targetDeployment), "TARGET_NAME should be injected from target")
			}, 1*time.Minute, 5*time.Second).Should(Succeed())
		})
	})
})
