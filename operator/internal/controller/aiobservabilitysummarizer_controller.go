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

package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	obsv1alpha1 "github.com/redhat-et/openshift-ai-observability-summarizer-operator/api/v1alpha1"
	"github.com/redhat-et/openshift-ai-observability-summarizer-operator/internal/dependencies"
	"github.com/redhat-et/openshift-ai-observability-summarizer-operator/internal/helm"
	"github.com/redhat-et/openshift-ai-observability-summarizer-operator/internal/operators"
)

const (
	// Condition types
	ConditionTypeReady                   = "Ready"
	ConditionTypeOperatorsInstalled      = "OperatorsInstalled"
	ConditionTypeObservabilityStackReady = "ObservabilityStackReady"
	ConditionTypeRAGReady                = "RAGReady"
	ConditionTypeMCPServerReady          = "MCPServerReady"
	ConditionTypeConsolePluginReady      = "ConsolePluginReady"
	ConditionTypeDegraded                = "Degraded"
	ConditionTypeProgressing             = "Progressing"

	// Phase constants
	PhasePending    = "Pending"
	PhaseInstalling = "Installing"
	PhaseReady      = "Ready"
	PhaseFailed     = "Failed"
	PhaseUpgrading  = "Upgrading"

	// Component state constants
	StateComponentPending    = "Pending"
	StateComponentInstalling = "Installing"
	StateComponentReady      = "Ready"
	StateComponentDegraded   = "Degraded"
	StateComponentFailed     = "Failed"

	// Health status constants
	HealthHealthy   = "Healthy"
	HealthDegraded  = "Degraded"
	HealthUnhealthy = "Unhealthy"

	// Requeue interval
	RequeueAfter = 5 * time.Minute
)

// AIObservabilitySummarizerReconciler reconciles a AIObservabilitySummarizer object
type AIObservabilitySummarizerReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	HelmManager *helm.Manager
}

// +kubebuilder:rbac:groups=obs.redhat.com,resources=aiobservabilitysummarizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=obs.redhat.com,resources=aiobservabilitysummarizers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=obs.redhat.com,resources=aiobservabilitysummarizers/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete;bind;escalate
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operators.coreos.com,resources=subscriptions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operators.coreos.com,resources=operatorgroups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operators.coreos.com,resources=installplans,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=operators.coreos.com,resources=clusterserviceversions,verbs=get;list;watch
// +kubebuilder:rbac:groups=operators.coreos.com,resources=operators,verbs=get;list;watch
// +kubebuilder:rbac:groups=tempo.grafana.com,resources=*,verbs=*
// +kubebuilder:rbac:groups=loki.grafana.com,resources=*,verbs=*
// +kubebuilder:rbac:groups=opentelemetry.io,resources=*,verbs=*
// +kubebuilder:rbac:groups=logging.openshift.io,resources=*,verbs=*
// +kubebuilder:rbac:groups=observability.openshift.io,resources=*,verbs=*
// +kubebuilder:rbac:groups=core,resources=appliedclusterresourcequotas,verbs=get;list;watch
// +kubebuilder:rbac:groups=quota.openshift.io,resources=appliedclusterresourcequotas,verbs=get;list;watch
// +kubebuilder:rbac:groups=quota.openshift.io,resources=clusterresourcequotas,verbs=get;list;watch
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=*,verbs=get;list;watch
// +kubebuilder:rbac:groups=monitoring.openshift.io,resources=*,verbs=get;list;watch
// +kubebuilder:rbac:groups=console.openshift.io,resources=consoleplugins,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.openshift.io,resources=consoles,verbs=get;list;watch;patch;update
// +kubebuilder:rbac:groups=core,resources=configmaps,resourceNames=cluster-monitoring-config,verbs=get;update;patch;create
// +kubebuilder:rbac:groups=core,resources=namespaces,resourceNames=openshift-monitoring,verbs=get

// Reconcile implements the main reconciliation loop for AIObservabilitySummarizer
// It follows a 7-phase approach:
// 1. Validation
// 2. Install Required Operators
// 3. Build Dependency Graph
// 4. Deploy Components
// 5. Post-Install Configuration
// 6. Health Checks
// 7. Self-Healing
func (r *AIObservabilitySummarizerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling AIObservabilitySummarizer", "name", req.Name, "namespace", req.Namespace)

	// Fetch the AIObservabilitySummarizer instance
	aiobs := &obsv1alpha1.AIObservabilitySummarizer{}
	if err := r.Get(ctx, req.NamespacedName, aiobs); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("AIObservabilitySummarizer resource not found, ignoring")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get AIObservabilitySummarizer")
		return ctrl.Result{}, err
	}

	// Initialize status if needed
	if aiobs.Status.Phase == "" {
		aiobs.Status.Phase = PhasePending
		aiobs.Status.ObservedGeneration = aiobs.Generation
		aiobs.Status.InstalledOperators = make(map[string]string)
		aiobs.Status.Health.Components = make(map[string]obsv1alpha1.ComponentHealth)
		if err := r.Status().Update(ctx, aiobs); err != nil {
			logger.Error(err, "Failed to initialize status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Phase 1: Validation
	if err := r.validateSpec(ctx, aiobs); err != nil {
		logger.Error(err, "Validation failed")
		return r.updateStatusFailed(ctx, aiobs, "Validation failed: "+err.Error())
	}
	logger.Info("Phase 1: Validation passed")

	// Phase 2: Install Required Operators
	if !r.isConditionTrue(aiobs, ConditionTypeOperatorsInstalled) {
		logger.Info("Phase 2: Installing required OpenShift operators")
		if err := r.installRequiredOperators(ctx, aiobs); err != nil {
			logger.Error(err, "Failed to install operators")
			return r.updateStatusProgressing(ctx, aiobs, "Installing operators")
		}
	}
	logger.Info("Phase 2: Required operators installed")

	// Phase 3: Build Dependency Graph
	logger.Info("Phase 3: Building dependency graph")
	depGraph := r.buildDependencyGraph(ctx, aiobs)

	// Phase 4: Deploy Components (via Helm)
	if err := r.deployComponents(ctx, aiobs, depGraph); err != nil {
		logger.Error(err, "Component deployment failed")
		return r.updateStatusProgressing(ctx, aiobs, "Deploying components")
	}
	logger.Info("Phase 4: Components deployed")

	// Phase 5: Post-Install Configuration
	logger.Info("Phase 5: Post-install configuration")
	if err := r.postInstallConfiguration(ctx, aiobs); err != nil {
		logger.Error(err, "Post-install configuration failed")
		return r.updateStatusProgressing(ctx, aiobs, "Configuring components")
	}

	// Phase 6: Health Checks
	logger.Info("Phase 6: Running health checks")
	if err := r.runHealthChecks(ctx, aiobs); err != nil {
		logger.Error(err, "Health checks failed")
		// Don't fail the reconciliation, just mark as degraded
		r.setCondition(aiobs, metav1.Condition{
			Type:    ConditionTypeDegraded,
			Status:  metav1.ConditionTrue,
			Reason:  "HealthChecksFailed",
			Message: err.Error(),
		})
	}

	// Phase 7: Self-Healing (if enabled)
	if aiobs.Spec.SelfHealing {
		logger.Info("Phase 7: Self-healing enabled, monitoring components")
		if err := r.selfHeal(ctx, aiobs); err != nil {
			logger.Error(err, "Self-healing failed")
			// Don't fail reconciliation
		}
	}

	// Update status to Ready
	return r.updateStatusReady(ctx, aiobs)
}

// validateSpec validates the AIObservabilitySummarizer spec
func (r *AIObservabilitySummarizerReconciler) validateSpec(ctx context.Context, aiobs *obsv1alpha1.AIObservabilitySummarizer) error {
	logger := log.FromContext(ctx)

	// Validate namespace is specified
	if aiobs.Spec.Namespace == "" {
		return fmt.Errorf("spec.namespace is required")
	}

	// Validate that either inline config or ref is provided (not both)
	if aiobs.Spec.RAG != nil && aiobs.Spec.RAGBackendRef != nil {
		return fmt.Errorf("cannot specify both spec.rag and spec.ragBackendRef")
	}
	if aiobs.Spec.MCPServer != nil && aiobs.Spec.MCPServerRef != nil {
		return fmt.Errorf("cannot specify both spec.mcpServer and spec.mcpServerRef")
	}
	if aiobs.Spec.ConsolePlugin != nil && aiobs.Spec.ConsolePluginRef != nil {
		return fmt.Errorf("cannot specify both spec.consolePlugin and spec.consolePluginRef")
	}
	if aiobs.Spec.ObservabilityStack != nil && aiobs.Spec.ObservabilityStackRef != nil {
		return fmt.Errorf("cannot specify both spec.observabilityStack and spec.observabilityStackRef")
	}

	// Validate RAG config if inline
	if aiobs.Spec.RAG != nil && aiobs.Spec.RAG.Enabled {
		if aiobs.Spec.RAG.Model == "" {
			return fmt.Errorf("spec.rag.model is required when RAG is enabled")
		}
	}

	// Verify namespace exists or can be created
	namespace := &corev1.Namespace{}
	if err := r.Get(ctx, client.ObjectKey{Name: aiobs.Spec.Namespace}, namespace); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Target namespace does not exist, will be created", "namespace", aiobs.Spec.Namespace)
			// Namespace will be created during deployment
		} else {
			return fmt.Errorf("failed to check namespace: %w", err)
		}
	}

	logger.Info("Validation successful")
	return nil
}

// installRequiredOperators installs the required OpenShift operators
func (r *AIObservabilitySummarizerReconciler) installRequiredOperators(ctx context.Context, aiobs *obsv1alpha1.AIObservabilitySummarizer) error {
	logger := log.FromContext(ctx)
	logger.Info("Installing required OpenShift operators")

	// Create operator installer
	installer := operators.NewInstaller(r.Client)

	// Build namespace overrides from CR spec
	overrides := r.buildNamespaceOverrides(aiobs)

	// Check if operators are already installed
	status, err := installer.GetAllOperatorStatus(ctx)
	if err != nil {
		logger.Error(err, "Failed to get operator status")
		return err
	}

	// Check if all operators are ready
	allReady := true
	for name, version := range status {
		if version == "Not Ready" {
			allReady = false
			logger.Info("Operator not ready", "operator", name)
		} else {
			logger.Info("Operator ready", "operator", name, "version", version)
		}
	}

	if !allReady {
		logger.Info("Installing missing operators with namespace overrides", "overrides", overrides)
		installedVersions, err := installer.InstallAllWithOverrides(ctx, overrides)
		if err != nil {
			logger.Error(err, "Failed to install operators")
			return err
		}

		// Update status with installed operators
		aiobs.Status.InstalledOperators = installedVersions
	} else {
		logger.Info("All required operators are already installed")
		aiobs.Status.InstalledOperators = status
	}

	// Set condition
	r.setCondition(aiobs, metav1.Condition{
		Type:    ConditionTypeOperatorsInstalled,
		Status:  metav1.ConditionTrue,
		Reason:  "OperatorsInstalled",
		Message: fmt.Sprintf("All %d required operators are installed", len(status)),
	})

	return r.Status().Update(ctx, aiobs)
}

// buildNamespaceOverrides builds namespace overrides from CR spec
func (r *AIObservabilitySummarizerReconciler) buildNamespaceOverrides(aiobs *obsv1alpha1.AIObservabilitySummarizer) *operators.NamespaceOverrides {
	overrides := &operators.NamespaceOverrides{}

	// Extract namespaces from observabilityStack config
	if aiobs.Spec.ObservabilityStack != nil {
		// Cluster Observability Operator - keep default namespace
		// It's a cluster-wide operator

		// OpenTelemetry Operator - use OTel Collector namespace
		if aiobs.Spec.ObservabilityStack.OTelCollector != nil && aiobs.Spec.ObservabilityStack.OTelCollector.Namespace != "" {
			overrides.OpenTelemetry = "openshift-opentelemetry-operator"
		}

		// Tempo Operator - use Tempo namespace
		if aiobs.Spec.ObservabilityStack.Tempo != nil && aiobs.Spec.ObservabilityStack.Tempo.Namespace != "" {
			overrides.Tempo = "openshift-tempo-operator"
		}

		// Logging Operator - use Loki namespace (OwnNamespace mode)
		if aiobs.Spec.ObservabilityStack.Loki != nil && aiobs.Spec.ObservabilityStack.Loki.Namespace != "" {
			overrides.Logging = aiobs.Spec.ObservabilityStack.Loki.Namespace
		}

		// Loki Operator - use default namespace
		if aiobs.Spec.ObservabilityStack.Loki != nil && aiobs.Spec.ObservabilityStack.Loki.Namespace != "" {
			overrides.Loki = "openshift-operators-redhat"
		}
	}

	return overrides
}

// Helper functions for status updates
func (r *AIObservabilitySummarizerReconciler) updateStatusProgressing(ctx context.Context, aiobs *obsv1alpha1.AIObservabilitySummarizer, message string) (ctrl.Result, error) {
	aiobs.Status.Phase = PhaseInstalling
	aiobs.Status.LastReconcileTime = &metav1.Time{Time: time.Now()}

	r.setCondition(aiobs, metav1.Condition{
		Type:    ConditionTypeProgressing,
		Status:  metav1.ConditionTrue,
		Reason:  "Installing",
		Message: message,
	})

	if err := r.Status().Update(ctx, aiobs); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *AIObservabilitySummarizerReconciler) updateStatusReady(ctx context.Context, aiobs *obsv1alpha1.AIObservabilitySummarizer) (ctrl.Result, error) {
	aiobs.Status.Phase = PhaseReady
	aiobs.Status.LastReconcileTime = &metav1.Time{Time: time.Now()}
	aiobs.Status.ObservedGeneration = aiobs.Generation

	r.setCondition(aiobs, metav1.Condition{
		Type:    ConditionTypeReady,
		Status:  metav1.ConditionTrue,
		Reason:  "ReconcileSuccess",
		Message: "All components are ready",
	})

	r.setCondition(aiobs, metav1.Condition{
		Type:    ConditionTypeProgressing,
		Status:  metav1.ConditionFalse,
		Reason:  "ReconcileSuccess",
		Message: "Reconciliation completed",
	})

	if err := r.Status().Update(ctx, aiobs); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: RequeueAfter}, nil
}

func (r *AIObservabilitySummarizerReconciler) updateStatusFailed(ctx context.Context, aiobs *obsv1alpha1.AIObservabilitySummarizer, message string) (ctrl.Result, error) {
	aiobs.Status.Phase = PhaseFailed
	aiobs.Status.LastReconcileTime = &metav1.Time{Time: time.Now()}

	r.setCondition(aiobs, metav1.Condition{
		Type:    ConditionTypeReady,
		Status:  metav1.ConditionFalse,
		Reason:  "ReconcileFailed",
		Message: message,
	})

	if err := r.Status().Update(ctx, aiobs); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: RequeueAfter}, nil
}

// setCondition sets or updates a condition in the status
func (r *AIObservabilitySummarizerReconciler) setCondition(aiobs *obsv1alpha1.AIObservabilitySummarizer, condition metav1.Condition) {
	condition.LastTransitionTime = metav1.Now()
	meta.SetStatusCondition(&aiobs.Status.Conditions, condition)
}

// isConditionTrue checks if a condition is true
func (r *AIObservabilitySummarizerReconciler) isConditionTrue(aiobs *obsv1alpha1.AIObservabilitySummarizer, conditionType string) bool {
	condition := meta.FindStatusCondition(aiobs.Status.Conditions, conditionType)
	return condition != nil && condition.Status == metav1.ConditionTrue
}

// buildDependencyGraph creates and initializes the dependency graph based on CR spec
func (r *AIObservabilitySummarizerReconciler) buildDependencyGraph(ctx context.Context, aiobs *obsv1alpha1.AIObservabilitySummarizer) *dependencies.DependencyGraph {
	logger := log.FromContext(ctx)
	logger.Info("Building dependency graph for component deployment")

	// Create new dependency graph
	graph := dependencies.NewDependencyGraph()

	// Remove RAG components if RAG is disabled
	if aiobs.Spec.RAG == nil || !aiobs.Spec.RAG.Enabled {
		logger.Info("RAG is disabled, removing RAG components from dependency graph")
		graph.RemoveComponent(dependencies.ComponentPostgreSQL)
		graph.RemoveComponent(dependencies.ComponentLlamaStack)
		graph.RemoveComponent(dependencies.ComponentIngestionPipe)
	}

	// Update component states based on current status
	if aiobs.Status.Components.MinIO.State != "" {
		graph.SetComponentState(dependencies.ComponentMinIO, r.mapComponentState(aiobs.Status.Components.MinIO.State))
	}
	if aiobs.Status.Components.Tempo.State != "" {
		graph.SetComponentState(dependencies.ComponentTempo, r.mapComponentState(aiobs.Status.Components.Tempo.State))
	}
	if aiobs.Status.Components.Loki.State != "" {
		graph.SetComponentState(dependencies.ComponentLoki, r.mapComponentState(aiobs.Status.Components.Loki.State))
	}
	if aiobs.Status.Components.OTelCollector.State != "" {
		graph.SetComponentState(dependencies.ComponentOTelCollector, r.mapComponentState(aiobs.Status.Components.OTelCollector.State))
	}
	if aiobs.Status.Components.Korrel8r.State != "" {
		graph.SetComponentState(dependencies.ComponentKorrel8r, r.mapComponentState(aiobs.Status.Components.Korrel8r.State))
	}
	if aiobs.Status.Components.RAG.State != "" && aiobs.Spec.RAG != nil && aiobs.Spec.RAG.Enabled {
		graph.SetComponentState(dependencies.ComponentPostgreSQL, r.mapComponentState(aiobs.Status.Components.RAG.State))
		graph.SetComponentState(dependencies.ComponentLlamaStack, r.mapComponentState(aiobs.Status.Components.RAG.State))
		graph.SetComponentState(dependencies.ComponentIngestionPipe, r.mapComponentState(aiobs.Status.Components.RAG.State))
	}
	if aiobs.Status.Components.MCPServer.State != "" {
		graph.SetComponentState(dependencies.ComponentMCPServer, r.mapComponentState(aiobs.Status.Components.MCPServer.State))
	}
	if aiobs.Status.Components.ConsolePlugin.State != "" {
		graph.SetComponentState(dependencies.ComponentConsolePlugin, r.mapComponentState(aiobs.Status.Components.ConsolePlugin.State))
	}
	if aiobs.Status.Components.UI.State != "" {
		graph.SetComponentState(dependencies.ComponentUI, r.mapComponentState(aiobs.Status.Components.UI.State))
	}

	return graph
}

// mapComponentState maps CR component state to dependency graph state
func (r *AIObservabilitySummarizerReconciler) mapComponentState(state string) dependencies.ComponentState {
	switch state {
	case StateComponentReady:
		return dependencies.StateReady
	case StateComponentInstalling:
		return dependencies.StateDeploying
	case StateComponentFailed:
		return dependencies.StateFailed
	default:
		return dependencies.StateNotDeployed
	}
}

// deployComponents deploys components in dependency order using Helm
func (r *AIObservabilitySummarizerReconciler) deployComponents(ctx context.Context, aiobs *obsv1alpha1.AIObservabilitySummarizer, graph *dependencies.DependencyGraph) error {
	logger := log.FromContext(ctx)

	// Get deployment order
	order, err := graph.GetDeploymentOrder()
	if err != nil {
		return fmt.Errorf("failed to get deployment order: %w", err)
	}

	logger.Info("Deployment order determined", "components", order)

	anyComponentDeploying := false

	// Deploy each component in order
	for _, componentType := range order {
		ready, notReady, err := graph.AreDependenciesReady(componentType)
		if err != nil {
			return fmt.Errorf("failed to check dependencies for %s: %w", componentType, err)
		}

		if !ready {
			logger.Info("Skipping component - dependencies not ready", "component", componentType, "waiting_for", notReady)
			continue
		}

		state, _ := graph.GetComponentState(componentType)
		if state == dependencies.StateReady {
			logger.Info("Component already deployed", "component", componentType)
			continue
		}

		// If component is already deploying, check if it's ready now
		if state == dependencies.StateDeploying {
			logger.Info("Checking readiness of deploying component", "component", componentType)
			isReady, message := r.isComponentReady(ctx, aiobs, componentType)
			if isReady {
				logger.Info("Component is now ready", "component", componentType)
				graph.SetComponentState(componentType, dependencies.StateReady)
				r.updateComponentStatus(aiobs, componentType, StateComponentReady, message)
			} else {
				logger.Info("Component still deploying", "component", componentType, "status", message)
				anyComponentDeploying = true
			}
			continue
		}

		// Deploy component
		logger.Info("Deploying component", "component", componentType)
		if err := r.deployComponent(ctx, aiobs, componentType); err != nil {
			logger.Error(err, "Failed to deploy component", "component", componentType)
			graph.SetComponentState(componentType, dependencies.StateFailed)
			r.updateComponentStatus(aiobs, componentType, StateComponentFailed, err.Error())
			return fmt.Errorf("failed to deploy %s: %w", componentType, err)
		}

		// Mark as deploying initially
		graph.SetComponentState(componentType, dependencies.StateDeploying)
		r.updateComponentStatus(aiobs, componentType, StateComponentInstalling, "Deployment in progress")

		// Check if component is ready
		isReady, message := r.isComponentReady(ctx, aiobs, componentType)
		if isReady {
			logger.Info("Component is ready", "component", componentType)
			graph.SetComponentState(componentType, dependencies.StateReady)
			r.updateComponentStatus(aiobs, componentType, StateComponentReady, message)
		} else {
			logger.Info("Component deployed but not yet ready", "component", componentType, "status", message)
			anyComponentDeploying = true
		}
	}

	// Update status after all components processed
	if err := r.Status().Update(ctx, aiobs); err != nil {
		logger.Error(err, "Failed to update component status")
		return err
	}

	// If any component is still deploying, return error to trigger faster requeue
	if anyComponentDeploying {
		return fmt.Errorf("components still deploying")
	}

	return nil
}

// deployComponent deploys a single component using Helm
func (r *AIObservabilitySummarizerReconciler) deployComponent(ctx context.Context, aiobs *obsv1alpha1.AIObservabilitySummarizer, componentType dependencies.ComponentType) error {
	logger := log.FromContext(ctx)

	// Map component type to Helm chart and values
	chartName, releaseName, namespace, values := r.getComponentChartInfo(aiobs, componentType)

	if chartName == "" {
		logger.Info("Component not enabled in spec", "component", componentType)
		return nil
	}

	// Use component-specific namespace, or fall back to aiobs.Spec.Namespace
	if namespace == "" {
		namespace = aiobs.Spec.Namespace
	}

	// Special handling for Loki: inject detected default StorageClass if not specified
	if componentType == dependencies.ComponentLoki {
		// Check if storageClassName is already set in the values
		if lokiStackVals, ok := values["lokiStack"].(map[string]interface{}); ok {
			if scName, exists := lokiStackVals["storageClassName"]; !exists || scName == "" {
				// Detect default StorageClass from cluster
				defaultSC := r.HelmManager.GetDefaultStorageClass(ctx)
				logger.Info("Using detected default StorageClass for Loki", "storageClassName", defaultSC)
				lokiStackVals["storageClassName"] = defaultSC
			}
		}
	}

	// Create Helm chart spec
	chartSpec := helm.ChartSpec{
		ChartName:   chartName,
		ReleaseName: releaseName,
		Namespace:   namespace,
		Values:      values,
		CreateNS:    true,
	}

	// Install or upgrade chart
	if err := r.HelmManager.InstallOrUpgradeChart(ctx, chartSpec); err != nil {
		return fmt.Errorf("failed to install chart %s: %w", chartName, err)
	}

	logger.Info("Successfully deployed component", "component", componentType, "chart", chartName, "namespace", namespace)
	return nil
}

// getComponentChartInfo returns chart name, release name, namespace, and values for a component
func (r *AIObservabilitySummarizerReconciler) getComponentChartInfo(aiobs *obsv1alpha1.AIObservabilitySummarizer, componentType dependencies.ComponentType) (string, string, string, map[string]interface{}) {
	imageRegistry := aiobs.Spec.ImageRegistry
	if imageRegistry == "" {
		imageRegistry = "quay.io/ecosystem-appeng"
	}
	imageVersion := aiobs.Spec.ImageVersion
	if imageVersion == "" {
		imageVersion = "1.0.7"
	}

	switch componentType {
	case dependencies.ComponentMinIO:
		if aiobs.Spec.ObservabilityStack != nil && aiobs.Spec.ObservabilityStack.MinIO != nil {
			namespace := aiobs.Spec.ObservabilityStack.MinIO.Namespace
			if namespace == "" {
				namespace = "observability-hub" // Default namespace
			}
			values := map[string]interface{}{
				"storageSize": aiobs.Spec.ObservabilityStack.MinIO.StorageSize,
				"global": map[string]interface{}{
					"namespace": namespace,
				},
				"minio": map[string]interface{}{
					"buckets": []string{"tempo", "loki"},
				},
			}
			return "minio", "minio-observability-storage", namespace, values
		}

	case dependencies.ComponentTempo:
		if aiobs.Spec.ObservabilityStack != nil && aiobs.Spec.ObservabilityStack.Tempo != nil {
			namespace := aiobs.Spec.ObservabilityStack.Tempo.Namespace
			if namespace == "" {
				namespace = "observability-hub" // Default from chart
			}
			values := map[string]interface{}{
				"global": map[string]interface{}{
					"namespace": namespace,
				},
			}
			if aiobs.Spec.ObservabilityStack.Tempo.Retention != "" {
				values["retention"] = aiobs.Spec.ObservabilityStack.Tempo.Retention
			}
			return "observability/tempo", "tempo", namespace, values
		}

	case dependencies.ComponentLoki:
		if aiobs.Spec.ObservabilityStack != nil && aiobs.Spec.ObservabilityStack.Loki != nil {
			namespace := aiobs.Spec.ObservabilityStack.Loki.Namespace
			if namespace == "" {
				namespace = "openshift-logging" // Default from chart
			}
			lokiStackVals := map[string]interface{}{}
			if aiobs.Spec.ObservabilityStack.Loki.Retention != "" {
				lokiStackVals["retention"] = aiobs.Spec.ObservabilityStack.Loki.Retention
			}
			// Note: storageClassName will be auto-detected and injected in deployComponent()
			values := map[string]interface{}{
				"global": map[string]interface{}{
					"namespace": namespace,
				},
				"lokiStack": lokiStackVals,
			}
			return "observability/loki", "loki", namespace, values
		}

	case dependencies.ComponentOTelCollector:
		if aiobs.Spec.ObservabilityStack != nil && aiobs.Spec.ObservabilityStack.OTelCollector != nil {
			namespace := aiobs.Spec.ObservabilityStack.OTelCollector.Namespace
			if namespace == "" {
				namespace = "observability-hub" // Default from chart
			}
			values := map[string]interface{}{
				"global": map[string]interface{}{
					"namespace": namespace,
				},
			}
			return "observability/otel-collector", "otel-collector", namespace, values
		}

	case dependencies.ComponentKorrel8r:
		// Korrel8r is always enabled
		namespace := "openshift-cluster-observability-operator" // Default from chart
		values := map[string]interface{}{
			"global": map[string]interface{}{
				"namespace": namespace,
			},
		}
		return "observability/korrel8r", "korrel8r-summarizer", namespace, values

	case dependencies.ComponentPostgreSQL, dependencies.ComponentLlamaStack, dependencies.ComponentIngestionPipe:
		// All RAG components (PostgreSQL, LlamaStack, IngestionPipe) are deployed together
		// using the same Helm chart and release name to avoid conflicts
		if aiobs.Spec.RAG != nil && aiobs.Spec.RAG.Enabled {
			values := map[string]interface{}{
				"model": aiobs.Spec.RAG.Model,
			}

			// Note: llama-stack uses upstream llamastack/distribution-starter image
			// We don't override it unless the user specifies a custom llama-stack image

			// Add PostgreSQL configuration if available
			if aiobs.Spec.RAG.PostgreSQL != nil {
				values["postgresql"] = map[string]interface{}{
					"storageSize": aiobs.Spec.RAG.PostgreSQL.StorageSize,
				}
			}

			// Add ingestion pipeline configuration
			values["ingestion-pipeline"] = map[string]interface{}{
				"enabled": true,
				"source":  "S3",
			}

			// Use a single release name "rag" for all RAG components
			// RAG components use the default namespace (aiobs.Spec.Namespace)
			return "rag", "rag", "", values
		}

	case dependencies.ComponentMCPServer:
		if aiobs.Spec.MCPServer != nil && aiobs.Spec.MCPServer.Enabled {
			// MCP Server uses the default namespace (aiobs.Spec.Namespace)
			return "mcp-server", "mcp-server", "", map[string]interface{}{
				"image": map[string]interface{}{
					"repository": imageRegistry + "/aiobs-mcp-server",
					"tag":        imageVersion,
				},
				"replicaCount": aiobs.Spec.MCPServer.Replicas,
			}
		}

	case dependencies.ComponentConsolePlugin:
		if aiobs.Spec.ConsolePlugin != nil && aiobs.Spec.ConsolePlugin.Enabled {
			// Console Plugin uses the default namespace (aiobs.Spec.Namespace)
			// Note: console plugin expects plugin.image as full path including tag
			return "openshift-console-plugin", "console-plugin", "", map[string]interface{}{
				"plugin": map[string]interface{}{
					"image": imageRegistry + "/aiobs-console-plugin:" + imageVersion,
				},
			}
		}

	case dependencies.ComponentUI:
		// UI component is always deployed when MCP Server is available
		// No specific Spec configuration required
		// UI uses the default namespace (aiobs.Spec.Namespace)
		return "ui", "ui", "", map[string]interface{}{
			"image": map[string]interface{}{
				"repository": imageRegistry + "/aiobs-metrics-ui",
				"tag":        imageVersion,
			},
		}
	}

	return "", "", "", nil
}

// postInstallConfiguration performs post-deployment configuration
func (r *AIObservabilitySummarizerReconciler) postInstallConfiguration(ctx context.Context, aiobs *obsv1alpha1.AIObservabilitySummarizer) error {
	logger := log.FromContext(ctx)

	// Enable user workload monitoring if observability stack is enabled
	if aiobs.Spec.ObservabilityStack != nil && aiobs.Spec.ObservabilityStack.Enabled {
		logger.Info("Enabling user workload monitoring for observability stack")
		if err := r.enableUserWorkloadMonitoring(ctx); err != nil {
			logger.Error(err, "Failed to enable user workload monitoring")
			// Don't fail the entire reconciliation if this fails
			// Just log the error and continue
		}
	}

	// Enable console plugin if configured
	if aiobs.Spec.ConsolePlugin != nil && aiobs.Spec.ConsolePlugin.Enabled {
		logger.Info("Enabling console plugin in OpenShift Console")
		if err := r.enableConsolePlugin(ctx, "openshift-ai-observability"); err != nil {
			logger.Error(err, "Failed to enable console plugin")
			// Don't fail the entire reconciliation if this fails
			// Just log the error and continue
		}
	}

	// Configure tracing if enabled
	if aiobs.Spec.EnableTracing {
		logger.Info("Tracing enabled, configuring instrumentation")
		if err := r.setupAutoInstrumentation(ctx, aiobs); err != nil {
			logger.Error(err, "Failed to setup auto-instrumentation")
			// Don't fail the entire reconciliation if this fails
			// Just log the error and continue
		}
	}

	// Configure logging if enabled
	if aiobs.Spec.EnableLogging {
		logger.Info("Logging enabled, configuring log forwarding")
		// TODO: Configure log forwarding to Loki
	}

	return nil
}

// runHealthChecks performs health checks on deployed components
func (r *AIObservabilitySummarizerReconciler) runHealthChecks(ctx context.Context, aiobs *obsv1alpha1.AIObservabilitySummarizer) error {
	logger := log.FromContext(ctx)
	logger.Info("Running component health checks")

	// Initialize health status if needed
	if aiobs.Status.Health.Components == nil {
		aiobs.Status.Health.Components = make(map[string]obsv1alpha1.ComponentHealth)
	}

	// Check each component's health
	// This is a basic implementation - Phase 3 will add comprehensive health checks

	healthy := 0
	degraded := 0
	unhealthy := 0

	components := map[string]obsv1alpha1.ComponentStatus{
		"rag":           aiobs.Status.Components.RAG,
		"mcpServer":     aiobs.Status.Components.MCPServer,
		"consolePlugin": aiobs.Status.Components.ConsolePlugin,
		"ui":            aiobs.Status.Components.UI,
		"minio":         aiobs.Status.Components.MinIO,
		"tempo":         aiobs.Status.Components.Tempo,
		"loki":          aiobs.Status.Components.Loki,
		"otelCollector": aiobs.Status.Components.OTelCollector,
	}

	for componentName, component := range components {
		health := obsv1alpha1.ComponentHealth{
			Status: "Healthy",
		}

		if component.State == StateComponentFailed {
			health.Status = "Unhealthy"
			health.Message = "Component deployment failed"
			unhealthy++
		} else if component.State == StateComponentDegraded {
			health.Status = "Degraded"
			degraded++
		} else if component.State == StateComponentReady {
			healthy++
		}

		aiobs.Status.Health.Components[componentName] = health
	}

	// Set overall health
	if unhealthy > 0 {
		aiobs.Status.Health.Overall = HealthUnhealthy
	} else if degraded > 0 {
		aiobs.Status.Health.Overall = HealthDegraded
	} else {
		aiobs.Status.Health.Overall = HealthHealthy
	}

	aiobs.Status.Health.LastCheckTime = &metav1.Time{Time: time.Now()}

	return r.Status().Update(ctx, aiobs)
}

// selfHeal attempts to remediate unhealthy components
func (r *AIObservabilitySummarizerReconciler) selfHeal(ctx context.Context, aiobs *obsv1alpha1.AIObservabilitySummarizer) error {
	logger := log.FromContext(ctx)

	// Check for unhealthy components
	for componentName, health := range aiobs.Status.Health.Components {
		if health.Status == "Unhealthy" {
			logger.Info("Attempting self-healing", "component", componentName)
			// TODO: Implement self-healing actions
			// - Restart failed pods
			// - Recreate missing resources
			// - Trigger rollout restart for degraded deployments
		}
	}

	return nil
}

// updateComponentStatus updates the status of a component in the CR
func (r *AIObservabilitySummarizerReconciler) updateComponentStatus(aiobs *obsv1alpha1.AIObservabilitySummarizer, componentType dependencies.ComponentType, state string, message string) {
	now := metav1.Now()
	componentStatus := obsv1alpha1.ComponentStatus{
		State:          state,
		Message:        message,
		LastUpdateTime: &now,
	}

	switch componentType {
	case dependencies.ComponentMinIO:
		aiobs.Status.Components.MinIO = componentStatus
	case dependencies.ComponentTempo:
		aiobs.Status.Components.Tempo = componentStatus
	case dependencies.ComponentLoki:
		aiobs.Status.Components.Loki = componentStatus
	case dependencies.ComponentOTelCollector:
		aiobs.Status.Components.OTelCollector = componentStatus
	case dependencies.ComponentKorrel8r:
		aiobs.Status.Components.Korrel8r = componentStatus
	case dependencies.ComponentPostgreSQL, dependencies.ComponentLlamaStack, dependencies.ComponentIngestionPipe:
		// All RAG components share the same status
		aiobs.Status.Components.RAG = componentStatus
	case dependencies.ComponentMCPServer:
		aiobs.Status.Components.MCPServer = componentStatus
	case dependencies.ComponentConsolePlugin:
		aiobs.Status.Components.ConsolePlugin = componentStatus
	case dependencies.ComponentUI:
		aiobs.Status.Components.UI = componentStatus
	}
}

// isComponentReady checks if a component is ready
func (r *AIObservabilitySummarizerReconciler) isComponentReady(ctx context.Context, aiobs *obsv1alpha1.AIObservabilitySummarizer, componentType dependencies.ComponentType) (bool, string) {
	logger := log.FromContext(ctx)
	namespace := aiobs.Spec.Namespace

	switch componentType {
	case dependencies.ComponentMinIO:
		// Check StatefulSet readiness AND bucket initialization Job completion
		minioNamespace := namespace
		if aiobs.Spec.ObservabilityStack != nil && aiobs.Spec.ObservabilityStack.MinIO != nil && aiobs.Spec.ObservabilityStack.MinIO.Namespace != "" {
			minioNamespace = aiobs.Spec.ObservabilityStack.MinIO.Namespace
		} else {
			minioNamespace = "observability-hub"
		}

		// First check if StatefulSet is ready
		stsReady, msg := r.isStatefulSetReady(ctx, minioNamespace, "minio-observability-storage")
		if !stsReady {
			return false, msg
		}

		// Then check if bucket initialization Job has completed
		// Note: Job name is based on Helm release name pattern
		jobReady, msg := r.isJobCompleted(ctx, minioNamespace, "minio-observability-storage-bucket-init")
		if !jobReady {
			// If Job is not found, it may have been cleaned up by TTL (300s after completion)
			// In this case, if StatefulSet is ready, we can assume buckets were initialized
			if msg == "Job not found" {
				logger.Info("Bucket init Job not found (likely cleaned up by TTL), assuming ready since StatefulSet is ready",
					"namespace", minioNamespace, "statefulset", "minio-observability-storage")
				return true, "MinIO ready (bucket init Job completed and cleaned up)"
			}
			return false, "Bucket initialization: " + msg
		}

		return true, "MinIO ready with buckets initialized"

	case dependencies.ComponentTempo:
		// Check if TempoStack CR exists and is ready
		tempoNamespace := "observability-hub" // default
		if aiobs.Spec.ObservabilityStack != nil && aiobs.Spec.ObservabilityStack.Tempo != nil && aiobs.Spec.ObservabilityStack.Tempo.Namespace != "" {
			tempoNamespace = aiobs.Spec.ObservabilityStack.Tempo.Namespace
		}
		return r.isTempoStackReady(ctx, tempoNamespace, "tempo")

	case dependencies.ComponentLoki:
		// Check if LokiStack CR exists and is ready
		lokiNamespace := "openshift-logging" // default
		if aiobs.Spec.ObservabilityStack != nil && aiobs.Spec.ObservabilityStack.Loki != nil && aiobs.Spec.ObservabilityStack.Loki.Namespace != "" {
			lokiNamespace = aiobs.Spec.ObservabilityStack.Loki.Namespace
		}
		return r.isLokiStackReady(ctx, lokiNamespace, "logging-loki")

	case dependencies.ComponentOTelCollector:
		// Check if OpenTelemetryCollector CR exists and pods are ready
		otelNamespace := "observability-hub" // default
		if aiobs.Spec.ObservabilityStack != nil && aiobs.Spec.ObservabilityStack.OTelCollector != nil && aiobs.Spec.ObservabilityStack.OTelCollector.Namespace != "" {
			otelNamespace = aiobs.Spec.ObservabilityStack.OTelCollector.Namespace
		}
		return r.isDeploymentReady(ctx, otelNamespace, "otel-collector-collector")

	case dependencies.ComponentKorrel8r:
		// Check Korrel8r Deployment readiness
		return r.isDeploymentReady(ctx, "openshift-cluster-observability-operator", "korrel8r-summarizer")

	case dependencies.ComponentPostgreSQL:
		// Check PostgreSQL StatefulSet readiness
		return r.isStatefulSetReady(ctx, namespace, "pgvector")

	case dependencies.ComponentLlamaStack:
		// Check llama-stack Deployment readiness
		// Note: deployment name is "llamastack" (no dash)
		return r.isDeploymentReady(ctx, namespace, "llamastack")

	case dependencies.ComponentIngestionPipe:
		// Ingestion pipeline is a Job or one-time task - mark as ready after deployment
		return true, "Ingestion pipeline configured"

	case dependencies.ComponentMCPServer:
		// Check MCP Server Deployment readiness
		return r.isDeploymentReady(ctx, namespace, "mcp-server")

	case dependencies.ComponentConsolePlugin:
		// Check Console Plugin Deployment readiness
		// The deployment name comes from plugin.name in values.yaml, which defaults to "openshift-ai-observability"
		return r.isDeploymentReady(ctx, namespace, "openshift-ai-observability")

	case dependencies.ComponentUI:
		// Check UI Deployment readiness
		return r.isDeploymentReady(ctx, namespace, "ui")

	default:
		logger.Info("Unknown component type for readiness check", "component", componentType)
		return false, "Unknown component type"
	}
}

// isStatefulSetReady checks if a StatefulSet is ready
func (r *AIObservabilitySummarizerReconciler) isStatefulSetReady(ctx context.Context, namespace, name string) (bool, string) {
	logger := log.FromContext(ctx)

	statefulSet := &appsv1.StatefulSet{}
	err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, statefulSet)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, "StatefulSet not found"
		}
		logger.Error(err, "Failed to get StatefulSet", "name", name)
		return false, fmt.Sprintf("Error checking StatefulSet: %v", err)
	}

	// Check if all replicas are ready
	if statefulSet.Status.ReadyReplicas == statefulSet.Status.Replicas && statefulSet.Status.Replicas > 0 {
		return true, "All replicas ready"
	}

	return false, fmt.Sprintf("Ready replicas: %d/%d", statefulSet.Status.ReadyReplicas, statefulSet.Status.Replicas)
}

// isDeploymentReady checks if a Deployment is ready
func (r *AIObservabilitySummarizerReconciler) isDeploymentReady(ctx context.Context, namespace, name string) (bool, string) {
	logger := log.FromContext(ctx)

	deployment := &appsv1.Deployment{}
	err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, deployment)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, "Deployment not found"
		}
		logger.Error(err, "Failed to get Deployment", "name", name)
		return false, fmt.Sprintf("Error checking Deployment: %v", err)
	}

	// Check if all replicas are ready
	if deployment.Status.ReadyReplicas == deployment.Status.Replicas && deployment.Status.Replicas > 0 {
		return true, "All replicas ready"
	}

	return false, fmt.Sprintf("Ready replicas: %d/%d", deployment.Status.ReadyReplicas, deployment.Status.Replicas)
}

// isJobCompleted checks if a Job has completed successfully
func (r *AIObservabilitySummarizerReconciler) isJobCompleted(ctx context.Context, namespace, name string) (bool, string) {
	logger := log.FromContext(ctx)

	job := &batchv1.Job{}
	err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, job)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, "Job not found"
		}
		logger.Error(err, "Failed to get Job", "name", name)
		return false, fmt.Sprintf("Error checking Job: %v", err)
	}

	// Check if Job has completed successfully
	if job.Status.Succeeded > 0 {
		return true, "Job completed successfully"
	}

	// Check if Job has failed
	if job.Status.Failed > 0 {
		return false, fmt.Sprintf("Job failed (%d failures)", job.Status.Failed)
	}

	// Job is still running
	return false, fmt.Sprintf("Job in progress (active: %d)", job.Status.Active)
}

// isTempoStackReady checks if TempoStack managed pods are ready
func (r *AIObservabilitySummarizerReconciler) isTempoStackReady(ctx context.Context, namespace, namePrefix string) (bool, string) {
	// Check TempoStack CR exists
	tempoStack := &unstructured.Unstructured{}
	tempoStack.SetAPIVersion("tempo.grafana.com/v1alpha1")
	tempoStack.SetKind("TempoStack")
	err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: namePrefix + "stack"}, tempoStack)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, "TempoStack CR not found"
		}
		return false, fmt.Sprintf("Error checking TempoStack: %v", err)
	}

	// Check if key Tempo pods are ready (ingester, querier, distributor)
	// TempoStack creates resources with pattern: {name}-tempostack-{component}
	stackName := namePrefix + "-tempostack"

	// Check ingester (StatefulSet)
	ingestReady, msg := r.isStatefulSetReady(ctx, namespace, stackName+"-ingester")
	if !ingestReady {
		return false, "Ingester: " + msg
	}

	// Check querier (Deployment)
	querierReady, msg := r.isDeploymentReady(ctx, namespace, stackName+"-querier")
	if !querierReady {
		return false, "Querier: " + msg
	}

	// Check distributor (Deployment)
	distReady, msg := r.isDeploymentReady(ctx, namespace, stackName+"-distributor")
	if !distReady {
		return false, "Distributor: " + msg
	}

	return true, "All Tempo components ready"
}

// isLokiStackReady checks if LokiStack managed pods are ready
func (r *AIObservabilitySummarizerReconciler) isLokiStackReady(ctx context.Context, namespace, name string) (bool, string) {
	// Check LokiStack CR exists
	lokiStack := &unstructured.Unstructured{}
	lokiStack.SetAPIVersion("loki.grafana.com/v1")
	lokiStack.SetKind("LokiStack")
	err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, lokiStack)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, "LokiStack CR not found"
		}
		return false, fmt.Sprintf("Error checking LokiStack: %v", err)
	}

	// Check if key Loki pods are ready (ingester, querier, distributor)
	// LokiStack creates resources with pattern: {name}-{component}

	// Check ingester (StatefulSet)
	ingestReady, msg := r.isStatefulSetReady(ctx, namespace, name+"-ingester")
	if !ingestReady {
		return false, "Ingester: " + msg
	}

	// Check querier (Deployment)
	querierReady, msg := r.isDeploymentReady(ctx, namespace, name+"-querier")
	if !querierReady {
		return false, "Querier: " + msg
	}

	// Check distributor (Deployment)
	distReady, msg := r.isDeploymentReady(ctx, namespace, name+"-distributor")
	if !distReady {
		return false, "Distributor: " + msg
	}

	return true, "All Loki components ready"
}

// enableUserWorkloadMonitoring enables OpenShift user workload monitoring
// by creating/updating the cluster-monitoring-config ConfigMap in openshift-monitoring namespace
func (r *AIObservabilitySummarizerReconciler) enableUserWorkloadMonitoring(ctx context.Context) error {
	logger := log.FromContext(ctx)

	const (
		cmName      = "cluster-monitoring-config"
		cmNamespace = "openshift-monitoring"
		configKey   = "config.yaml"
	)

	// Check if ConfigMap already exists
	existingCM := &corev1.ConfigMap{}
	err := r.Get(ctx, client.ObjectKey{Namespace: cmNamespace, Name: cmName}, existingCM)

	if err == nil {
		// ConfigMap exists, check if enableUserWorkload is already set
		if configYAML, ok := existingCM.Data[configKey]; ok {
			if strings.Contains(configYAML, "enableUserWorkload: true") {
				logger.Info("User workload monitoring already enabled")
				return nil
			}
		}
		// Update existing ConfigMap
		logger.Info("Updating cluster-monitoring-config to enable user workload monitoring")
		if existingCM.Data == nil {
			existingCM.Data = make(map[string]string)
		}
		existingCM.Data[configKey] = "enableUserWorkload: true\n"
		if err := r.Update(ctx, existingCM); err != nil {
			return fmt.Errorf("failed to update cluster-monitoring-config: %w", err)
		}
		logger.Info("Successfully enabled user workload monitoring")
		return nil
	}

	if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get cluster-monitoring-config: %w", err)
	}

	// ConfigMap doesn't exist, create it
	logger.Info("Creating cluster-monitoring-config to enable user workload monitoring")
	newCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: cmNamespace,
		},
		Data: map[string]string{
			configKey: "enableUserWorkload: true\n",
		},
	}

	if err := r.Create(ctx, newCM); err != nil {
		return fmt.Errorf("failed to create cluster-monitoring-config: %w", err)
	}

	logger.Info("Successfully enabled user workload monitoring")
	return nil
}

// enableConsolePlugin enables an OpenShift console plugin by adding it to the Console CR
func (r *AIObservabilitySummarizerReconciler) enableConsolePlugin(ctx context.Context, pluginName string) error {
	logger := log.FromContext(ctx)

	// Get the Console CR
	console := &unstructured.Unstructured{}
	console.SetAPIVersion("operator.openshift.io/v1")
	console.SetKind("Console")
	console.SetName("cluster")

	if err := r.Get(ctx, client.ObjectKey{Name: "cluster"}, console); err != nil {
		return fmt.Errorf("failed to get Console CR: %w", err)
	}

	// Get existing plugins
	existingPlugins, found, err := unstructured.NestedStringSlice(console.Object, "spec", "plugins")
	if err != nil {
		return fmt.Errorf("failed to get existing plugins: %w", err)
	}

	if !found {
		existingPlugins = []string{}
	}

	// Check if plugin is already enabled
	for _, plugin := range existingPlugins {
		if plugin == pluginName {
			logger.Info("Console plugin already enabled", "plugin", pluginName)
			return nil
		}
	}

	// Add plugin to the list
	existingPlugins = append(existingPlugins, pluginName)
	if err := unstructured.SetNestedStringSlice(console.Object, existingPlugins, "spec", "plugins"); err != nil {
		return fmt.Errorf("failed to set plugins: %w", err)
	}

	// Update the Console CR
	if err := r.Update(ctx, console); err != nil {
		return fmt.Errorf("failed to update Console CR: %w", err)
	}

	logger.Info("Successfully enabled console plugin", "plugin", pluginName)
	return nil
}

// setupAutoInstrumentation creates an Instrumentation CR for Python auto-instrumentation
func (r *AIObservabilitySummarizerReconciler) setupAutoInstrumentation(ctx context.Context, aiobs *obsv1alpha1.AIObservabilitySummarizer) error {
	logger := log.FromContext(ctx)

	// Determine OTel Collector namespace
	otelNamespace := "observability-hub" // default
	if aiobs.Spec.ObservabilityStack != nil && aiobs.Spec.ObservabilityStack.OTelCollector != nil && aiobs.Spec.ObservabilityStack.OTelCollector.Namespace != "" {
		otelNamespace = aiobs.Spec.ObservabilityStack.OTelCollector.Namespace
	}

	// Create Instrumentation CR in the application namespace
	instrumentation := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "opentelemetry.io/v1alpha1",
			"kind":       "Instrumentation",
			"metadata": map[string]interface{}{
				"name":      "python-instrumentation",
				"namespace": aiobs.Spec.Namespace,
			},
			"spec": map[string]interface{}{
				"exporter": map[string]interface{}{
					"endpoint": fmt.Sprintf("http://otel-collector-collector.%s.svc.cluster.local:4318", otelNamespace),
				},
				"propagators": []string{
					"tracecontext",
					"baggage",
					"b3",
				},
				"python": map[string]interface{}{
					"image": "ghcr.io/open-telemetry/opentelemetry-operator/autoinstrumentation-python:latest",
					"env": []map[string]interface{}{
						{
							"name":  "OTEL_PYTHON_PLATFORM",
							"value": "glibc",
						},
					},
				},
			},
		},
	}

	// Check if Instrumentation already exists
	existing := &unstructured.Unstructured{}
	existing.SetAPIVersion("opentelemetry.io/v1alpha1")
	existing.SetKind("Instrumentation")
	err := r.Get(ctx, client.ObjectKey{
		Namespace: aiobs.Spec.Namespace,
		Name:      "python-instrumentation",
	}, existing)

	if err == nil {
		// Instrumentation already exists
		logger.Info("Python auto-instrumentation already configured", "namespace", aiobs.Spec.Namespace)
		return nil
	}

	if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check for existing Instrumentation: %w", err)
	}

	// Create the Instrumentation CR
	logger.Info("Creating Python auto-instrumentation", "namespace", aiobs.Spec.Namespace)
	if err := r.Create(ctx, instrumentation); err != nil {
		return fmt.Errorf("failed to create Instrumentation CR: %w", err)
	}

	logger.Info("Successfully configured Python auto-instrumentation", "namespace", aiobs.Spec.Namespace)
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AIObservabilitySummarizerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&obsv1alpha1.AIObservabilitySummarizer{}).
		Complete(r)
}
