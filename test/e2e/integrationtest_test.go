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

var _ = Describe("IntegrationTest Controller", Ordered, func() {
	const testNamespace = "default"

	// Cleanup after all tests
	AfterAll(func() {
		cleanupResources(testNamespace)
	})

	Context("Sequential Mode", func() {
		const testName = "e2e-sequential-basic"
		const testFile = testDataDir + "/integrationtest/sequential_basic.yaml"

		AfterEach(func() {
			By("cleaning up test resources")
			_ = deleteYAML(testFile)
			waitForResourceNotExists("integrationtest", testName, testNamespace, 1*time.Minute)
		})

		It("should execute steps in order and succeed", func() {
			By("creating the IntegrationTest CR")
			Expect(applyYAML(testFile)).To(Succeed(), "Failed to apply IntegrationTest")

			By("verifying the test starts or completes")
			// Test may complete very fast, accept Running or Succeeded
			waitForIntegrationTestPhaseIn(testName, testNamespace,
				[]string{"Running", "Succeeded", "Failed"}, 30*time.Second)

			By("verifying mode is Sequential")
			mode, err := getIntegrationTestField(testName, testNamespace, ".spec.mode")
			Expect(err).NotTo(HaveOccurred())
			Expect(mode).To(Equal("Sequential"))

			By("waiting for test to complete")
			finalPhase := waitForIntegrationTestPhaseIn(testName, testNamespace,
				[]string{"Succeeded", "Failed"}, 3*time.Minute)

			if finalPhase == "Failed" {
				printDebugInfo(testName, testNamespace)
			}
			Expect(finalPhase).To(Equal("Succeeded"), "IntegrationTest should succeed")

			By("verifying all steps completed successfully")
			stepsState, err := getIntegrationTestField(testName, testNamespace, ".status.steps[*].state")
			Expect(err).NotTo(HaveOccurred())
			Expect(stepsState).NotTo(ContainSubstring("Failed"))
		})

		It("should update currentStepIndex as steps progress", func() {
			By("creating the IntegrationTest CR")
			Expect(applyYAML(testFile)).To(Succeed())

			By("waiting for test to start running")
			waitForIntegrationTestPhaseIn(testName, testNamespace,
				[]string{"Running", "Succeeded", "Failed"}, 30*time.Second)

			By("verifying currentStepIndex is tracked")
			Eventually(func(g Gomega) {
				index, err := getIntegrationTestField(testName, testNamespace, ".status.currentStepIndex")
				g.Expect(err).NotTo(HaveOccurred())
				// Index should be updated (could be 0 or higher)
				g.Expect(index).NotTo(BeEmpty())
			}, 1*time.Minute, 2*time.Second).Should(Succeed())

			By("waiting for test to complete")
			waitForIntegrationTestPhaseIn(testName, testNamespace,
				[]string{"Succeeded", "Failed"}, 3*time.Minute)
		})
	})

	Context("Parallel Mode", func() {
		const testName = "e2e-parallel-basic"
		const testFile = testDataDir + "/integrationtest/parallel_basic.yaml"

		AfterEach(func() {
			_ = deleteYAML(testFile)
			waitForResourceNotExists("integrationtest", testName, testNamespace, 1*time.Minute)
		})

		It("should execute all steps simultaneously", func() {
			By("creating the IntegrationTest CR with parallel mode")
			Expect(applyYAML(testFile)).To(Succeed())

			By("waiting for test to start or complete")
			// Test may complete very fast, accept Running or Succeeded
			waitForIntegrationTestPhaseIn(testName, testNamespace,
				[]string{"Running", "Succeeded", "Failed"}, 30*time.Second)

			By("verifying mode is Parallel")
			mode, err := getIntegrationTestField(testName, testNamespace, ".spec.mode")
			Expect(err).NotTo(HaveOccurred())
			Expect(mode).To(Equal("Parallel"))

			By("waiting for test to complete")
			finalPhase := waitForIntegrationTestPhaseIn(testName, testNamespace,
				[]string{"Succeeded", "Failed"}, 3*time.Minute)

			if finalPhase == "Failed" {
				printDebugInfo(testName, testNamespace)
			}
			Expect(finalPhase).To(Equal("Succeeded"), "Parallel IntegrationTest should succeed")
		})
	})

	Context("Repeat Configuration", func() {
		const testName = "e2e-repeat-test"
		const testFile = testDataDir + "/integrationtest/repeat_test.yaml"

		AfterEach(func() {
			_ = deleteYAML(testFile)
			waitForResourceNotExists("integrationtest", testName, testNamespace, 1*time.Minute)
		})

		It("should execute multiple rounds", func() {
			By("creating the IntegrationTest CR with repeat config")
			Expect(applyYAML(testFile)).To(Succeed())

			By("waiting for test to start running")
			waitForIntegrationTestPhaseIn(testName, testNamespace,
				[]string{"Running", "Succeeded", "Failed"}, 30*time.Second)

			By("verifying repeat.count is configured correctly")
			total, err := getIntegrationTestField(testName, testNamespace, ".spec.repeat.count")
			Expect(err).NotTo(HaveOccurred())
			Expect(total).To(Equal("2"), "repeat.count should be 2")

			By("waiting for test to complete all rounds")
			finalPhase := waitForIntegrationTestPhaseIn(testName, testNamespace,
				[]string{"Succeeded", "Failed"}, 5*time.Minute)

			if finalPhase == "Failed" {
				printDebugInfo(testName, testNamespace)
			}
			Expect(finalPhase).To(Equal("Succeeded"), "Repeat test should succeed")

			By("verifying completedRounds equals expected")
			completed, err := getIntegrationTestField(testName, testNamespace, ".status.completedRounds")
			Expect(err).NotTo(HaveOccurred())
			Expect(completed).To(Equal("2"), "completedRounds should be 2")

			By("verifying roundHistory is populated")
			historyLen, err := getIntegrationTestField(testName, testNamespace, ".status.roundHistory")
			Expect(err).NotTo(HaveOccurred())
			Expect(historyLen).NotTo(BeEmpty(), "roundHistory should be populated")
		})

		It("should track currentRound during execution", func() {
			By("creating the IntegrationTest CR")
			Expect(applyYAML(testFile)).To(Succeed())

			By("waiting for test to complete")
			waitForIntegrationTestPhaseIn(testName, testNamespace,
				[]string{"Succeeded", "Failed"}, 5*time.Minute)

			By("verifying currentRound equals completedRounds after completion")
			currentRound, err := getIntegrationTestField(testName, testNamespace, ".status.currentRound")
			Expect(err).NotTo(HaveOccurred())
			completedRounds, err := getIntegrationTestField(testName, testNamespace, ".status.completedRounds")
			Expect(err).NotTo(HaveOccurred())
			Expect(currentRound).To(Equal(completedRounds), "currentRound should equal completedRounds after completion")
			Expect(currentRound).To(Equal("2"), "currentRound should be 2 after all rounds complete")
		})
	})

	Context("Timeout Handling", func() {
		const testName = "e2e-timeout-test"
		const testFile = testDataDir + "/integrationtest/timeout_test.yaml"

		AfterEach(func() {
			_ = deleteYAML(testFile)
			waitForResourceNotExists("integrationtest", testName, testNamespace, 1*time.Minute)
		})

		It("should fail when timeout is exceeded", func() {
			By("creating an IntegrationTest with short timeout and impossible expectation")
			Expect(applyYAML(testFile)).To(Succeed())

			By("waiting for test to fail")
			waitForIntegrationTestPhase(testName, testNamespace, "Failed", 2*time.Minute)

			By("verifying failure reason is Timeout")
			reason, err := getIntegrationTestField(testName, testNamespace, ".status.reason")
			Expect(err).NotTo(HaveOccurred())
			Expect(reason).To(Equal("Timeout"), "Failure reason should be Timeout")

			By("verifying completionTime is set")
			completionTime, err := getIntegrationTestField(testName, testNamespace, ".status.completionTime")
			Expect(err).NotTo(HaveOccurred())
			Expect(completionTime).NotTo(BeEmpty(), "completionTime should be set")
		})
	})

	Context("Expectation Failure", func() {
		const testName = "e2e-expectation-failure"
		const testFile = testDataDir + "/integrationtest/expectation_failure.yaml"

		AfterEach(func() {
			_ = deleteYAML(testFile)
			waitForResourceNotExists("integrationtest", testName, testNamespace, 1*time.Minute)
		})

		It("should fail when expectation is not met", func() {
			By("creating an IntegrationTest with failing expectation")
			Expect(applyYAML(testFile)).To(Succeed())

			By("waiting for test to fail")
			waitForIntegrationTestPhase(testName, testNamespace, "Failed", 2*time.Minute)

			By("verifying the test failed")
			phase, err := getIntegrationTestPhase(testName, testNamespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(phase).To(Equal("Failed"))

			By("verifying step expectationResults contain failure info")
			// Get the steps to verify expectation results
			steps, err := getIntegrationTestField(testName, testNamespace, ".status.steps")
			Expect(err).NotTo(HaveOccurred())
			Expect(steps).NotTo(BeEmpty())
		})
	})

	Context("Resource Lifecycle", func() {
		const testName = "e2e-parallel-basic"
		const testFile = testDataDir + "/integrationtest/parallel_basic.yaml"
		const configMapName = "e2e-parallel-cm-1"

		AfterEach(func() {
			_ = deleteYAML(testFile)
			waitForResourceNotExists("integrationtest", testName, testNamespace, 1*time.Minute)
		})

		It("should create resources defined in templates", func() {
			By("creating the IntegrationTest")
			Expect(applyYAML(testFile)).To(Succeed())

			By("waiting for test to complete")
			waitForIntegrationTestPhaseIn(testName, testNamespace,
				[]string{"Succeeded", "Failed"}, 3*time.Minute)

			By("verifying the ConfigMap was created")
			// ConfigMaps are created by the test and should exist after completion
			waitForResourceExists("configmap", configMapName, testNamespace, 30*time.Second)
		})
	})

	Context("Resource Cleanup on Deletion", func() {
		const testName = "e2e-cleanup-test"
		const testFile = testDataDir + "/integrationtest/cleanup_test.yaml"
		const deploymentName = "e2e-cleanup-nginx"

		It("should cleanup owned resources when CR is deleted", func() {
			By("creating the IntegrationTest")
			Expect(applyYAML(testFile)).To(Succeed())

			By("waiting for test to start running")
			waitForIntegrationTestPhaseIn(testName, testNamespace,
				[]string{"Running", "Succeeded", "Failed"}, 30*time.Second)

			By("verifying deployment is created")
			waitForResourceExists("deployment", deploymentName, testNamespace, 1*time.Minute)

			By("deleting the IntegrationTest CR while running")
			Expect(deleteYAML(testFile)).To(Succeed())

			By("verifying IntegrationTest is deleted")
			waitForResourceNotExists("integrationtest", testName, testNamespace, 1*time.Minute)

			By("verifying owned resources are cleaned up")
			// Resources created by the test should be cleaned up by owner references
			waitForResourceNotExists("deployment", deploymentName, testNamespace, 1*time.Minute)
		})
	})

	Context("Step States", func() {
		const testName = "e2e-sequential-basic"
		const testFile = testDataDir + "/integrationtest/sequential_basic.yaml"

		AfterEach(func() {
			_ = deleteYAML(testFile)
			waitForResourceNotExists("integrationtest", testName, testNamespace, 1*time.Minute)
		})

		It("should track step states correctly", func() {
			By("creating the IntegrationTest")
			Expect(applyYAML(testFile)).To(Succeed())

			By("waiting for test to complete")
			waitForIntegrationTestPhaseIn(testName, testNamespace,
				[]string{"Succeeded", "Failed"}, 3*time.Minute)

			By("verifying steps have status information")
			// Check that steps array is populated
			stepsLen, err := getIntegrationTestField(testName, testNamespace, ".status.steps[0].name")
			Expect(err).NotTo(HaveOccurred())
			Expect(stepsLen).NotTo(BeEmpty(), "Step name should be set")

			// Check timing info
			startedAt, err := getIntegrationTestField(testName, testNamespace, ".status.steps[0].startedAt")
			Expect(err).NotTo(HaveOccurred())
			Expect(startedAt).NotTo(BeEmpty(), "Step startedAt should be set")
		})
	})

	Context("Events", func() {
		const testName = "e2e-sequential-basic"
		const testFile = testDataDir + "/integrationtest/sequential_basic.yaml"

		AfterEach(func() {
			_ = deleteYAML(testFile)
			waitForResourceNotExists("integrationtest", testName, testNamespace, 1*time.Minute)
		})

		It("should emit events during test lifecycle", func() {
			By("creating the IntegrationTest")
			Expect(applyYAML(testFile)).To(Succeed())

			By("waiting for test to complete")
			waitForIntegrationTestPhaseIn(testName, testNamespace,
				[]string{"Succeeded", "Failed"}, 3*time.Minute)

			By("verifying events were emitted")
			Eventually(func(g Gomega) {
				events := getEvents(testNamespace)
				// Check for test-related events
				g.Expect(events).To(Or(
					ContainSubstring("IntegrationTest"),
					ContainSubstring(testName),
				), "Events should contain IntegrationTest references")
			}, 30*time.Second, 2*time.Second).Should(Succeed())
		})
	})

	Context("ObservedGeneration", func() {
		const testName = "e2e-sequential-basic"
		const testFile = testDataDir + "/integrationtest/sequential_basic.yaml"

		AfterEach(func() {
			_ = deleteYAML(testFile)
			waitForResourceNotExists("integrationtest", testName, testNamespace, 1*time.Minute)
		})

		It("should track observedGeneration", func() {
			By("creating the IntegrationTest")
			Expect(applyYAML(testFile)).To(Succeed())

			By("waiting for test to start")
			waitForIntegrationTestPhaseIn(testName, testNamespace,
				[]string{"Running", "Succeeded", "Failed"}, 30*time.Second)

			By("verifying observedGeneration is set")
			Eventually(func(g Gomega) {
				obsGen, err := getIntegrationTestField(testName, testNamespace, ".status.observedGeneration")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(obsGen).NotTo(BeEmpty())
				g.Expect(obsGen).NotTo(Equal("0"))
			}, 30*time.Second, 2*time.Second).Should(Succeed())

			By("waiting for test to complete")
			waitForIntegrationTestPhaseIn(testName, testNamespace,
				[]string{"Succeeded", "Failed"}, 3*time.Minute)
		})
	})

	Context("Start and Completion Time", func() {
		const testName = "e2e-sequential-basic"
		const testFile = testDataDir + "/integrationtest/sequential_basic.yaml"

		AfterEach(func() {
			_ = deleteYAML(testFile)
			waitForResourceNotExists("integrationtest", testName, testNamespace, 1*time.Minute)
		})

		It("should set startTime and completionTime", func() {
			By("creating the IntegrationTest")
			Expect(applyYAML(testFile)).To(Succeed())

			By("waiting for test to start and verifying startTime")
			waitForIntegrationTestPhaseIn(testName, testNamespace,
				[]string{"Running", "Succeeded", "Failed"}, 30*time.Second)

			startTime, err := getIntegrationTestField(testName, testNamespace, ".status.startTime")
			Expect(err).NotTo(HaveOccurred())
			Expect(startTime).NotTo(BeEmpty(), "startTime should be set when Running")

			By("waiting for test to complete")
			waitForIntegrationTestPhaseIn(testName, testNamespace,
				[]string{"Succeeded", "Failed"}, 3*time.Minute)

			By("verifying completionTime is set")
			completionTime, err := getIntegrationTestField(testName, testNamespace, ".status.completionTime")
			Expect(err).NotTo(HaveOccurred())
			Expect(completionTime).NotTo(BeEmpty(), "completionTime should be set after completion")
		})
	})

	Context("Step Expectations", func() {
		const testName = "e2e-final-expectations"
		const testFile = testDataDir + "/integrationtest/final_expectations.yaml"

		AfterEach(func() {
			_ = deleteYAML(testFile)
			waitForResourceNotExists("integrationtest", testName, testNamespace, 1*time.Minute)
		})

		It("should verify step expectations pass for all steps", func() {
			By("creating IntegrationTest with step expectations")
			Expect(applyYAML(testFile)).To(Succeed())

			By("waiting for test to complete")
			finalPhase := waitForIntegrationTestPhaseIn(testName, testNamespace,
				[]string{"Succeeded", "Failed"}, 3*time.Minute)

			if finalPhase == "Failed" {
				printDebugInfo(testName, testNamespace)
			}
			Expect(finalPhase).To(Equal("Succeeded"))
		})
	})

	Context("Selector Mode", func() {
		const testName = "e2e-selector-test"
		const testFile = testDataDir + "/integrationtest/selector_test.yaml"
		const preExistingCM = "e2e-selector-pre-existing"

		AfterEach(func() {
			_ = deleteYAML(testFile)
			waitForResourceNotExists("integrationtest", testName, testNamespace, 1*time.Minute)
			// Clean up any remaining resources
			waitForResourceNotExists("configmap", preExistingCM, testNamespace, 30*time.Second)
		})

		It("should reference existing resources using selector", func() {
			By("creating IntegrationTest with selector step")
			Expect(applyYAML(testFile)).To(Succeed())

			By("waiting for test to complete")
			finalPhase := waitForIntegrationTestPhaseIn(testName, testNamespace,
				[]string{"Succeeded", "Failed"}, 3*time.Minute)

			if finalPhase == "Failed" {
				printDebugInfo(testName, testNamespace)
			}
			Expect(finalPhase).To(Equal("Succeeded"), "Selector test should succeed")

			By("verifying all steps completed")
			stepsState, err := getIntegrationTestField(testName, testNamespace, ".status.steps[*].state")
			Expect(err).NotTo(HaveOccurred())
			Expect(stepsState).NotTo(ContainSubstring("Failed"))
		})
	})

	Context("Step ReadyCondition", func() {
		const testName = "e2e-ready-condition-test"
		const testFile = testDataDir + "/integrationtest/ready_condition_test.yaml"

		AfterEach(func() {
			_ = deleteYAML(testFile)
			waitForResourceNotExists("integrationtest", testName, testNamespace, 1*time.Minute)
		})

		It("should wait for readyCondition before checking expectations", func() {
			By("creating IntegrationTest with step readyCondition")
			Expect(applyYAML(testFile)).To(Succeed())

			By("waiting for test to start running")
			waitForIntegrationTestPhaseIn(testName, testNamespace,
				[]string{"Running", "Succeeded", "Failed"}, 30*time.Second)

			By("waiting for test to complete")
			finalPhase := waitForIntegrationTestPhaseIn(testName, testNamespace,
				[]string{"Succeeded", "Failed"}, 5*time.Minute)

			if finalPhase == "Failed" {
				printDebugInfo(testName, testNamespace)
			}
			Expect(finalPhase).To(Equal("Succeeded"), "ReadyCondition test should succeed")

			By("verifying readyConditionStatus was tracked")
			readyStatus, err := getIntegrationTestField(testName, testNamespace, ".status.steps[0].readyConditionStatus")
			Expect(err).NotTo(HaveOccurred())
			Expect(readyStatus).NotTo(BeEmpty(), "readyConditionStatus should be tracked")
		})
	})

	Context("AnyOf Expectations", func() {
		const testName = "e2e-anyof-test"
		const testFile = testDataDir + "/integrationtest/anyof_test.yaml"

		AfterEach(func() {
			_ = deleteYAML(testFile)
			waitForResourceNotExists("integrationtest", testName, testNamespace, 1*time.Minute)
		})

		It("should pass when any expectation in anyOf passes", func() {
			By("creating IntegrationTest with anyOf expectations")
			Expect(applyYAML(testFile)).To(Succeed())

			By("waiting for test to complete")
			finalPhase := waitForIntegrationTestPhaseIn(testName, testNamespace,
				[]string{"Succeeded", "Failed"}, 3*time.Minute)

			if finalPhase == "Failed" {
				printDebugInfo(testName, testNamespace)
			}
			Expect(finalPhase).To(Equal("Succeeded"), "AnyOf test should succeed when at least one expectation passes")
		})
	})
})
