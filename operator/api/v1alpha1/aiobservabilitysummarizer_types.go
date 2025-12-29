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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AIObservabilitySummarizerSpec defines the desired state of AIObservabilitySummarizer
type AIObservabilitySummarizerSpec struct {
	// Namespace is the target namespace where components will be deployed
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Namespace string `json:"namespace"`

	// ImageRegistry is the container registry for component images
	// +kubebuilder:default="quay.io/ecosystem-appeng"
	// +optional
	ImageRegistry string `json:"imageRegistry,omitempty"`

	// ImageVersion is the version tag for component images
	// +kubebuilder:default="1.0.7"
	// +optional
	ImageVersion string `json:"imageVersion,omitempty"`

	// RAG contains inline RAG backend configuration
	// +optional
	RAG *RAGConfig `json:"rag,omitempty"`

	// RAGBackendRef references an external RAGBackend CR
	// +optional
	RAGBackendRef *corev1.LocalObjectReference `json:"ragBackendRef,omitempty"`

	// MCPServer contains inline MCP Server configuration
	// +optional
	MCPServer *MCPServerConfig `json:"mcpServer,omitempty"`

	// MCPServerRef references an external MCPServer CR
	// +optional
	MCPServerRef *corev1.LocalObjectReference `json:"mcpServerRef,omitempty"`

	// ConsolePlugin contains inline Console Plugin configuration
	// +optional
	ConsolePlugin *ConsolePluginConfig `json:"consolePlugin,omitempty"`

	// ConsolePluginRef references an external ConsolePlugin CR
	// +optional
	ConsolePluginRef *corev1.LocalObjectReference `json:"consolePluginRef,omitempty"`

	// ObservabilityStack contains inline Observability Stack configuration
	// +optional
	ObservabilityStack *ObservabilityStackConfig `json:"observabilityStack,omitempty"`

	// ObservabilityStackRef references an external ObservabilityStack CR
	// +optional
	ObservabilityStackRef *corev1.LocalObjectReference `json:"observabilityStackRef,omitempty"`

	// SelfHealing enables automatic remediation of degraded components
	// +kubebuilder:default=true
	// +optional
	SelfHealing bool `json:"selfHealing,omitempty"`

	// EnableTracing enables distributed tracing with OpenTelemetry
	// +kubebuilder:default=true
	// +optional
	EnableTracing bool `json:"enableTracing,omitempty"`

	// EnableLogging enables centralized logging with Loki
	// +kubebuilder:default=true
	// +optional
	EnableLogging bool `json:"enableLogging,omitempty"`
}

// RAGConfig defines the RAG backend configuration
type RAGConfig struct {
	// Enabled determines if RAG backend should be deployed
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	// Model is the LLM model identifier (e.g., llama-3-1-8b-instruct)
	// +kubebuilder:validation:Required
	Model string `json:"model"`

	// ExternalLLMURL is the URL of an external LLM service
	// +optional
	ExternalLLMURL string `json:"externalLLMURL,omitempty"`

	// HFTokenSecret is the name of the secret containing Hugging Face token
	// +optional
	HFTokenSecret string `json:"hfTokenSecret,omitempty"`

	// PostgreSQL contains PostgreSQL configuration
	// +optional
	PostgreSQL *PostgreSQLConfig `json:"postgresql,omitempty"`

	// MinIO contains MinIO configuration
	// +optional
	MinIO *MinIOConfig `json:"minio,omitempty"`

	// GPUTolerations for GPU node scheduling
	// +optional
	GPUTolerations []corev1.Toleration `json:"gpuTolerations,omitempty"`

	// Resources for RAG components
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// PostgreSQLConfig defines PostgreSQL configuration
type PostgreSQLConfig struct {
	// StorageSize is the PVC size for PostgreSQL
	// +kubebuilder:default="50Gi"
	// +optional
	StorageSize string `json:"storageSize,omitempty"`

	// User is the PostgreSQL username
	// +kubebuilder:default="postgres"
	// +optional
	User string `json:"user,omitempty"`

	// PasswordSecret is the name of the secret containing PostgreSQL password
	// +optional
	PasswordSecret string `json:"passwordSecret,omitempty"`

	// Database is the PostgreSQL database name
	// +kubebuilder:default="rag_blueprint"
	// +optional
	Database string `json:"database,omitempty"`
}

// MinIOConfig defines MinIO configuration
type MinIOConfig struct {
	// StorageSize is the PVC size for MinIO
	// +kubebuilder:default="100Gi"
	// +optional
	StorageSize string `json:"storageSize,omitempty"`

	// AccessKeySecret is the name of the secret containing MinIO access key
	// +optional
	AccessKeySecret string `json:"accessKeySecret,omitempty"`

	// Buckets is a list of bucket names to create
	// +optional
	Buckets []string `json:"buckets,omitempty"`
}

// MCPServerConfig defines MCP Server configuration
type MCPServerConfig struct {
	// Enabled determines if MCP Server should be deployed
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	// Replicas is the number of MCP Server replicas
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Resources for MCP Server
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// RouteHost is the custom hostname for the Route
	// +optional
	RouteHost string `json:"routeHost,omitempty"`
}

// ConsolePluginConfig defines Console Plugin configuration
type ConsolePluginConfig struct {
	// Enabled determines if Console Plugin should be deployed
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	// AutoEnable automatically enables the plugin in OpenShift Console
	// +kubebuilder:default=true
	// +optional
	AutoEnable bool `json:"autoEnable,omitempty"`

	// Resources for Console Plugin
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// ObservabilityStackConfig defines Observability Stack configuration
type ObservabilityStackConfig struct {
	// Enabled determines if Observability Stack should be deployed
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	// MinIO contains observability MinIO configuration
	// +optional
	MinIO *ObservabilityMinIOConfig `json:"minio,omitempty"`

	// Tempo contains Tempo configuration
	// +optional
	Tempo *TempoConfig `json:"tempo,omitempty"`

	// Loki contains Loki configuration
	// +optional
	Loki *LokiConfig `json:"loki,omitempty"`

	// OTelCollector contains OpenTelemetry Collector configuration
	// +optional
	OTelCollector *OTelCollectorConfig `json:"otelCollector,omitempty"`
}

// ObservabilityMinIOConfig defines MinIO configuration for observability
type ObservabilityMinIOConfig struct {
	// StorageSize is the PVC size for observability MinIO
	// +kubebuilder:default="500Gi"
	// +optional
	StorageSize string `json:"storageSize,omitempty"`

	// Namespace is the namespace for MinIO deployment
	// +kubebuilder:default="observability-hub"
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// TempoConfig defines Tempo configuration
type TempoConfig struct {
	// Retention is the trace retention period (e.g., "7d", "30d")
	// +kubebuilder:default="7d"
	// +optional
	Retention string `json:"retention,omitempty"`

	// Namespace is the namespace for Tempo deployment
	// +kubebuilder:default="observability-hub"
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// LokiConfig defines Loki configuration
type LokiConfig struct {
	// Retention is the log retention period (e.g., "7d", "30d", "90d")
	// +kubebuilder:default="30d"
	// +optional
	Retention string `json:"retention,omitempty"`

	// Namespace is the namespace for Loki deployment
	// +kubebuilder:default="openshift-logging"
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// OTelCollectorConfig defines OpenTelemetry Collector configuration
type OTelCollectorConfig struct {
	// Namespace is the namespace for OTel Collector deployment
	// +kubebuilder:default="observability-hub"
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// AIObservabilitySummarizerStatus defines the observed state of AIObservabilitySummarizer
type AIObservabilitySummarizerStatus struct {
	// Phase represents the current phase of the deployment
	// +kubebuilder:validation:Enum=Pending;Installing;Ready;Failed;Upgrading
	// +optional
	Phase string `json:"phase,omitempty"`

	// ObservedGeneration reflects the generation of the most recently observed spec
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent the latest available observations of the object's state
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// Components contains the status of each component
	// +optional
	Components ComponentsStatus `json:"components,omitempty"`

	// InstalledOperators tracks the installed OpenShift operators and their versions
	// +optional
	InstalledOperators map[string]string `json:"installedOperators,omitempty"`

	// Health contains the overall health status
	// +optional
	Health HealthStatus `json:"health,omitempty"`

	// LastReconcileTime is the last time the operator reconciled this resource
	// +optional
	LastReconcileTime *metav1.Time `json:"lastReconcileTime,omitempty"`
}

// ComponentsStatus tracks the status of all components
type ComponentsStatus struct {
	// RAG contains RAG backend status
	// +optional
	RAG ComponentStatus `json:"rag,omitempty"`

	// MCPServer contains MCP Server status
	// +optional
	MCPServer ComponentStatus `json:"mcpServer,omitempty"`

	// ConsolePlugin contains Console Plugin status
	// +optional
	ConsolePlugin ComponentStatus `json:"consolePlugin,omitempty"`

	// UI contains UI status
	// +optional
	UI ComponentStatus `json:"ui,omitempty"`

	// MinIO contains MinIO status
	// +optional
	MinIO ComponentStatus `json:"minio,omitempty"`

	// Tempo contains Tempo status
	// +optional
	Tempo ComponentStatus `json:"tempo,omitempty"`

	// Loki contains Loki status
	// +optional
	Loki ComponentStatus `json:"loki,omitempty"`

	// OTelCollector contains OpenTelemetry Collector status
	// +optional
	OTelCollector ComponentStatus `json:"otelCollector,omitempty"`

	// Korrel8r contains Korrel8r status
	// +optional
	Korrel8r ComponentStatus `json:"korrel8r,omitempty"`
}

// ComponentStatus represents the status of a single component
type ComponentStatus struct {
	// State is the current state of the component
	// +kubebuilder:validation:Enum=Pending;Installing;Ready;Degraded;Failed
	// +optional
	State string `json:"state,omitempty"`

	// Message provides additional information about the component state
	// +optional
	Message string `json:"message,omitempty"`

	// HelmRelease is the name of the Helm release managing this component
	// +optional
	HelmRelease string `json:"helmRelease,omitempty"`

	// Version is the deployed version of the component
	// +optional
	Version string `json:"version,omitempty"`

	// LastUpdateTime is the last time this component was updated
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`
}

// HealthStatus represents the overall health status
type HealthStatus struct {
	// Overall is the overall health state
	// +kubebuilder:validation:Enum=Healthy;Degraded;Unhealthy
	// +optional
	Overall string `json:"overall,omitempty"`

	// Components contains health status for each component
	// +optional
	Components map[string]ComponentHealth `json:"components,omitempty"`

	// LastCheckTime is the last time health checks were performed
	// +optional
	LastCheckTime *metav1.Time `json:"lastCheckTime,omitempty"`
}

// ComponentHealth represents the health of a single component
type ComponentHealth struct {
	// Status is the health status
	// +kubebuilder:validation:Enum=Healthy;Degraded;Unhealthy
	Status string `json:"status,omitempty"`

	// Message provides details about the health status
	// +optional
	Message string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Health",type=string,JSONPath=`.status.health.overall`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:resource:shortName=aiobs

// AIObservabilitySummarizer is the Schema for the aiobservabilitysummarizers API
type AIObservabilitySummarizer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AIObservabilitySummarizerSpec   `json:"spec,omitempty"`
	Status AIObservabilitySummarizerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AIObservabilitySummarizerList contains a list of AIObservabilitySummarizer
type AIObservabilitySummarizerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AIObservabilitySummarizer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AIObservabilitySummarizer{}, &AIObservabilitySummarizerList{})
}
