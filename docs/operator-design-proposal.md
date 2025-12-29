# Golang-based Operator for OpenShift AI Observability Summarizer

## Overview

Design and implement a production-ready Kubernetes operator using **Operator SDK with native Go controllers** to manage the complete lifecycle of the OpenShift AI Observability Summarizer platform, including RAG backend, MCP Server, Console PlugIn, and full observability stack (MinIO, Tempo, Loki, Korrel8r, OTEL).

## Design Decisions

### Architecture
- **Framework**: Operator SDK with native Go controllers (not Helm-based)
- **CRD Structure**: Hybrid approach with primary `AIObservabilitySummarizer` CRD + optional component-specific CRDs (`RAGBackend`, `MCPServer`, `ConsolePlugin`, `ObservabilityStack`)
- **Deployment Strategy**: Leverage existing Helm charts via `helm.sh/helm/v3` Go libraries (embedded in operator binary)
- **Operator Maturity Level**: Target Level 4 (Auto-pilot) with health monitoring and self-healing

### Scope
Operator manages:
1. **Application Components**: RAG, MCP Server, Console PlugIn, UI, Alerting
2. **Observability Stack**: MinIO, TempoStack, LokiStack, Korrel8r, OpenTelemetry Collector
3. **Required Operators**: Auto-install Cluster Observability, OpenTelemetry, Tempo, Logging, and Loki operators via OLM
4. **Health & Self-Healing**: Continuous monitoring with automatic remediation

## Project Structure

```
openshift-ai-observability-summarizer/
├── operator/                          # New operator root (separate from app code)
│   ├── api/v1alpha1/                 # CRD definitions
│   │   ├── aiobservabilitysummarizer_types.go  # Primary CRD
│   │   ├── ragbackend_types.go                  # Optional component CRDs
│   │   ├── mcpserver_types.go
│   │   ├── consoleplugin_types.go
│   │   └── observabilitystack_types.go
│   │
│   ├── controllers/                   # Reconciliation controllers
│   │   ├── aiobservabilitysummarizer_controller.go  # Main orchestrator
│   │   └── component controllers (rag, mcp, plugin, observability)
│   │
│   ├── internal/                      # Internal packages
│   │   ├── helm/                      # Helm chart management
│   │   │   ├── chart_manager.go      # Load and render embedded charts
│   │   │   └── release_manager.go    # Helm release lifecycle
│   │   ├── health/                    # Health checking framework
│   │   │   ├── checker.go            # Orchestrator
│   │   │   └── component checks (postgres, minio, service)
│   │   ├── operators/                 # OpenShift operator installer
│   │   │   ├── installer.go          # Install via OLM
│   │   │   └── watcher.go            # Monitor CSV status
│   │   └── dependencies/              # Dependency graph
│   │       ├── graph.go              # DAG implementation
│   │       └── resolver.go           # Dependency resolution
│   │
│   ├── helm-charts/                   # Embedded Helm charts (go:embed)
│   │   ├── rag/
│   │   ├── mcp-server/
│   │   ├── console-plugin/
│   │   ├── minio/
│   │   ├── tempo/
│   │   ├── loki/
│   │   ├── korrel8r/
│   │   └── otel-collector/
│   │
│   └── config/                        # Kustomize configs (generated)
│       ├── crd/bases/
│       ├── rbac/
│       └── manager/
│
├── deploy/helm/                       # Existing charts (backwards compat)
└── Makefile                           # Root Makefile (supports both)
```

## CRD Design

### Primary CRD: AIObservabilitySummarizer (obs.redhat.com/v1alpha1)

**Spec Structure**:
```go
type AIObservabilitySummarizerSpec struct {
    Namespace     string  // Target namespace
    ImageRegistry string  // Default: quay.io/ecosystem-appeng
    ImageVersion  string  // Default: 1.0.7

    // Component configuration (inline or via reference)
    RAG                  *RAGConfig
    RAGBackendRef        *LocalObjectReference

    MCPServer            *MCPServerConfig
    MCPServerRef         *LocalObjectReference

    ConsolePlugin        *ConsolePluginConfig
    ConsolePluginRef     *LocalObjectReference

    ObservabilityStack   *ObservabilityStackConfig
    ObservabilityStackRef *LocalObjectReference

    // Feature flags
    SelfHealing   bool  // Default: true
    EnableTracing bool  // Default: true
    EnableLogging bool  // Default: true
}
```

**Status Structure**:
```go
type AIObservabilitySummarizerStatus struct {
    Phase              string             // Pending, Installing, Ready, Failed, Upgrading
    ObservedGeneration int64
    Conditions         []metav1.Condition // Ready, OperatorsInstalled, Degraded, etc.

    Components         ComponentsStatus   // Per-component state
    InstalledOperators map[string]string  // Operator name -> CSV version
    Health             HealthStatus       // Health check results
    LastReconcileTime  *metav1.Time
}
```

**Usage Patterns**:
1. **All-in-One**: Single CR with inline component configs (recommended)
2. **Hybrid**: Main CR references external component CRs for granular control

### Optional Component CRDs

For advanced users requiring fine-grained control:
- `RAGBackend`: Independent RAG configuration
- `MCPServer`: Standalone MCP server
- `ConsolePlugin`: Console plugin management
- `ObservabilityStack`: Shared observability infrastructure

## Controller Architecture

### Main Controller Reconciliation Flow

```
Reconcile Trigger
      │
      ├─→ Phase 1: Validation
      │   └─ Validate spec, namespace, secrets
      │
      ├─→ Phase 2: Install Required Operators
      │   └─ Cluster Observability, OpenTelemetry, Tempo, Logging, Loki
      │   └─ Wait for CSV phase = Succeeded
      │
      ├─→ Phase 3: Build Dependency Graph
      │   └─ DAG: ObsStack → RAG → MCPServer → ConsolePlugin → UI
      │
      ├─→ Phase 4: Deploy Components (via Helm)
      │   └─ For each component in dependency order:
      │       - Wait for dependencies to reach Ready
      │       - Render Helm chart with dynamic values
      │       - Apply resources
      │       - Update component status
      │
      ├─→ Phase 5: Post-Install Configuration
      │   └─ Enable console plugin
      │   └─ Setup tracing instrumentation
      │   └─ Configure RBAC
      │
      ├─→ Phase 6: Health Checks
      │   └─ Run component health checks
      │   └─ Update health status
      │   └─ Set Ready condition
      │
      └─→ Phase 7: Self-Healing (if enabled)
          └─ Detect degraded components
          └─ Restart failed pods
          └─ Recreate missing resources
          └─ Requeue in 5 minutes
```

### Dependency Management

**Dependency Graph (DAG)**:
```
ObservabilityStack (Tier 0)
  ├── MinIO
  ├── Tempo
  ├── Loki
  ├── Korrel8r (depends on Tempo, Loki)
  └── OTel Collector
        ↓
RAG Backend (Tier 1)
  ├── PostgreSQL
  ├── llama-stack
  └── ingestion-pipeline
        ↓
MCP Server (Tier 2)
        ↓
Console Plugin (Tier 3)
        ↓
UI (Tier 3)
```

Components wait for dependencies to reach `Ready` state before deploying.

## Observability Stack Integration

### Required OpenShift Operators

Operator automatically installs via OLM:
1. Cluster Observability Operator → `openshift-cluster-observability-operator`
2. Red Hat OpenTelemetry Operator → `openshift-opentelemetry-operator`
3. Tempo Operator → `openshift-tempo-operator`
4. OpenShift Logging Operator → `openshift-logging`
5. Loki Operator → `openshift-operators-redhat`

**Implementation**: Create Subscription + OperatorGroup resources, poll CSV status until `phase: Succeeded`

### Observability Resources

- **MinIO**: Shared S3 storage with auto-created buckets (tempo, loki)
- **TempoStack**: CR deployed with MinIO backend configuration
- **LokiStack**: CR deployed with MinIO backend, configurable retention
- **Korrel8r**: Correlation engine for observability data, connects Tempo traces, Loki logs, and Prometheus metrics
- **OTel Collector**: Auto-configured to export traces to Tempo

### Korrel8r Integration

**Purpose**: Korrel8r provides correlation between different observability signals (traces, logs, metrics) enabling:
- Automatic linking from traces to related logs
- Cross-signal navigation in the Console Plugin
- Root cause analysis by correlating multiple data sources
- Enhanced troubleshooting workflow for AI model inference issues

**Deployment Configuration**:
- Deployed as a Deployment with Service and optional Route
- Configured with connection details for Tempo, Loki, and Prometheus
- Auto-generates correlation rules for OpenShift AI workloads
- Integrates with Console Plugin for seamless user experience

**Dependencies**:
- Requires TempoStack and LokiStack to be Ready
- Optionally integrates with Prometheus/Thanos for metrics correlation

## Health Monitoring & Self-Healing

### Health Checks (every 5 minutes)

**Per-Component Checks**:
- **PostgreSQL**: StatefulSet ready, PVC bound, pod readiness
- **MinIO**: Deployment ready, buckets exist, S3 API responds
- **llama-stack**: Deployment ready, HTTP /health endpoint
- **MCP Server**: Deployment ready, Service exists, Route accessible
- **Tempo**: TempoStack CR Ready condition = True
- **Loki**: LokiStack CR Ready condition = True
- **Korrel8r**: Deployment ready, HTTP API endpoint responding, connections to Tempo/Loki verified
- **OTel Collector**: Deployment ready, trace ingestion working

### Self-Healing Actions (when `selfHealing: true`)

**Unhealthy Component Remediation**:
- Pod CrashLoopBackOff → Delete pod to force recreation
- Deployment not ready → Trigger rollout restart
- Service missing → Recreate from Helm chart
- TempoStack degraded → Update CR to trigger reconciliation

**Status Updates**:
- Overall `Phase`: Pending, Installing, Ready, Failed, Upgrading
- Component `State`: Pending, Installing, Ready, Degraded, Failed
- Health `Overall`: Healthy, Degraded, Unhealthy
- Kubernetes Events for significant changes

## Deployment Strategy

### Helm Chart Reuse (Hybrid Approach)

**Rationale**: Leverage existing battle-tested Helm charts while maintaining operator control

**Implementation**:
1. Embed Helm charts in operator binary using `go:embed`
2. Use `helm.sh/helm/v3` libraries to render charts with dynamic values
3. Apply rendered manifests via controller-runtime
4. Track Helm releases in operator namespace for visibility

**Benefits**:
- Reuse existing charts (less code duplication)
- Easier maintenance
- Community-standard approach
- OLM bundle includes chart metadata

### Image Management

- Registry: Configurable via `spec.imageRegistry` (default: `quay.io/ecosystem-appeng`)
- Version: Configurable via `spec.imageVersion` (default: `1.0.7`)
- Components: `mcp-server`, `console-plugin`, `ui`, `alerting`
- Version tracking in `status.components.*.version`
- Support rolling upgrades with zero downtime

## Migration Path

### From Makefile/Helm to Operator

**Phase 1**: Operator detects and adopts existing Helm releases
**Phase 2**: Support both deployment methods in parallel
**Phase 3**: Provide migration CLI tool
**Phase 4**: Deprecation warnings in Makefile

**Migration Tool**:
```bash
# Generate CR from existing deployment
./operator-cli migrate --namespace aiobs-test --output aiobs-cr.yaml

# Apply CR (operator adopts resources)
oc apply -f aiobs-cr.yaml
```

**Compatibility**:
- Operator adopts existing Helm releases via labels/annotations
- Uses `ownerReferences` only for new resources
- Imports config from existing ConfigMaps/Secrets

## Testing Strategy

1. **Unit Tests** (80%+ coverage)
   - API validation, dependency graph, health checks, Helm templating

2. **Integration Tests** (envtest)
   - Controller reconciliation, Helm rendering, resource creation

3. **E2E Tests** (Real OpenShift 4.16+)
   - Full stack deployment, upgrades, self-healing, component interaction

## Implementation Phases

**Phase 1** (Weeks 1-4): Foundation
- Operator SDK scaffolding
- Primary CRD definition
- Basic controller with validation
- **OpenShift operator installer** (install Cluster Observability, OpenTelemetry, Tempo, Logging, Loki operators via OLM)

**Phase 2** (Weeks 5-8): Core Components
- Dependency graph implementation
- Helm chart embedding and rendering
- RAG, MCP Server, Console Plugin deployment

**Phase 3** (Weeks 9-12): Observability Stack
- MinIO, Tempo, Loki, OTel resource management
- Health check framework

**Phase 4** (Weeks 13-16): Advanced Features
- Self-healing implementation
- Optional component CRDs
- Migration tooling

**Phase 5** (Week 17+): Production Readiness
- OLM bundle creation
- Documentation
- Beta/GA launch

## Critical Files to Implement

**Phase 1: Foundation (implement first)**

1. `operator/api/v1alpha1/aiobservabilitysummarizer_types.go` (~500 lines)
   - Primary CRD spec/status definitions

2. `operator/controllers/aiobservabilitysummarizer_controller.go` (~800 lines)
   - Main reconciliation logic with 7-phase flow (initial skeleton)

3. `operator/internal/operators/installer.go` (~300 lines)
   - OpenShift operator installation via OLM (Subscription, OperatorGroup, CSV monitoring)

**Phase 2: Core Components**

4. `operator/internal/helm/chart_manager.go` (~400 lines)
   - Helm chart embedding, rendering, release management

5. `operator/internal/dependencies/graph.go` (~200 lines)
   - Dependency graph (DAG) implementation

**Phase 3: Observability Stack**

6. `operator/internal/health/checker.go` (~600 lines)
   - Health check framework for all components

These files establish the core architecture incrementally across phases.

## Benefits of This Approach

1. **User Experience**: Single CR deployment vs. complex Makefile commands
2. **Consistency**: Declarative, idempotent deployments
3. **Automation**: Auto-install dependencies, self-healing, health monitoring
4. **Flexibility**: Hybrid CRD structure supports simple and advanced use cases
5. **Standards**: OLM-ready, OperatorHub certification path
6. **Migration**: Smooth transition from existing Makefile/Helm deployments
7. **Observability**: Rich status reporting, events, and health metrics

## Example Usage

### Simple Deployment (All-in-One)

```yaml
apiVersion: obs.redhat.com/v1alpha1
kind: AIObservabilitySummarizer
metadata:
  name: aiobs-production
  namespace: openshift-ai-observability
spec:
  namespace: aiobs-prod
  imageVersion: "1.0.7"

  # RAG configuration
  rag:
    enabled: true
    model: llama-3-1-8b-instruct
    postgresql:
      storageSize: 50Gi
    minio:
      storageSize: 100Gi

  # MCP Server
  mcpServer:
    enabled: true
    replicas: 2

  # Console Plugin
  consolePlugin:
    enabled: true
    autoEnable: true

  # Observability stack
  observabilityStack:
    enabled: true
    tempo:
      retention: 7d
    loki:
      retention: 30d
    korrel8r:
      enabled: true

  # Feature flags
  selfHealing: true
  enableTracing: true
  enableLogging: true
```

### Advanced Deployment (Hybrid with Component CRs)

```yaml
---
# Shared observability stack
apiVersion: obs.redhat.com/v1alpha1
kind: ObservabilityStack
metadata:
  name: shared-observability
  namespace: observability-hub
spec:
  minio:
    storageSize: 500Gi
  tempo:
    retention: 30d
  loki:
    retention: 90d
  korrel8r:
    enabled: true
---
# Main application
apiVersion: obs.redhat.com/v1alpha1
kind: AIObservabilitySummarizer
metadata:
  name: aiobs-advanced
  namespace: openshift-ai-observability
spec:
  namespace: aiobs-prod

  # Reference shared observability stack
  observabilityStackRef:
    name: shared-observability

  # Inline RAG config
  rag:
    enabled: true
    model: llama-3-1-8b-instruct
    externalLLMURL: https://external-llm.example.com/v1

  mcpServer:
    enabled: true

  consolePlugin:
    enabled: true
```

## Next Steps

1. **Review & Approve**: Stakeholder review of this design document
2. **Initialize Operator**: Run `operator-sdk init --domain redhat.com --repo github.com/redhat/openshift-ai-observability-summarizer-operator`
3. **Scaffold CRDs**: Create API definitions with `operator-sdk create api`
4. **Implement Foundation**: Build the 5 critical files listed above
5. **Iterate**: Follow implementation phases 1-5
6. **Test**: Unit → Integration → E2E testing on OpenShift cluster
7. **Document**: User guide, operator installation, troubleshooting
8. **Publish**: OLM bundle, OperatorHub listing, release

## Questions for Review

1. Do we need any additional component-specific CRDs beyond the ones proposed?
2. Should the operator support air-gapped deployments (bundled images)?
3. What's the preferred namespace strategy: operator-managed or user-provided?
4. Should we support multi-cluster deployments in the future?
5. Any specific compliance requirements (FedRAMP, FIPS, etc.)?
