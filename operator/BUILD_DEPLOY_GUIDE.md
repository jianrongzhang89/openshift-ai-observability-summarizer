# Operator Build & Deploy Guide

This guide covers building operator images, bundle images, catalog images, and deploying to OpenShift.

## Available Makefile Targets

The operator-sdk generated Makefile provides comprehensive build and deployment targets:

### 📦 Build Targets

```bash
# Build operator binary (local)
make build

# Build operator container image
make docker-build IMG=<registry>/<org>/aiobs-operator:v0.0.1

# Build multi-platform operator image
make docker-buildx IMG=<registry>/<org>/aiobs-operator:v0.0.1

# Build OLM bundle image
make bundle-build BUNDLE_IMG=<registry>/<org>/aiobs-operator-bundle:v0.0.1

# Build catalog image
make catalog-build CATALOG_IMG=<registry>/<org>/aiobs-operator-catalog:v0.0.1
```

### 🚀 Deployment Targets

```bash
# Install CRDs only
make install

# Deploy operator (requires operator image)
make deploy IMG=<registry>/<org>/aiobs-operator:v0.0.1

# Undeploy operator
make undeploy

# Uninstall CRDs
make uninstall
```

---

## Complete Build & Deploy Workflow

### Step 1: Configure Image Registry

Set your container registry details:

```bash
# Option 1: Export environment variables
export REGISTRY=quay.io
export ORG=ecosystem-appeng
export VERSION=0.0.1

# Option 2: Set in each command
# See examples below
```

### Step 2: Build Operator Image

```bash
# Build operator container image
make docker-build IMG=${REGISTRY}/${ORG}/aiobs-operator:v${VERSION}

# Or with direct values
make docker-build IMG=quay.io/ecosystem-appeng/aiobs-operator:v0.0.1
```

**What it does:**
- Compiles Go code
- Builds Docker/Podman image using `Dockerfile`
- Tags image locally

### Step 3: Push Operator Image

```bash
# Login to registry first
docker login quay.io
# or
podman login quay.io

# Push operator image
make docker-push IMG=${REGISTRY}/${ORG}/aiobs-operator:v${VERSION}
```

### Step 4: Deploy to OpenShift

**Option A: Direct Deployment (without OLM)**

```bash
# 1. Install CRDs
make install

# 2. Deploy operator
make deploy IMG=${REGISTRY}/${ORG}/aiobs-operator:v${VERSION}

# 3. Verify deployment
oc get deployment -n aiobs-operator-system
oc get pods -n aiobs-operator-system

# 4. Check operator logs
oc logs -n aiobs-operator-system deployment/aiobs-operator-controller-manager -f
```

**Option B: OLM-based Deployment (Recommended for Production)**

```bash
# 1. Generate OLM bundle
make bundle IMG=${REGISTRY}/${ORG}/aiobs-operator:v${VERSION}

# 2. Build and push bundle image
make bundle-build BUNDLE_IMG=${REGISTRY}/${ORG}/aiobs-operator-bundle:v${VERSION}
docker push ${REGISTRY}/${ORG}/aiobs-operator-bundle:v${VERSION}

# 3. Build and push catalog image
make catalog-build CATALOG_IMG=${REGISTRY}/${ORG}/aiobs-operator-catalog:v${VERSION}
make catalog-push CATALOG_IMG=${REGISTRY}/${ORG}/aiobs-operator-catalog:v${VERSION}

# 4. Create CatalogSource
cat <<EOF | oc apply -f -
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: aiobs-operator-catalog
  namespace: openshift-marketplace
spec:
  sourceType: grpc
  image: ${REGISTRY}/${ORG}/aiobs-operator-catalog:v${VERSION}
  displayName: AI Observability Operator
  publisher: Red Hat
  updateStrategy:
    registryPoll:
      interval: 10m
EOF

# 5. Verify catalog is ready
oc get catalogsource -n openshift-marketplace
oc get packagemanifest | grep aiobs

# 6. Install operator via OperatorHub UI or create Subscription
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

---

## Quick Reference Commands

### Complete Build & Push Pipeline

```bash
#!/bin/bash
# complete-build.sh

export REGISTRY=quay.io
export ORG=ecosystem-appeng
export VERSION=0.0.1
export IMG=${REGISTRY}/${ORG}/aiobs-operator:v${VERSION}
export BUNDLE_IMG=${REGISTRY}/${ORG}/aiobs-operator-bundle:v${VERSION}
export CATALOG_IMG=${REGISTRY}/${ORG}/aiobs-operator-catalog:v${VERSION}

# Login to registry
podman login ${REGISTRY}

# Build and push operator image
make docker-build IMG=${IMG}
make docker-push IMG=${IMG}

# Generate and build bundle
make bundle IMG=${IMG}
make bundle-build BUNDLE_IMG=${BUNDLE_IMG}
podman push ${BUNDLE_IMG}

# Build and push catalog
make catalog-build CATALOG_IMG=${CATALOG_IMG}
make catalog-push CATALOG_IMG=${CATALOG_IMG}

echo "✅ All images built and pushed:"
echo "  Operator: ${IMG}"
echo "  Bundle:   ${BUNDLE_IMG}"
echo "  Catalog:  ${CATALOG_IMG}"
```

### Deploy to OpenShift

```bash
#!/bin/bash
# deploy-to-openshift.sh

export REGISTRY=quay.io
export ORG=ecosystem-appeng
export VERSION=0.0.1
export IMG=${REGISTRY}/${ORG}/aiobs-operator:v${VERSION}

# Ensure logged into OpenShift
oc whoami

# Deploy operator
make deploy IMG=${IMG}

# Wait for operator to be ready
oc wait --for=condition=available --timeout=300s \
  deployment/aiobs-operator-controller-manager \
  -n aiobs-operator-system

# Check operator status
oc get all -n aiobs-operator-system

echo "✅ Operator deployed successfully"
```

---

## Default Configuration

The Makefile uses these defaults (can be overridden):

```makefile
VERSION ?= 0.0.1
IMAGE_TAG_BASE ?= redhat.com/operator
IMG ?= controller:latest
BUNDLE_IMG ?= $(IMAGE_TAG_BASE)-bundle:v$(VERSION)
CATALOG_IMG ?= $(IMAGE_TAG_BASE)-catalog:v$(VERSION)
CONTAINER_TOOL ?= docker
```

**To override:**

```bash
# Set via environment
export VERSION=1.0.0
export IMAGE_TAG_BASE=quay.io/ecosystem-appeng/aiobs-operator
make bundle

# Or via command line
make bundle VERSION=1.0.0 IMAGE_TAG_BASE=quay.io/ecosystem-appeng/aiobs-operator
```

---

## Image Details

### 1. Operator Image

**Built from:** `Dockerfile`
**Purpose:** Contains the operator binary
**Size:** ~50-70MB (multi-stage build)

```dockerfile
# Key stages:
1. builder - Compiles Go code
2. runtime - Minimal distroless image with binary
```

### 2. Bundle Image

**Built from:** `bundle.Dockerfile` (generated by `make bundle`)
**Purpose:** OLM bundle with manifests, metadata, CRDs
**Contents:**
- ClusterServiceVersion (CSV)
- CRDs
- RBAC manifests
- Operator metadata

### 3. Catalog Image

**Purpose:** OLM catalog index
**Contains:** Bundle references for operator discovery

---

## Verification Steps

### Verify Operator Image

```bash
# Check image was built
docker images | grep aiobs-operator

# Inspect image
docker inspect ${IMG}

# Test run locally
docker run --rm ${IMG} --version
```

### Verify Bundle

```bash
# Validate bundle
operator-sdk bundle validate ./bundle

# Check bundle files
ls -la bundle/
cat bundle/manifests/aiobs-operator.clusterserviceversion.yaml
```

### Verify Deployment

```bash
# Check namespace
oc get namespace aiobs-operator-system

# Check deployment
oc get deployment -n aiobs-operator-system

# Check pods
oc get pods -n aiobs-operator-system

# Check operator logs
oc logs -n aiobs-operator-system -l control-plane=controller-manager -f

# Verify CRDs installed
oc get crd aiobservabilitysummarizers.obs.redhat.com
```

---

## Troubleshooting

### Build Issues

**Problem:** Docker build fails

```bash
# Solution: Use podman instead
make docker-build CONTAINER_TOOL=podman IMG=...
```

**Problem:** Multi-platform build fails

```bash
# Solution: Setup buildx
docker buildx create --use
docker buildx inspect --bootstrap
```

### Deployment Issues

**Problem:** ImagePullBackOff

```bash
# Check image exists and is public
oc describe pod -n aiobs-operator-system <pod-name>

# Make image public in registry (quay.io example)
# Go to: https://quay.io/repository/<org>/aiobs-operator
# Settings > Make Public
```

**Problem:** Operator not reconciling

```bash
# Check RBAC permissions
oc get clusterrole | grep aiobs-operator

# Check operator logs
oc logs -n aiobs-operator-system deployment/aiobs-operator-controller-manager
```

---

## Production Recommendations

### 1. Use Semantic Versioning

```bash
VERSION=1.0.0     # Major release
VERSION=1.0.1     # Patch release
VERSION=1.1.0     # Minor release
```

### 2. Tag Images Properly

```bash
# Build with version tag
make docker-build IMG=quay.io/org/aiobs-operator:v1.0.0

# Also tag as latest (after testing)
docker tag quay.io/org/aiobs-operator:v1.0.0 quay.io/org/aiobs-operator:latest
docker push quay.io/org/aiobs-operator:latest
```

### 3. Use Image Digests

```bash
# Enable digest pinning
make bundle USE_IMAGE_DIGESTS=true IMG=quay.io/org/aiobs-operator:v1.0.0
```

### 4. Multi-arch Builds

```bash
# Build for multiple platforms
make docker-buildx PLATFORMS=linux/amd64,linux/arm64 IMG=...
```

---

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Build and Push Operator

on:
  push:
    tags:
      - 'v*'

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v2

      - name: Login to Quay.io
        uses: docker/login-action@v2
        with:
          registry: quay.io
          username: ${{ secrets.QUAY_USERNAME }}
          password: ${{ secrets.QUAY_PASSWORD }}

      - name: Build and push operator image
        run: |
          cd operator
          make docker-buildx IMG=quay.io/ecosystem-appeng/aiobs-operator:${GITHUB_REF#refs/tags/}

      - name: Build and push bundle
        run: |
          cd operator
          make bundle bundle-build bundle-push \
            IMG=quay.io/ecosystem-appeng/aiobs-operator:${GITHUB_REF#refs/tags/} \
            BUNDLE_IMG=quay.io/ecosystem-appeng/aiobs-operator-bundle:${GITHUB_REF#refs/tags/}
```

---

## Summary

✅ **Yes, the Makefile fully supports:**
- Building operator images (`make docker-build`)
- Building bundle images (`make bundle-build`)
- Building catalog images (`make catalog-build`)
- Deploying to OpenShift (`make deploy`)
- OLM-based deployment workflow

All standard operator-sdk workflows are available out of the box!
