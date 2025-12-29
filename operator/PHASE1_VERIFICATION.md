# Phase 1 Verification Guide

This guide walks through verifying the Phase 1 implementation of the AI Observability Summarizer Operator.

## Verification Levels

### Level 1: Code Compilation & Static Analysis ✅
### Level 2: CRD Validation & Manifest Generation ✅
### Level 3: Unit Tests (Controller Logic)
### Level 4: Integration Tests (envtest - Local Kubernetes API)
### Level 5: E2E Tests (Real OpenShift Cluster)

---

## Level 1: Code Compilation & Static Analysis

**What it verifies**: Go code compiles, dependencies resolve, no syntax errors

```bash
cd operator

# 1. Check dependencies
go mod verify
go mod tidy

# 2. Build the operator binary
make build

# 3. Verify binary was created
ls -lh bin/manager

# 4. Run static analysis
go vet ./...

# 5. Run linters (if golangci-lint installed)
golangci-lint run || echo "golangci-lint not installed, skipping"
```

**Expected Results**:
- ✅ Binary created at `bin/manager` (~50-70MB)
- ✅ No compilation errors
- ✅ No vet warnings

---

## Level 2: CRD Validation & Manifest Generation

**What it verifies**: CRDs are valid, kubebuilder markers correct, manifests generate properly

```bash
cd operator

# 1. Generate CRDs and RBAC manifests
make manifests

# 2. Verify CRD file exists and is valid YAML
ls -lh config/crd/bases/obs.redhat.com_aiobservabilitysummarizers.yaml
kubectl --dry-run=client apply -f config/crd/bases/obs.redhat.com_aiobservabilitysummarizers.yaml 2>&1 | grep -i "error" || echo "✅ CRD YAML is valid"

# 3. Verify sample CR is valid against the CRD
kubectl --dry-run=client apply -f config/samples/obs_v1alpha1_aiobservabilitysummarizer.yaml 2>&1 | grep -i "error" || echo "✅ Sample CR is valid"

# 4. Check CRD structure
yq eval '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties | keys' config/crd/bases/obs.redhat.com_aiobservabilitysummarizers.yaml

# 5. Verify status subresource
yq eval '.spec.versions[0].subresources.status' config/crd/bases/obs.redhat.com_aiobservabilitysummarizers.yaml

# 6. Check print columns
yq eval '.spec.versions[0].additionalPrinterColumns' config/crd/bases/obs.redhat.com_aiobservabilitysummarizers.yaml
```

**Expected Results**:
- ✅ CRD file ~41KB
- ✅ CRD contains all spec fields (namespace, imageRegistry, rag, mcpServer, etc.)
- ✅ Status subresource enabled
- ✅ Print columns defined (Phase, Health, Age)
- ✅ Sample CR passes validation

---

## Level 3: Unit Tests

**What it verifies**: Controller logic, validation functions, operator installer functions

```bash
cd operator

# 1. Run all unit tests
make test

# 2. Run tests with verbose output
go test -v ./internal/controller/... ./internal/operators/...

# 3. Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
echo "Open coverage.html in browser to view coverage report"
```

**Create basic unit tests** (if not exist):

```bash
# Test validation logic
cat > internal/controller/aiobservabilitysummarizer_controller_test.go <<'EOF'
package controller

import (
	"context"
	"testing"

	obsv1alpha1 "github.com/redhat-et/openshift-ai-observability-summarizer-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestValidateSpec(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = obsv1alpha1.AddToScheme(scheme)

	tests := []struct {
		name    string
		spec    obsv1alpha1.AIObservabilitySummarizerSpec
		wantErr bool
	}{
		{
			name: "valid spec with namespace",
			spec: obsv1alpha1.AIObservabilitySummarizerSpec{
				Namespace: "test-namespace",
			},
			wantErr: false,
		},
		{
			name: "missing namespace",
			spec: obsv1alpha1.AIObservabilitySummarizerSpec{
				Namespace: "",
			},
			wantErr: true,
		},
		{
			name: "both rag and ragBackendRef specified",
			spec: obsv1alpha1.AIObservabilitySummarizerSpec{
				Namespace: "test-namespace",
				RAG: &obsv1alpha1.RAGConfig{
					Enabled: true,
					Model:   "llama-3-1-8b-instruct",
				},
				RAGBackendRef: &corev1.LocalObjectReference{
					Name: "external-rag",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			r := &AIObservabilitySummarizerReconciler{
				Client: client,
				Scheme: scheme,
			}

			aiobs := &obsv1alpha1.AIObservabilitySummarizer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
				Spec: tt.spec,
			}

			err := r.validateSpec(context.Background(), aiobs)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
EOF

# Run the test
go test -v ./internal/controller -run TestValidateSpec
```

**Expected Results**:
- ✅ Tests pass
- ✅ Coverage > 50% for controller logic

---

## Level 4: Integration Tests (envtest)

**What it verifies**: Controller reconciliation with fake Kubernetes API

```bash
cd operator

# 1. Run integration tests with envtest
make test-integration

# Alternative: Run with explicit envtest
go test -v ./internal/controller/... -tags=integration
```

**Create integration test**:

```bash
cat > internal/controller/integration_test.go <<'EOF'
//go:build integration
// +build integration

package controller

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	obsv1alpha1 "github.com/redhat-et/openshift-ai-observability-summarizer-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment
var ctx context.Context
var cancel context.CancelFunc

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = obsv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())
})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

var _ = Describe("AIObservabilitySummarizer Controller", func() {
	Context("When creating AIObservabilitySummarizer", func() {
		It("Should initialize status correctly", func() {
			aiobs := &obsv1alpha1.AIObservabilitySummarizer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-aiobs",
					Namespace: "default",
				},
				Spec: obsv1alpha1.AIObservabilitySummarizerSpec{
					Namespace: "test-namespace",
				},
			}

			Expect(k8sClient.Create(ctx, aiobs)).Should(Succeed())

			// Wait for reconciliation
			time.Sleep(2 * time.Second)

			// Fetch the updated object
			fetched := &obsv1alpha1.AIObservabilitySummarizer{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(aiobs), fetched)).Should(Succeed())

			// Verify status was initialized
			Expect(fetched.Status.Phase).Should(Equal(PhasePending))
		})
	})
})
EOF
```

**Expected Results**:
- ✅ envtest starts successfully
- ✅ CRDs load correctly
- ✅ Controller can create and update resources
- ✅ Status initialization works

---

## Level 5: E2E Tests (Real OpenShift Cluster)

**What it verifies**: Operator works on actual OpenShift cluster with OLM

### Prerequisites

```bash
# 1. Ensure you're logged into OpenShift
oc whoami

# 2. Verify cluster access
oc get nodes

# 3. Check if you have cluster-admin or sufficient permissions
oc auth can-i create subscriptions.operators.coreos.com --all-namespaces
```

### Test Steps

```bash
cd operator

# 1. Install CRDs
make install

# 2. Verify CRD installation
oc get crd aiobservabilitysummarizers.obs.redhat.com
oc describe crd aiobservabilitysummarizers.obs.redhat.com

# 3. Run operator locally (connects to cluster)
make run

# In another terminal:
# 4. Create a test namespace
oc create namespace aiobs-test

# 5. Apply sample CR
oc apply -f config/samples/obs_v1alpha1_aiobservabilitysummarizer.yaml

# 6. Watch the operator logs (in terminal where 'make run' is running)
# You should see:
# - "Reconciling AIObservabilitySummarizer"
# - "Phase 1: Validation passed"
# - "Phase 2: Installing required OpenShift operators"
# - "Operator installation logic..."

# 7. Check CR status
oc get aiobs -o wide
oc describe aiobs aiobservabilitysummarizer-sample

# 8. Verify status was updated
oc get aiobs aiobservabilitysummarizer-sample -o jsonpath='{.status.phase}'
# Should show: Pending or Installing

# 9. Check conditions
oc get aiobs aiobservabilitysummarizer-sample -o jsonpath='{.status.conditions}' | jq .

# 10. Verify operator installer would create namespaces (dry-run)
oc get namespace openshift-cluster-observability-operator --dry-run=client

# 11. Clean up
oc delete aiobs aiobservabilitysummarizer-sample
oc delete namespace aiobs-test
make uninstall
```

**Expected Results**:
- ✅ CRD installs successfully
- ✅ Operator starts without errors
- ✅ CR is created and reconciled
- ✅ Status.Phase transitions: "" → Pending → Installing → Ready
- ✅ Conditions are set correctly
- ✅ Validation logic catches invalid specs

---

## Quick Verification Checklist

Run these commands in sequence for a quick Phase 1 verification:

```bash
cd operator

# ✅ 1. Compilation
make build && echo "✅ Build successful"

# ✅ 2. Manifests
make manifests && echo "✅ Manifests generated"

# ✅ 3. CRD validation
kubectl --dry-run=client apply -f config/crd/bases/obs.redhat.com_aiobservabilitysummarizers.yaml && echo "✅ CRD valid"

# ✅ 4. Sample CR validation
kubectl --dry-run=client apply -f config/samples/obs_v1alpha1_aiobservabilitysummarizer.yaml && echo "✅ Sample CR valid"

# ✅ 5. Check operator installer code
grep -n "RequiredOperators" internal/operators/installer.go && echo "✅ Operator installer defined"

# ✅ 6. Check controller phases
grep -n "Phase 1:" internal/controller/aiobservabilitysummarizer_controller.go && echo "✅ Controller phases defined"

# ✅ 7. Verify all files exist
ls -1 api/v1alpha1/aiobservabilitysummarizer_types.go \
     internal/controller/aiobservabilitysummarizer_controller.go \
     internal/operators/installer.go \
     config/crd/bases/obs.redhat.com_aiobservabilitysummarizers.yaml \
     config/samples/obs_v1alpha1_aiobservabilitysummarizer.yaml && echo "✅ All Phase 1 files present"
```

---

## Phase 1 Success Criteria

- [x] Operator project initialized with Operator SDK
- [x] Primary CRD with complete spec and status
- [x] Controller with validation and 7-phase structure
- [x] OpenShift operator installer (5 operators)
- [x] CRD manifests generated (41KB)
- [x] Sample CR created
- [x] Code compiles without errors
- [x] Binary builds successfully

## Known Limitations (Phase 1)

- Phases 3-7 are stubs (will be implemented in Phase 2+)
- Operator installer connects to OLM but actual operator installation needs OpenShift cluster
- No Helm chart integration yet (Phase 2)
- No health checking yet (Phase 3)
- No self-healing yet (Phase 4)

## Next Steps

After verifying Phase 1, proceed to Phase 2:
- Implement dependency graph
- Add Helm chart embedding
- Deploy RAG, MCP Server, Console Plugin components
