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

// Package watch provides dynamic watch capabilities for test resources.
package watch

import (
	"context"
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	infrav1alpha1 "github.com/lunz1207/testplane/api/v1alpha1"
)

var log = logf.Log.WithName("dynamic-watch")

// WatchTarget represents a resource to watch.
type WatchTarget struct {
	// GVK is the GroupVersionKind of the resource.
	GVK schema.GroupVersionKind
	// Namespace is the namespace of the resource.
	Namespace string
	// Name is the name of the resource. Empty means watch all resources of this GVK in the namespace.
	Name string
}

// String returns a string representation of the WatchTarget.
func (t WatchTarget) String() string {
	return fmt.Sprintf("%s/%s/%s/%s", t.GVK.Group, t.GVK.Kind, t.Namespace, t.Name)
}

// DynamicWatchManager manages dynamic watches for test resources.
// When watched resources change, it uses OwnerReference to find the associated
// IntegrationTest and triggers reconciliation.
type DynamicWatchManager struct {
	cache  cache.Cache
	client client.Client

	mu sync.RWMutex
	// activeTests tracks which tests are currently waiting for assertions.
	// Key format: "namespace/name"
	activeTests map[string]struct{}
	// registeredGVKs tracks which GVKs have informer handlers registered.
	registeredGVKs map[schema.GroupVersionKind]bool

	// eventChan is used to send events to trigger reconciles.
	eventChan chan event.TypedGenericEvent[*infrav1alpha1.IntegrationTest]
}

// NewDynamicWatchManager creates a new DynamicWatchManager.
func NewDynamicWatchManager(c cache.Cache, cli client.Client) *DynamicWatchManager {
	return &DynamicWatchManager{
		cache:          c,
		client:         cli,
		activeTests:    make(map[string]struct{}),
		registeredGVKs: make(map[schema.GroupVersionKind]bool),
		eventChan:      make(chan event.TypedGenericEvent[*infrav1alpha1.IntegrationTest], 1024),
	}
}

// EventSource returns a source that can be used with controller-runtime's Watches.
// This source will emit events when watched resources change.
func (m *DynamicWatchManager) EventSource() source.Source {
	return source.Channel(m.eventChan, &handler.TypedEnqueueRequestForObject[*infrav1alpha1.IntegrationTest]{})
}

// StartWatch registers the test as active and ensures informer handlers are registered
// for the target resource GVKs. When these resources change, the manager will use
// OwnerReference to find the associated IntegrationTest and trigger reconciliation.
func (m *DynamicWatchManager) StartWatch(ctx context.Context, namespace, testName string, targets []WatchTarget) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Mark test as active
	testKey := m.testKey(namespace, testName)
	m.activeTests[testKey] = struct{}{}

	// Ensure informer handlers are registered for all target GVKs
	for _, target := range targets {
		if err := m.ensureInformerHandler(ctx, target.GVK); err != nil {
			log.Error(err, "failed to register informer handler", "gvk", target.GVK)
			// Continue with other targets, don't fail completely
			continue
		}
	}

	log.V(1).Info("started watch", "test", testKey, "targets", len(targets))
	return nil
}

// StopWatch marks the test as inactive.
func (m *DynamicWatchManager) StopWatch(namespace, testName string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	testKey := m.testKey(namespace, testName)
	delete(m.activeTests, testKey)

	log.V(1).Info("stopped watch", "test", testKey)
}

// IsWatching checks if the test is currently active (waiting for assertions).
func (m *DynamicWatchManager) IsWatching(namespace, testName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	testKey := m.testKey(namespace, testName)
	_, exists := m.activeTests[testKey]
	return exists
}

// ensureInformerHandler ensures an event handler is registered for the GVK.
func (m *DynamicWatchManager) ensureInformerHandler(ctx context.Context, gvk schema.GroupVersionKind) error {
	if m.registeredGVKs[gvk] {
		return nil
	}

	// Get informer for this GVK
	informer, err := m.cache.GetInformerForKind(ctx, gvk)
	if err != nil {
		return fmt.Errorf("get informer for %v: %w", gvk, err)
	}

	// Add event handler
	_, err = informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { m.onResourceChange(obj) },
		UpdateFunc: func(old, new interface{}) { m.onResourceChange(new) },
		DeleteFunc: func(obj interface{}) { m.onResourceChange(obj) },
	})
	if err != nil {
		return fmt.Errorf("add event handler for %v: %w", gvk, err)
	}

	m.registeredGVKs[gvk] = true
	log.V(1).Info("registered informer handler", "gvk", gvk)
	return nil
}

// onResourceChange handles resource change events.
// It uses OwnerReference to find the associated IntegrationTest and triggers reconcile.
func (m *DynamicWatchManager) onResourceChange(obj interface{}) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		// Handle DeletedFinalStateUnknown
		if tombstone, ok := obj.(toolscache.DeletedFinalStateUnknown); ok {
			u, ok = tombstone.Obj.(*unstructured.Unstructured)
			if !ok {
				return
			}
		} else {
			return
		}
	}

	// Find IntegrationTest via OwnerReference
	namespace, testName := m.findOwnerIntegrationTest(u)
	if testName == "" {
		return
	}

	// Check if this test is active
	testKey := m.testKey(namespace, testName)
	m.mu.RLock()
	_, isActive := m.activeTests[testKey]
	m.mu.RUnlock()

	if !isActive {
		return
	}

	// Trigger reconcile by sending an event
	it := &infrav1alpha1.IntegrationTest{}
	it.SetNamespace(namespace)
	it.SetName(testName)

	// Non-blocking send to avoid blocking the informer
	select {
	case m.eventChan <- event.TypedGenericEvent[*infrav1alpha1.IntegrationTest]{Object: it}:
		log.V(1).Info("triggered reconcile via OwnerReference",
			"test", testKey,
			"resource", fmt.Sprintf("%s/%s", u.GetKind(), u.GetName()))
	default:
		log.V(1).Info("event channel full, reconcile will be triggered by next event or fallback requeue",
			"test", testKey)
	}
}

// findOwnerIntegrationTest finds the IntegrationTest that owns this resource.
// Returns (namespace, name) of the IntegrationTest, or ("", "") if not found.
func (m *DynamicWatchManager) findOwnerIntegrationTest(u *unstructured.Unstructured) (namespace, name string) {
	for _, owner := range u.GetOwnerReferences() {
		if owner.Kind == "IntegrationTest" && owner.APIVersion == infrav1alpha1.GroupVersion.String() {
			return u.GetNamespace(), owner.Name
		}
	}
	return "", ""
}

// testKey generates a unique key for an IntegrationTest.
func (m *DynamicWatchManager) testKey(namespace, name string) string {
	return fmt.Sprintf("%s/%s", namespace, name)
}

// GetEventChannel returns the event channel for testing purposes.
func (m *DynamicWatchManager) GetEventChannel() <-chan event.TypedGenericEvent[*infrav1alpha1.IntegrationTest] {
	return m.eventChan
}
