# OLM-Based Operator Deployment Guide

This guide covers deploying the AI Observability Summarizer Operator via **Operator Lifecycle Manager (OLM)** using the automated `install-ai-observability-operator` Makefile target.

## Overview

The new Makefile target automates the complete OLM deployment workflow:
1. ✅ Creates CatalogSource with your catalog image
2. ✅ Waits for CatalogSource to be ready (up to 5 minutes)
3. ✅ Verifies PackageManifest is available
4. ✅ Creates Subscription to install the operator
5. ✅ Waits for ClusterServiceVersion to succeed (up to 5 minutes)
6. ✅ Shows installation summary and next steps

## Prerequisites

### 1. Build and Push Images

You must build and push all three images before using OLM deployment:

```bash
# Set your registry and version
export REGISTRY=quay.io
export ORG=<your-org>
export VERSION=0.0.1

# Set image names
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
```

### 2. OpenShift Cluster Access

```bash
# Login to OpenShift
oc login --server=https://api.your-cluster.com:6443 --token=<your-token>

# Verify access
oc whoami
oc get nodes

# Ensure you have permissions to create operators
oc auth can-i create catalogsources -n openshift-marketplace
oc auth can-i create subscriptions -n openshift-operators
```

### 3. Make Images Public (or Configure Pull Secrets)

**Option A: Make images public** (easier for testing)
```bash
# In Quay.io web interface:
# 1. Go to https://quay.io/repository/<org>/aiobs-operator
# 2. Settings > Make Public
# 3. Repeat for bundle and catalog images
```

**Option B: Create pull secret** (for private images)
```bash
# Create pull secret in openshift-marketplace
oc create secret docker-registry quay-pull-secret \
  --docker-server=quay.io \
  --docker-username=<username> \
  --docker-password=<password> \
  -n openshift-marketplace

# Create pull secret in openshift-operators
oc create secret docker-registry quay-pull-secret \
  --docker-server=quay.io \
  --docker-username=<username> \
  --docker-password=<password> \
  -n openshift-operators
```

## Quick Start

### Install Operator via OLM

```bash
# One command to install everything!
make install-ai-observability-operator CATALOG_IMG=${CATALOG_IMG}
```

**What it does:**
1. Creates CatalogSource in `openshift-marketplace`
2. Waits for catalog to be ready
3. Creates Subscription in `openshift-operators`
4. Waits for operator to be installed
5. Shows installation summary

**Expected Output:**
```
==================================================
Installing AI Observability Operator via OLM
==================================================
Catalog Image: quay.io/ecosystem-appeng/aiobs-operator-catalog:v0.0.1
Operator Namespace: openshift-operators
Channel: alpha

==> Creating CatalogSource...
✓ CatalogSource created

==> Waiting for CatalogSource to be ready...
  Waiting for CatalogSource (0/300 seconds)...
  Waiting for CatalogSource (5/300 seconds)...
✓ CatalogSource is READY

==> Verifying PackageManifest...
✓ PackageManifest found
NAME             CATALOG                    AGE
aiobs-operator   AI Observability Operator  10s

==> Creating Subscription...
✓ Subscription created

==> Waiting for InstallPlan...
✓ InstallPlan created: install-xxxxx

==> Waiting for ClusterServiceVersion...
  CSV: aiobs-operator.v0.0.1, Phase: Installing
  CSV: aiobs-operator.v0.0.1, Phase: Succeeded
✓ Operator installed successfully!

==> Operator Installation Summary
---
NAME                      DISPLAY                     TYPE   PUBLISHER   AGE
aiobs-operator-catalog    AI Observability Operator   grpc   Red Hat     30s
---
NAME             PACKAGE          SOURCE                   CHANNEL
aiobs-operator   aiobs-operator   aiobs-operator-catalog   alpha
---
NAME                     DISPLAY                            VERSION   REPLACES   PHASE
aiobs-operator.v0.0.1    AI Observability Summarizer Op...  0.0.1                Succeeded

==================================================
✓ AI Observability Operator installed via OLM!
==================================================

Next steps:
  1. Check operator deployment:
     kubectl get csv -n openshift-operators
     kubectl get pods -n openshift-operators -l control-plane=controller-manager

  2. Create AIObservabilitySummarizer CR:
     kubectl apply -f config/samples/obs_v1alpha1_aiobservabilitysummarizer.yaml
```

### Verify Installation

```bash
# Check CatalogSource
oc get catalogsource -n openshift-marketplace

# Check Subscription
oc get subscription -n openshift-operators

# Check CSV (ClusterServiceVersion)
oc get csv -n openshift-operators

# Check operator pods
oc get pods -n openshift-operators | grep aiobs-operator

# Check operator logs
oc logs -n openshift-operators deployment/aiobs-operator-controller-manager -c manager -f
```

### Create Custom Resource

```bash
# Apply sample CR
oc apply -f config/samples/obs_v1alpha1_aiobservabilitysummarizer.yaml

# Watch CR status
oc get aiobs -A --watch

# Check CR details
oc describe aiobs aiobservabilitysummarizer-sample
```

## Configuration Options

### Customize Installation

```bash
# Install to different namespace
make install-ai-observability-operator \
  CATALOG_IMG=${CATALOG_IMG} \
  OPERATOR_NAMESPACE=my-operators

# Use different catalog namespace
make install-ai-observability-operator \
  CATALOG_IMG=${CATALOG_IMG} \
  CATALOG_NAMESPACE=custom-marketplace

# Use different channel
make install-ai-observability-operator \
  CATALOG_IMG=${CATALOG_IMG} \
  SUBSCRIPTION_CHANNEL=stable
```

### Available Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `CATALOG_IMG` | (required) | Catalog image with tag |
| `CATALOG_NAMESPACE` | `openshift-marketplace` | Where to create CatalogSource |
| `OPERATOR_NAMESPACE` | `openshift-operators` | Where to install operator |
| `SUBSCRIPTION_CHANNEL` | `alpha` | Subscription channel |
| `KUBECTL` | `kubectl` | kubectl/oc binary to use |

## Uninstall Operator

```bash
# Uninstall operator via OLM
make uninstall-ai-observability-operator
```

**What it does:**
1. Deletes ClusterServiceVersion
2. Deletes Subscription
3. Deletes CatalogSource

**Expected Output:**
```
Uninstalling AI Observability Operator via OLM...
Deleting CSV: aiobs-operator.v0.0.1
Deleting Subscription...
Deleting CatalogSource...
✓ Operator uninstalled
```

**Note:** This does NOT delete CRDs or existing Custom Resources. To fully clean up:

```bash
# Delete all AIObservabilitySummarizer CRs
oc delete aiobs --all -A

# Delete CRDs (optional - will fail if CRs exist)
make uninstall
```

## Complete Workflow Example

### End-to-End Deployment

```bash
#!/bin/bash
set -e

# Configuration
export REGISTRY=quay.io
export ORG=ecosystem-appeng
export VERSION=0.0.1
export IMG=${REGISTRY}/${ORG}/aiobs-operator:v${VERSION}
export BUNDLE_IMG=${REGISTRY}/${ORG}/aiobs-operator-bundle:v${VERSION}
export CATALOG_IMG=${REGISTRY}/${ORG}/aiobs-operator-catalog:v${VERSION}

echo "==> Step 1: Login to registry"
podman login ${REGISTRY}

echo "==> Step 2: Build and push operator image"
make docker-build IMG=${IMG}
make docker-push IMG=${IMG}

echo "==> Step 3: Generate bundle"
make bundle IMG=${IMG}

echo "==> Step 4: Build and push bundle image"
make bundle-build BUNDLE_IMG=${BUNDLE_IMG}
podman push ${BUNDLE_IMG}

echo "==> Step 5: Build and push catalog image"
make catalog-build CATALOG_IMG=${CATALOG_IMG}
make catalog-push CATALOG_IMG=${CATALOG_IMG}

echo "==> Step 6: Login to OpenShift"
oc login --server=https://api.your-cluster.com:6443

echo "==> Step 7: Install operator via OLM"
make install-ai-observability-operator CATALOG_IMG=${CATALOG_IMG}

echo "==> Step 8: Wait for operator to be ready"
sleep 30

echo "==> Step 9: Apply sample CR"
oc apply -f config/samples/obs_v1alpha1_aiobservabilitysummarizer.yaml

echo "==> Step 10: Watch reconciliation"
oc get aiobs -A --watch
```

## Troubleshooting

### CatalogSource Not Ready

**Problem:** CatalogSource stuck in non-READY state

**Check:**
```bash
# Check catalogsource status
oc get catalogsource aiobs-operator-catalog -n openshift-marketplace -o yaml

# Check catalogsource pod
oc get pods -n openshift-marketplace | grep aiobs-operator-catalog

# Check pod logs
oc logs -n openshift-marketplace $(oc get pods -n openshift-marketplace -l olm.catalogSource=aiobs-operator-catalog -o name)
```

**Common Causes:**
1. **Image pull error** - Make image public or add pull secret
2. **Invalid catalog image** - Verify catalog was built correctly
3. **Registry unavailable** - Check network/firewall

**Solution:**
```bash
# Make catalog image public in Quay.io
# OR create pull secret in openshift-marketplace

# Delete and recreate catalogsource
oc delete catalogsource aiobs-operator-catalog -n openshift-marketplace
make install-ai-observability-operator CATALOG_IMG=${CATALOG_IMG}
```

### Subscription Not Installing

**Problem:** Subscription created but no InstallPlan

**Check:**
```bash
# Check subscription status
oc get subscription aiobs-operator -n openshift-operators -o yaml

# Look for status.conditions
oc get subscription aiobs-operator -n openshift-operators -o jsonpath='{.status.conditions}' | jq .
```

**Common Causes:**
1. **PackageManifest not found** - CatalogSource not ready yet
2. **Invalid channel** - Channel name doesn't match bundle
3. **OLM not working** - Check OLM pods

**Solution:**
```bash
# Check OLM is running
oc get pods -n openshift-operator-lifecycle-manager

# Wait longer for PackageManifest
oc get packagemanifest | grep aiobs

# Delete and recreate subscription
oc delete subscription aiobs-operator -n openshift-operators
make install-ai-observability-operator CATALOG_IMG=${CATALOG_IMG}
```

### CSV Installation Failed

**Problem:** CSV stuck in non-Succeeded phase

**Check:**
```bash
# Get CSV details
csv=$(oc get subscription aiobs-operator -n openshift-operators -o jsonpath='{.status.installedCSV}')
oc get csv $csv -n openshift-operators -o yaml

# Check deployment
oc get deployment -n openshift-operators | grep aiobs

# Check pod status
oc get pods -n openshift-operators -l control-plane=controller-manager

# Check pod logs
oc logs -n openshift-operators deployment/aiobs-operator-controller-manager -c manager
```

**Common Causes:**
1. **Image pull error** - Operator image not accessible
2. **RBAC issues** - Insufficient permissions
3. **Resource constraints** - Insufficient CPU/memory

**Solution:**
```bash
# Check events
oc get events -n openshift-operators --sort-by='.lastTimestamp'

# Describe pod for errors
oc describe pod -n openshift-operators -l control-plane=controller-manager

# If image pull error, make operator image public or add pull secret
```

### Timeout Errors

**Problem:** Make target times out

**Solutions:**
```bash
# CatalogSource timeout (default: 5 minutes)
# Edit Makefile line 248: timeout=300 → timeout=600

# CSV timeout (default: 5 minutes)
# Edit Makefile line 311: timeout=300 → timeout=600

# Or wait manually and check:
oc get catalogsource aiobs-operator-catalog -n openshift-marketplace -w
oc get subscription aiobs-operator -n openshift-operators -w
```

## Advanced Usage

### Manual OLM Resources

If you prefer manual control:

```bash
# 1. Create CatalogSource
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

# 2. Wait for ready
oc wait --for=jsonpath='{.status.connectionState.lastObservedState}'=READY \
  catalogsource/aiobs-operator-catalog -n openshift-marketplace --timeout=5m

# 3. Create Subscription
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
  installPlanApproval: Automatic
EOF

# 4. Wait for CSV
oc wait --for=jsonpath='{.status.phase}'=Succeeded \
  csv -l operators.coreos.com/aiobs-operator.openshift-operators \
  -n openshift-operators --timeout=5m
```

### Install to Specific Namespace

```bash
# Create namespace
oc create namespace my-aiobs-operator

# Create OperatorGroup
cat <<EOF | oc apply -f -
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: aiobs-operator-group
  namespace: my-aiobs-operator
spec:
  targetNamespaces:
  - my-aiobs-operator
EOF

# Install operator
make install-ai-observability-operator \
  CATALOG_IMG=${CATALOG_IMG} \
  OPERATOR_NAMESPACE=my-aiobs-operator
```

### Update Operator Version

```bash
# Build new version
export VERSION=0.0.2
export CATALOG_IMG=${REGISTRY}/${ORG}/aiobs-operator-catalog:v${VERSION}

# Build and push new images
# ... (same build steps)

# Update catalog image in CatalogSource
oc patch catalogsource aiobs-operator-catalog \
  -n openshift-marketplace \
  --type=merge \
  -p "{\"spec\":{\"image\":\"${CATALOG_IMG}\"}}"

# OLM will automatically upgrade operator
```

## Comparison: OLM vs Direct Deployment

| Aspect | OLM (`install-ai-observability-operator`) | Direct (`make deploy`) |
|--------|-------------------------------------------|------------------------|
| **Method** | OLM Subscription | Kustomize manifests |
| **Upgrade** | Automatic via OLM | Manual redeployment |
| **Namespace** | openshift-operators | aiobs-operator-system |
| **RBAC** | Managed by OLM | Manually applied |
| **Discovery** | OperatorHub | Not discoverable |
| **Production** | ✅ Recommended | For development only |
| **Uninstall** | OLM-managed | Manual cleanup |

## Summary

The `install-ai-observability-operator` target provides:
- ✅ Automated OLM deployment workflow
- ✅ Built-in waiting and verification
- ✅ Production-ready installation method
- ✅ Automatic updates and lifecycle management
- ✅ Clean uninstallation

**Recommended for:** Production deployments, multi-tenant clusters, managed environments

**Use direct deployment for:** Development, testing, quick iterations

---

**Generated:** December 27, 2025
**Makefile Target:** `install-ai-observability-operator`
**OpenShift Version:** 4.14+
