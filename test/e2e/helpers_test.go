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

//nolint:unparam // helper functions are designed for general use
package e2e

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2" //nolint:revive,staticcheck
	. "github.com/onsi/gomega"    //nolint:revive,staticcheck

	"github.com/lunz1207/testplane/test/utils"
)

const (
	// testDataDir is the directory containing test data files
	testDataDir = "test/e2e/testdata"
)

// applyYAML applies a YAML file to the cluster
func applyYAML(filePath string) error {
	projectDir, _ := utils.GetProjectDir()
	fullPath := filepath.Join(projectDir, filePath)
	cmd := exec.Command("kubectl", "apply", "-f", fullPath)
	_, err := utils.Run(cmd)
	return err
}

// deleteYAML deletes resources defined in a YAML file
func deleteYAML(filePath string) error {
	projectDir, _ := utils.GetProjectDir()
	fullPath := filepath.Join(projectDir, filePath)
	cmd := exec.Command("kubectl", "delete", "-f", fullPath, "--ignore-not-found", "--wait=false")
	_, err := utils.Run(cmd)
	return err
}

// getIntegrationTestPhase gets the phase of an IntegrationTest
func getIntegrationTestPhase(name, namespace string) (string, error) {
	cmd := exec.Command("kubectl", "get", "integrationtest", name,
		"-n", namespace, "-o", "jsonpath={.status.phase}")
	output, err := utils.Run(cmd)
	return strings.TrimSpace(output), err
}

// getLoadTestPhase gets the phase of a LoadTest
func getLoadTestPhase(name, namespace string) (string, error) {
	cmd := exec.Command("kubectl", "get", "loadtest", name,
		"-n", namespace, "-o", "jsonpath={.status.phase}")
	output, err := utils.Run(cmd)
	return strings.TrimSpace(output), err
}

// getIntegrationTestField gets a field from IntegrationTest using jsonpath
func getIntegrationTestField(name, namespace, jsonpath string) (string, error) {
	cmd := exec.Command("kubectl", "get", "integrationtest", name,
		"-n", namespace, "-o", fmt.Sprintf("jsonpath={%s}", jsonpath))
	output, err := utils.Run(cmd)
	return strings.TrimSpace(output), err
}

// getLoadTestField gets a field from LoadTest using jsonpath
func getLoadTestField(name, namespace, jsonpath string) (string, error) {
	cmd := exec.Command("kubectl", "get", "loadtest", name,
		"-n", namespace, "-o", fmt.Sprintf("jsonpath={%s}", jsonpath))
	output, err := utils.Run(cmd)
	return strings.TrimSpace(output), err
}

// waitForIntegrationTestPhase waits for IntegrationTest to reach a specific phase
func waitForIntegrationTestPhase(name, namespace, expectedPhase string, timeout time.Duration) {
	By(fmt.Sprintf("waiting for IntegrationTest %s to reach phase %s", name, expectedPhase))
	Eventually(func(g Gomega) {
		phase, err := getIntegrationTestPhase(name, namespace)
		g.Expect(err).NotTo(HaveOccurred(), "Failed to get IntegrationTest phase")
		g.Expect(phase).To(Equal(expectedPhase),
			fmt.Sprintf("IntegrationTest phase is %s, expected %s", phase, expectedPhase))
	}, timeout, 2*time.Second).Should(Succeed())
}

// waitForLoadTestPhase waits for LoadTest to reach a specific phase
func waitForLoadTestPhase(name, namespace, expectedPhase string, timeout time.Duration) {
	By(fmt.Sprintf("waiting for LoadTest %s to reach phase %s", name, expectedPhase))
	Eventually(func(g Gomega) {
		phase, err := getLoadTestPhase(name, namespace)
		g.Expect(err).NotTo(HaveOccurred(), "Failed to get LoadTest phase")
		g.Expect(phase).To(Equal(expectedPhase),
			fmt.Sprintf("LoadTest phase is %s, expected %s", phase, expectedPhase))
	}, timeout, 2*time.Second).Should(Succeed())
}

// waitForIntegrationTestPhaseIn waits for IntegrationTest to reach one of the specified phases
func waitForIntegrationTestPhaseIn(
	name, namespace string,
	expectedPhases []string,
	timeout time.Duration,
) string {
	By(fmt.Sprintf("waiting for IntegrationTest %s to reach one of phases %v", name, expectedPhases))
	var currentPhase string
	Eventually(func(g Gomega) {
		phase, err := getIntegrationTestPhase(name, namespace)
		g.Expect(err).NotTo(HaveOccurred(), "Failed to get IntegrationTest phase")
		currentPhase = phase
		g.Expect(expectedPhases).To(ContainElement(phase),
			fmt.Sprintf("IntegrationTest phase is %s, expected one of %v", phase, expectedPhases))
	}, timeout, 2*time.Second).Should(Succeed())
	return currentPhase
}

// waitForLoadTestPhaseIn waits for LoadTest to reach one of the specified phases
func waitForLoadTestPhaseIn(
	name, namespace string,
	expectedPhases []string,
	timeout time.Duration,
) string {
	By(fmt.Sprintf("waiting for LoadTest %s to reach one of phases %v", name, expectedPhases))
	var currentPhase string
	Eventually(func(g Gomega) {
		phase, err := getLoadTestPhase(name, namespace)
		g.Expect(err).NotTo(HaveOccurred(), "Failed to get LoadTest phase")
		currentPhase = phase
		g.Expect(expectedPhases).To(ContainElement(phase),
			fmt.Sprintf("LoadTest phase is %s, expected one of %v", phase, expectedPhases))
	}, timeout, 2*time.Second).Should(Succeed())
	return currentPhase
}

// resourceExists checks if a Kubernetes resource exists
func resourceExists(kind, name, namespace string) bool {
	cmd := exec.Command("kubectl", "get", kind, name, "-n", namespace, "--ignore-not-found")
	output, err := utils.Run(cmd)
	if err != nil {
		return false
	}
	return strings.TrimSpace(output) != ""
}

// waitForResourceExists waits for a resource to exist
func waitForResourceExists(kind, name, namespace string, timeout time.Duration) {
	By(fmt.Sprintf("waiting for %s/%s to exist", kind, name))
	Eventually(func() bool {
		return resourceExists(kind, name, namespace)
	}, timeout, 2*time.Second).Should(BeTrue())
}

// waitForResourceNotExists waits for a resource to be deleted
func waitForResourceNotExists(kind, name, namespace string, timeout time.Duration) {
	By(fmt.Sprintf("waiting for %s/%s to be deleted", kind, name))
	Eventually(func() bool {
		return !resourceExists(kind, name, namespace)
	}, timeout, 2*time.Second).Should(BeTrue())
}

// getEvents gets events for a namespace
func getEvents(namespace string) string {
	cmd := exec.Command("kubectl", "get", "events", "-n", namespace, "--sort-by=.lastTimestamp")
	output, _ := utils.Run(cmd)
	return output
}

// deleteResource deletes a specific resource by name
func deleteResource(kind, name, namespace string) {
	cmd := exec.Command("kubectl", "delete", kind, name,
		"-n", namespace, "--ignore-not-found", "--wait=false")
	_, _ = utils.Run(cmd)
}

// cleanupResources cleans up test resources from a namespace
func cleanupResources(namespace string) {
	By("cleaning up test resources")
	// Delete IntegrationTests
	cmd := exec.Command("kubectl", "delete", "integrationtest", "--all",
		"-n", namespace, "--ignore-not-found")
	_, _ = utils.Run(cmd)

	// Delete LoadTests
	cmd = exec.Command("kubectl", "delete", "loadtest", "--all",
		"-n", namespace, "--ignore-not-found")
	_, _ = utils.Run(cmd)

	// Delete test deployments
	cmd = exec.Command("kubectl", "delete", "deployment",
		"-l", "app.kubernetes.io/managed-by=testplane-e2e",
		"-n", namespace, "--ignore-not-found")
	_, _ = utils.Run(cmd)

	// Delete test pods
	cmd = exec.Command("kubectl", "delete", "pod",
		"-l", "app.kubernetes.io/managed-by=testplane-e2e",
		"-n", namespace, "--ignore-not-found", "--grace-period=1")
	_, _ = utils.Run(cmd)

	// Delete test configmaps
	cmd = exec.Command("kubectl", "delete", "configmap",
		"-l", "app.kubernetes.io/managed-by=testplane-e2e",
		"-n", namespace, "--ignore-not-found")
	_, _ = utils.Run(cmd)
}

// printDebugInfo prints debugging information for failed tests
func printDebugInfo(testName, namespace string) {
	By("printing debug information")

	// Print IntegrationTest status
	cmd := exec.Command("kubectl", "get", "integrationtest", testName,
		"-n", namespace, "-o", "yaml")
	output, err := utils.Run(cmd)
	if err == nil {
		_, _ = fmt.Fprintf(GinkgoWriter,
			"\n=== IntegrationTest %s ===\n%s\n", testName, output)
	}

	// Print LoadTest status
	cmd = exec.Command("kubectl", "get", "loadtest", testName,
		"-n", namespace, "-o", "yaml")
	output, err = utils.Run(cmd)
	if err == nil {
		_, _ = fmt.Fprintf(GinkgoWriter,
			"\n=== LoadTest %s ===\n%s\n", testName, output)
	}

	// Print events
	events := getEvents(namespace)
	_, _ = fmt.Fprintf(GinkgoWriter, "\n=== Events in %s ===\n%s\n", namespace, events)
}
