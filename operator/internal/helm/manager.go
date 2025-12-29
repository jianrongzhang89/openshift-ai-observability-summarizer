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

package helm

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Manager handles Helm chart operations
type Manager struct {
	client    client.Client
	chartPath string // Base path to Helm charts
}

// NewManager creates a new Helm manager
func NewManager(c client.Client, chartPath string) *Manager {
	return &Manager{
		client:    c,
		chartPath: chartPath,
	}
}

// ChartSpec defines parameters for rendering a Helm chart
type ChartSpec struct {
	ChartName   string                 // Name of the chart (e.g., "rag", "mcp-server")
	ReleaseName string                 // Helm release name
	Namespace   string                 // Target namespace
	Values      map[string]interface{} // Values to override
	CreateNS    bool                   // Create namespace if not exists
}

// RenderChart renders a Helm chart with the given values and returns the manifests
func (m *Manager) RenderChart(ctx context.Context, spec ChartSpec) ([]unstructured.Unstructured, error) {
	logger := log.FromContext(ctx)
	logger.Info("Rendering Helm chart", "chart", spec.ChartName, "release", spec.ReleaseName, "namespace", spec.Namespace)

	// Load the chart
	chartPath := filepath.Join(m.chartPath, spec.ChartName)
	chartRequested, err := loader.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load chart %s: %w", spec.ChartName, err)
	}

	// Validate chart
	if err := chartRequested.Validate(); err != nil {
		return nil, fmt.Errorf("chart validation failed for %s: %w", spec.ChartName, err)
	}

	// Setup Helm action configuration
	settings := cli.New()
	settings.SetNamespace(spec.Namespace)

	actionConfig := new(action.Configuration)
	// Initialize with the namespace
	if err := actionConfig.Init(settings.RESTClientGetter(), spec.Namespace, "secret", logger.Info); err != nil {
		return nil, fmt.Errorf("failed to initialize Helm action config: %w", err)
	}

	// Create install action to render templates
	install := action.NewInstall(actionConfig)
	install.DryRun = true
	install.ReleaseName = spec.ReleaseName
	install.Namespace = spec.Namespace
	install.Replace = true
	install.ClientOnly = true
	install.CreateNamespace = spec.CreateNS

	// Render the release
	rel, err := install.Run(chartRequested, spec.Values)
	if err != nil {
		return nil, fmt.Errorf("failed to render chart %s: %w", spec.ChartName, err)
	}

	// Parse manifests into unstructured objects
	manifests, err := m.parseManifests(rel.Manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to parse manifests for %s: %w", spec.ChartName, err)
	}

	logger.Info("Successfully rendered Helm chart", "chart", spec.ChartName, "objects", len(manifests))
	return manifests, nil
}

// parseManifests parses Helm-rendered YAML manifests into unstructured objects
func (m *Manager) parseManifests(manifest string) ([]unstructured.Unstructured, error) {
	var objects []unstructured.Unstructured

	// Split by YAML document separator
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(manifest)), 4096)

	for {
		var obj unstructured.Unstructured
		if err := decoder.Decode(&obj); err != nil {
			if err == io.EOF {
				break
			}
			// Skip empty documents
			continue
		}

		// Skip empty objects
		if obj.Object == nil || len(obj.Object) == 0 {
			continue
		}

		objects = append(objects, obj)
	}

	return objects, nil
}

// ApplyManifests applies a list of unstructured objects to the cluster
func (m *Manager) ApplyManifests(ctx context.Context, manifests []unstructured.Unstructured, namespace string) error {
	logger := log.FromContext(ctx)

	for _, obj := range manifests {
		gvk := obj.GroupVersionKind()

		// Ensure namespace is set for namespaced resources
		// If the object doesn't have a namespace set, use the target namespace
		if obj.GetNamespace() == "" && !isClusterScoped(gvk.Kind) {
			obj.SetNamespace(namespace)
		}

		logger.Info("Applying resource", "kind", gvk.Kind, "name", obj.GetName(), "namespace", obj.GetNamespace())

		// Check if object exists
		existing := &unstructured.Unstructured{}
		existing.SetGroupVersionKind(gvk)
		err := m.client.Get(ctx, client.ObjectKey{
			Namespace: obj.GetNamespace(),
			Name:      obj.GetName(),
		}, existing)

		if err != nil {
			if client.IgnoreNotFound(err) == nil {
				// Object doesn't exist, create it
				if err := m.client.Create(ctx, &obj); err != nil {
					return fmt.Errorf("failed to create %s %s/%s: %w", gvk.Kind, obj.GetNamespace(), obj.GetName(), err)
				}
				logger.Info("Created resource", "kind", gvk.Kind, "name", obj.GetName())
			} else {
				return fmt.Errorf("failed to get %s %s/%s: %w", gvk.Kind, obj.GetNamespace(), obj.GetName(), err)
			}
		} else {
			// Object exists
			// Special handling for Jobs - delete and recreate since they're immutable
			if gvk.Kind == "Job" {
				logger.Info("Job exists, deleting and recreating", "kind", gvk.Kind, "name", obj.GetName())
				// Delete the existing Job
				if err := m.client.Delete(ctx, existing); err != nil {
					return fmt.Errorf("failed to delete existing Job %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
				}
				// Create the new Job
				if err := m.client.Create(ctx, &obj); err != nil {
					return fmt.Errorf("failed to recreate Job %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
				}
				logger.Info("Recreated Job", "kind", gvk.Kind, "name", obj.GetName())
				continue
			}

			// Skip updates for immutable resources
			if isImmutableResource(gvk.Kind) {
				logger.Info("Resource already exists, skipping update for immutable resource", "kind", gvk.Kind, "name", obj.GetName())
				continue
			}

			// Special handling for Deployments with changed selectors (immutable field)
			if gvk.Kind == "Deployment" && hasSelectorChanged(existing, &obj) {
				logger.Info("Deployment selector changed, deleting and recreating", "kind", gvk.Kind, "name", obj.GetName())
				// Delete the existing deployment
				if err := m.client.Delete(ctx, existing); err != nil {
					return fmt.Errorf("failed to delete %s %s/%s with changed selector: %w", gvk.Kind, obj.GetNamespace(), obj.GetName(), err)
				}
				// Create the new deployment
				if err := m.client.Create(ctx, &obj); err != nil {
					return fmt.Errorf("failed to recreate %s %s/%s: %w", gvk.Kind, obj.GetNamespace(), obj.GetName(), err)
				}
				logger.Info("Recreated resource with new selector", "kind", gvk.Kind, "name", obj.GetName())
				continue
			}

			// Update the resource
			obj.SetResourceVersion(existing.GetResourceVersion())
			if err := m.client.Update(ctx, &obj); err != nil {
				return fmt.Errorf("failed to update %s %s/%s: %w", gvk.Kind, obj.GetNamespace(), obj.GetName(), err)
			}
			logger.Info("Updated resource", "kind", gvk.Kind, "name", obj.GetName())
		}
	}

	return nil
}

// InstallOrUpgradeChart renders and applies a Helm chart
func (m *Manager) InstallOrUpgradeChart(ctx context.Context, spec ChartSpec) error {
	logger := log.FromContext(ctx)
	logger.Info("Installing/upgrading Helm chart", "chart", spec.ChartName, "release", spec.ReleaseName)

	// Create namespace if needed
	if spec.CreateNS {
		if err := m.ensureNamespace(ctx, spec.Namespace); err != nil {
			return fmt.Errorf("failed to create namespace: %w", err)
		}
	}

	// Also ensure namespace from global.namespace value if specified
	if globalNS, ok := getGlobalNamespace(spec.Values); ok && globalNS != "" && globalNS != spec.Namespace {
		logger.Info("Creating namespace from global.namespace", "namespace", globalNS)
		if err := m.ensureNamespace(ctx, globalNS); err != nil {
			return fmt.Errorf("failed to create global namespace: %w", err)
		}
	}

	// Render the chart
	manifests, err := m.RenderChart(ctx, spec)
	if err != nil {
		return fmt.Errorf("failed to render chart: %w", err)
	}

	// Apply manifests
	if err := m.ApplyManifests(ctx, manifests, spec.Namespace); err != nil {
		return fmt.Errorf("failed to apply manifests: %w", err)
	}

	logger.Info("Successfully installed/upgraded Helm chart", "chart", spec.ChartName)
	return nil
}

// getGlobalNamespace extracts the global.namespace value from Helm values
func getGlobalNamespace(values map[string]interface{}) (string, bool) {
	if values == nil {
		return "", false
	}

	global, ok := values["global"].(map[string]interface{})
	if !ok {
		return "", false
	}

	namespace, ok := global["namespace"].(string)
	return namespace, ok
}

// ensureNamespace creates a namespace if it doesn't exist
func (m *Manager) ensureNamespace(ctx context.Context, name string) error {
	logger := log.FromContext(ctx)

	ns := &unstructured.Unstructured{}
	ns.SetAPIVersion("v1")
	ns.SetKind("Namespace")
	ns.SetName(name)

	err := m.client.Get(ctx, client.ObjectKey{Name: name}, ns)
	if err == nil {
		logger.Info("Namespace already exists", "namespace", name)
		return nil
	}

	if client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed to check namespace: %w", err)
	}

	// Create the namespace
	logger.Info("Creating namespace", "namespace", name)
	if err := m.client.Create(ctx, ns); err != nil {
		return fmt.Errorf("failed to create namespace %s: %w", name, err)
	}

	logger.Info("Namespace created successfully", "namespace", name)
	return nil
}

// isImmutableResource returns true if the resource should not be updated once created
func isImmutableResource(kind string) bool {
	immutableKinds := map[string]bool{
		"PersistentVolumeClaim": true, // PVC spec is immutable except for storage size
		"ServiceAccount":        true, // Avoid conflicts with token management
	}
	return immutableKinds[kind]
}

// hasSelectorChanged checks if a Deployment's selector has changed
func hasSelectorChanged(existing, desired *unstructured.Unstructured) bool {
	// Get selectors from both objects
	existingSelector, existingFound, _ := unstructured.NestedMap(existing.Object, "spec", "selector", "matchLabels")
	desiredSelector, desiredFound, _ := unstructured.NestedMap(desired.Object, "spec", "selector", "matchLabels")

	// If selectors don't exist in one or both, consider them different
	if existingFound != desiredFound {
		return true
	}

	// Compare selector labels
	if len(existingSelector) != len(desiredSelector) {
		return true
	}

	for key, existingValue := range existingSelector {
		desiredValue, ok := desiredSelector[key]
		if !ok || existingValue != desiredValue {
			return true
		}
	}

	return false
}

// GetDefaultStorageClass queries the cluster for the default StorageClass
func (m *Manager) GetDefaultStorageClass(ctx context.Context) string {
	logger := log.FromContext(ctx)

	// Try to list StorageClasses
	storageClasses := &unstructured.UnstructuredList{}
	storageClasses.SetAPIVersion("storage.k8s.io/v1")
	storageClasses.SetKind("StorageClass")

	if err := m.client.List(ctx, storageClasses); err != nil {
		logger.Error(err, "Failed to list StorageClasses")
		return "gp3-csi" // Fallback default
	}

	// Check for default StorageClass with standard annotation
	for _, sc := range storageClasses.Items {
		annotations := sc.GetAnnotations()
		if annotations != nil {
			// Check standard annotation
			if isDefault, ok := annotations["storageclass.kubernetes.io/is-default-class"]; ok && isDefault == "true" {
				logger.Info("Found default StorageClass", "name", sc.GetName())
				return sc.GetName()
			}
			// Check beta annotation (for older Kubernetes versions)
			if isDefault, ok := annotations["storageclass.beta.kubernetes.io/is-default-class"]; ok && isDefault == "true" {
				logger.Info("Found default StorageClass (beta annotation)", "name", sc.GetName())
				return sc.GetName()
			}
		}
	}

	logger.Info("No default StorageClass found, using fallback", "default", "gp3-csi")
	return "gp3-csi" // Fallback default for AWS clusters
}

// isClusterScoped returns true if the resource kind is cluster-scoped
func isClusterScoped(kind string) bool {
	clusterScopedKinds := map[string]bool{
		"Namespace":                      true,
		"ClusterRole":                    true,
		"ClusterRoleBinding":             true,
		"PersistentVolume":               true,
		"StorageClass":                   true,
		"CustomResourceDefinition":       true,
		"APIService":                     true,
		"ValidatingWebhookConfiguration": true,
		"MutatingWebhookConfiguration":   true,
		"PriorityClass":                  true,
		"RuntimeClass":                   true,
		"CSIDriver":                      true,
		"CSINode":                        true,
		"VolumeAttachment":               true,
	}
	return clusterScopedKinds[kind]
}

// UninstallChart removes resources associated with a Helm release
func (m *Manager) UninstallChart(ctx context.Context, releaseName, namespace string) error {
	logger := log.FromContext(ctx)
	logger.Info("Uninstalling Helm release", "release", releaseName, "namespace", namespace)

	settings := cli.New()
	settings.SetNamespace(namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), namespace, "secret", logger.Info); err != nil {
		return fmt.Errorf("failed to initialize Helm action config: %w", err)
	}

	uninstall := action.NewUninstall(actionConfig)
	if _, err := uninstall.Run(releaseName); err != nil {
		return fmt.Errorf("failed to uninstall release %s: %w", releaseName, err)
	}

	logger.Info("Successfully uninstalled Helm release", "release", releaseName)
	return nil
}

// GetReleaseStatus gets the status of a Helm release
func (m *Manager) GetReleaseStatus(ctx context.Context, releaseName, namespace string) (*release.Release, error) {
	logger := log.FromContext(ctx)

	settings := cli.New()
	settings.SetNamespace(namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), namespace, "secret", logger.Info); err != nil {
		return nil, fmt.Errorf("failed to initialize Helm action config: %w", err)
	}

	get := action.NewGet(actionConfig)
	rel, err := get.Run(releaseName)
	if err != nil {
		return nil, fmt.Errorf("failed to get release %s: %w", releaseName, err)
	}

	logger.Info("Retrieved release status", "release", releaseName, "status", rel.Info.Status)
	return rel, nil
}

// ListReleases lists all Helm releases in a namespace
func (m *Manager) ListReleases(ctx context.Context, namespace string) ([]*release.Release, error) {
	logger := log.FromContext(ctx)

	settings := cli.New()
	settings.SetNamespace(namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), namespace, "secret", logger.Info); err != nil {
		return nil, fmt.Errorf("failed to initialize Helm action config: %w", err)
	}

	list := action.NewList(actionConfig)
	list.All = true

	releases, err := list.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to list releases: %w", err)
	}

	logger.Info("Listed Helm releases", "namespace", namespace, "count", len(releases))
	return releases, nil
}

// GetChartMetadata returns metadata for a chart
func (m *Manager) GetChartMetadata(chartName string) (*chart.Metadata, error) {
	chartPath := filepath.Join(m.chartPath, chartName)
	chartRequested, err := loader.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load chart %s: %w", chartName, err)
	}

	return chartRequested.Metadata, nil
}

// ValidateChart validates a Helm chart
func (m *Manager) ValidateChart(chartName string) error {
	chartPath := filepath.Join(m.chartPath, chartName)
	chartRequested, err := loader.Load(chartPath)
	if err != nil {
		return fmt.Errorf("failed to load chart %s: %w", chartName, err)
	}

	return chartRequested.Validate()
}

// GetResourceGVK returns the GroupVersionKind for a Kubernetes resource string
func GetResourceGVK(apiVersion, kind string) schema.GroupVersionKind {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		// Fallback for core resources
		return schema.GroupVersionKind{
			Group:   "",
			Version: apiVersion,
			Kind:    kind,
		}
	}

	return schema.GroupVersionKind{
		Group:   gv.Group,
		Version: gv.Version,
		Kind:    kind,
	}
}
