# Deploy Operator to OpenShift - Step by Step Guide

This guide walks through building and deploying the AI Observability Summarizer Operator to an OpenShift cluster using **podman** on **Mac**.

## Prerequisites

### 1. Install Podman on Mac

```bash
# Install podman via Homebrew
brew install podman

# Initialize and start podman machine (required on Mac)
podman machine init
podman machine start

# Verify podman is working
podman info
```

**Note:** Podman on Mac uses a VM (similar to Docker Desktop) to run containers since macOS doesn't have native container support.

### 2. Container Registry Access

You need push access to a container registry. Options:
- **Quay.io** (recommended for OpenShift)
- Docker Hub
- GitHub Container Registry (ghcr.io)
- OpenShift Internal Registry

#### Setup Quay.io (Recommended)

```bash
# Create account at https://quay.io
# Create a repository: quay.io/<your-org>/aiobs-operator

# Login to quay.io
podman login quay.io
# Enter username and password
```

### 3. OpenShift Cluster Access

```bash
# Login to your OpenShift cluster
oc login --server=https://api.your-cluster.com:6443 --token=<your-token>

# Verify access
oc whoami
oc get nodes

# Check cluster-admin or sufficient permissions
oc auth can-i create clusterroles
oc auth can-i create customresourcedefinitions
```

## Build & Deploy Workflow

### Step 1: Set Environment Variables

```bash
# Navigate to operator directory
cd operator

# Set your registry and version
export REGISTRY=quay.io
export ORG=<your-quay-username-or-org>
export VERSION=0.0.1
export IMG=${REGISTRY}/${ORG}/aiobs-operator:v${VERSION}

# Verify settings
echo "Image will be: ${IMG}"
```

### Step 2: Build Operator Image (AMD64 for OpenShift)

The Makefile is configured to build AMD64 images by default (compatible with OpenShift x86_64 clusters).

```bash
# Build operator image for AMD64 (even on Mac ARM)
make docker-build IMG=${IMG}

# This runs:
# podman build --platform=linux/amd64 -t ${IMG} .
```

**What happens:**
- Podman uses QEMU emulation to build AMD64 images on Mac ARM
- Multi-stage build creates minimal distroless image (~100MB)
- Image is tagged with your registry URL

**Troubleshooting Build:**
```bash
# If build fails with "exec format error", ensure podman machine supports emulation
podman machine ssh
# Inside VM:
cat /proc/sys/fs/binfmt_misc/qemu-aarch64  # Should show enabled

# If QEMU not installed in podman VM:
podman machine stop
podman machine rm
podman machine init --now --rootful
```

### Step 3: Push Operator Image

```bash
# Push to registry
make docker-push IMG=${IMG}

# This runs:
# podman push ${IMG}
```

**Verify Push:**
```bash
# Check image exists in registry
podman search ${IMG}

# For Quay.io, visit: https://quay.io/repository/<org>/aiobs-operator
# Make repository PUBLIC if deploying to cluster without pull secrets
```

### Step 4: Deploy to OpenShift

#### Option A: Direct Deployment (Recommended for Testing)

```bash
# 1. Install CRDs
make install

# Verify CRDs installed
oc get crd aiobservabilitysummarizers.obs.redhat.com

# 2. Deploy operator
make deploy IMG=${IMG}

# This creates:
# - Namespace: aiobs-operator-system
# - ServiceAccount, Role, RoleBinding, ClusterRole, ClusterRoleBinding
# - Deployment: aiobs-operator-controller-manager
# - Service, Metrics service
```

**Verify Deployment:**
```bash
# Check namespace
oc get namespace aiobs-operator-system

# Check deployment
oc get deployment -n aiobs-operator-system

# Check pods
oc get pods -n aiobs-operator-system

# Expected output:
# NAME                                                    READY   STATUS    RESTARTS   AGE
# aiobs-operator-controller-manager-xxxxxxxxxx-xxxxx     2/2     Running   0          30s

# Check operator logs
oc logs -n aiobs-operator-system deployment/aiobs-operator-controller-manager -c manager -f

# Should see:
# "Helm manager initialized" chartsPath="../deploy/helm"
# "Starting manager"
```

#### Option B: OLM-Based Deployment (Production)

```bash
# 1. Build and push operator image (done above)

# 2. Generate OLM bundle
make bundle IMG=${IMG}

# 3. Build and push bundle image
export BUNDLE_IMG=${REGISTRY}/${ORG}/aiobs-operator-bundle:v${VERSION}
make bundle-build BUNDLE_IMG=${BUNDLE_IMG}
podman push ${BUNDLE_IMG}

# 4. Build and push catalog image
export CATALOG_IMG=${REGISTRY}/${ORG}/aiobs-operator-catalog:v${VERSION}
make catalog-build CATALOG_IMG=${CATALOG_IMG}
make catalog-push CATALOG_IMG=${CATALOG_IMG}

# 5. Create CatalogSource
cat <<EOF | oc apply -f -
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: aiobs-operator-catalog
  namespace: openshift-marketplace
spec:
  sourceType: grpc
  image: ${CATALOG_IMG}
  displayName: AI Observability Operator
  publisher: Red Hat
  updateStrategy:
    registryPoll:
      interval: 10m
EOF

# 6. Wait for catalog to be ready
oc get catalogsource -n openshift-marketplace
oc get packagemanifest | grep aiobs

# 7. Install via OperatorHub UI or create Subscription
cat <<EOF | oc apply -f -
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: aiobs-operator
  namespace: openshift-operators
spec:
  channel: alpha
  name: aiobs-operator
  source: aiobs-operator-catalog
  sourceNamespace: openshift-marketplace
EOF
```

### Step 5: Deploy Sample CR

```bash
# Create test namespace
oc create namespace aiobs-test

# Apply sample Custom Resource
cat <<EOF | oc apply -f -
apiVersion: obs.redhat.com/v1alpha1
kind: AIObservabilitySummarizer
metadata:
  name: aiobs-sample
  namespace: default
spec:
  namespace: aiobs-test
  imageRegistry: quay.io/ecosystem-appeng
  imageVersion: "1.0.7"

  rag:
    enabled: true
    model: llama-3-1-8b-instruct
    postgresql:
      storageSize: 10Gi
    minio:
      storageSize: 20Gi

  mcpServer:
    enabled: true
    replicas: 1

  consolePlugin:
    enabled: true
    autoEnable: false

  observabilityStack:
    enabled: true
    minio:
      storageSize: 100Gi
    tempo:
      retention: "7d"
    loki:
      retention: "30d"

  selfHealing: true
  enableTracing: true
  enableLogging: true
EOF
```

**Watch Reconciliation:**
```bash
# Watch operator logs
oc logs -n aiobs-operator-system deployment/aiobs-operator-controller-manager -c manager -f

# Watch CR status
watch oc get aiobs -o wide

# Describe CR for detailed status
oc describe aiobs aiobs-sample

# Check conditions
oc get aiobs aiobs-sample -o jsonpath='{.status.conditions}' | jq .
```

## Verification Steps

### 1. Operator Health

```bash
# Check operator deployment
oc get deployment -n aiobs-operator-system
oc get pods -n aiobs-operator-system

# Check operator metrics endpoint
oc get svc -n aiobs-operator-system
oc port-forward -n aiobs-operator-system svc/aiobs-operator-controller-manager-metrics-service 8443:8443
```

### 2. CRD Validation

```bash
# List installed CRDs
oc get crd | grep obs.redhat.com

# Describe CRD
oc describe crd aiobservabilitysummarizers.obs.redhat.com

# Check API versions
oc api-resources | grep aiobs
```

### 3. RBAC Permissions

```bash
# Check operator ServiceAccount
oc get sa -n aiobs-operator-system

# Check ClusterRoles
oc get clusterrole | grep aiobs-operator

# Check RoleBindings
oc get clusterrolebinding | grep aiobs-operator
```

### 4. Component Deployment

```bash
# Check if operator installed required OpenShift operators
oc get csv -n openshift-cluster-observability-operator
oc get csv -n openshift-opentelemetry-operator
oc get csv -n openshift-tempo-operator
oc get csv -n openshift-logging
oc get csv -n openshift-operators-redhat

# Check target namespace for components
oc get all -n aiobs-test
```

## Troubleshooting

### Issue: Image Pull Errors

```bash
# Problem: ImagePullBackOff on operator pod

# Solution 1: Make image public in Quay.io
# Go to: https://quay.io/repository/<org>/aiobs-operator
# Settings > Make Public

# Solution 2: Create pull secret
oc create secret docker-registry quay-pull-secret \
  --docker-server=quay.io \
  --docker-username=<username> \
  --docker-password=<password> \
  -n aiobs-operator-system

# Patch ServiceAccount
oc patch sa aiobs-operator-controller-manager \
  -n aiobs-operator-system \
  -p '{"imagePullSecrets":[{"name":"quay-pull-secret"}]}'
```

### Issue: Operator CrashLoopBackOff

```bash
# Check logs
oc logs -n aiobs-operator-system deployment/aiobs-operator-controller-manager -c manager --previous

# Common issues:
# 1. Charts path not found
# 2. Missing RBAC permissions
# 3. OLM API not available

# Fix charts path (for local testing):
# Update deployment to mount Helm charts as ConfigMap
```

### Issue: Permission Denied

```bash
# Check if you have cluster-admin
oc auth can-i '*' '*'

# If not, ensure your user has these permissions:
oc adm policy add-cluster-role-to-user cluster-admin <your-user>
```

### Issue: Build Fails on Mac

```bash
# If podman build fails with platform errors:

# 1. Check podman machine
podman machine ls
podman machine inspect

# 2. Restart podman machine
podman machine stop
podman machine start

# 3. Enable rootful mode (if needed)
podman machine set --rootful

# 4. Rebuild
make docker-build IMG=${IMG}
```

## Cleanup

### Remove CR

```bash
# Delete Custom Resource
oc delete aiobs aiobs-sample

# Delete test namespace
oc delete namespace aiobs-test
```

### Undeploy Operator

```bash
# Remove operator deployment
make undeploy

# Remove CRDs
make uninstall

# Clean up OLM (if used)
oc delete subscription aiobs-operator -n openshift-operators
oc delete catalogsource aiobs-operator-catalog -n openshift-marketplace
```

### Clean Local Images

```bash
# Remove local images
podman rmi ${IMG}
podman rmi ${BUNDLE_IMG}
podman rmi ${CATALOG_IMG}

# Clean up builder cache
podman system prune -a
```

## Quick Reference

### Environment Setup
```bash
export REGISTRY=quay.io
export ORG=<your-org>
export VERSION=0.0.1
export IMG=${REGISTRY}/${ORG}/aiobs-operator:v${VERSION}
```

### Complete Build & Deploy
```bash
# Build and push
make docker-build IMG=${IMG}
make docker-push IMG=${IMG}

# Deploy
make install
make deploy IMG=${IMG}

# Apply CR
oc apply -f config/samples/obs_v1alpha1_aiobservabilitysummarizer.yaml

# Watch
oc logs -n aiobs-operator-system deployment/aiobs-operator-controller-manager -c manager -f
```

### Check Status
```bash
# Operator
oc get pods -n aiobs-operator-system
oc get deployment -n aiobs-operator-system

# CRs
oc get aiobs -A
oc describe aiobs aiobs-sample

# Components
oc get all -n aiobs-test
```

## Next Steps

After successful deployment:
1. Monitor operator logs for reconciliation progress
2. Check Phase 2 implementation status (operator should install required operators)
3. Verify component deployment order follows dependency graph
4. Test health checks and self-healing features
5. Create additional test CRs with different configurations

## Platform-Specific Notes

### Mac Apple Silicon (M1/M2/M3)
- Default build uses `--platform=linux/amd64`
- QEMU emulation automatically used
- Build time may be slower than native architecture
- Test locally: `podman run --platform=linux/amd64 ${IMG} --version`

### Mac Intel
- Native AMD64 builds (faster)
- No emulation overhead

### Build Multi-Platform (Optional)
```bash
# Build for multiple architectures
make docker-buildx IMG=${IMG} PLATFORMS=linux/amd64,linux/arm64

# This creates manifest lists supporting multiple platforms
```

---

**Generated:** December 27, 2025
**Operator Version:** 0.0.1
**OpenShift Compatibility:** 4.14+
