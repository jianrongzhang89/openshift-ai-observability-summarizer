# OLM Deployment - Quick Reference Card

## New Makefile Targets

### `make install-ai-observability-operator`
Automated OLM-based operator installation

**Usage:**
```bash
make install-ai-observability-operator CATALOG_IMG=<catalog-image>
```

**What it does:**
1. Creates CatalogSource
2. Waits for catalog to be ready (5 min timeout)
3. Verifies PackageManifest
4. Creates Subscription
5. Waits for CSV to succeed (5 min timeout)
6. Shows installation summary

### `make uninstall-ai-observability-operator`
Clean removal of OLM-deployed operator

**Usage:**
```bash
make uninstall-ai-observability-operator
```

**What it removes:**
- ClusterServiceVersion (operator deployment)
- Subscription
- CatalogSource

---

## Complete Workflow

### 1. Build Images
```bash
export REGISTRY=quay.io
export ORG=<your-org>
export VERSION=0.0.1
export IMG=${REGISTRY}/${ORG}/aiobs-operator:v${VERSION}
export BUNDLE_IMG=${REGISTRY}/${ORG}/aiobs-operator-bundle:v${VERSION}
export CATALOG_IMG=${REGISTRY}/${ORG}/aiobs-operator-catalog:v${VERSION}

# Build operator
make docker-build IMG=${IMG}
make docker-push IMG=${IMG}

# Build bundle
make bundle IMG=${IMG}
make bundle-build BUNDLE_IMG=${BUNDLE_IMG}
podman push ${BUNDLE_IMG}

# Build catalog
make catalog-build CATALOG_IMG=${CATALOG_IMG}
make catalog-push CATALOG_IMG=${CATALOG_IMG}
```

### 2. Install Operator
```bash
make install-ai-observability-operator CATALOG_IMG=${CATALOG_IMG}
```

### 3. Verify Installation
```bash
# Check operator
oc get csv -n openshift-operators
oc get pods -n openshift-operators | grep aiobs

# Check operator logs
oc logs -n openshift-operators deployment/aiobs-operator-controller-manager -c manager -f
```

### 4. Create Custom Resource
```bash
oc apply -f config/samples/obs_v1alpha1_aiobservabilitysummarizer.yaml
oc get aiobs -A --watch
```

### 5. Uninstall (when done)
```bash
# Delete CRs first
oc delete aiobs --all -A

# Uninstall operator
make uninstall-ai-observability-operator
```

---

## Configuration Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `CATALOG_IMG` | *required* | Catalog image (e.g., quay.io/org/catalog:v0.0.1) |
| `CATALOG_NAMESPACE` | `openshift-marketplace` | Namespace for CatalogSource |
| `OPERATOR_NAMESPACE` | `openshift-operators` | Namespace for operator |
| `SUBSCRIPTION_CHANNEL` | `alpha` | Subscription channel |

**Override example:**
```bash
make install-ai-observability-operator \
  CATALOG_IMG=${CATALOG_IMG} \
  OPERATOR_NAMESPACE=custom-operators \
  SUBSCRIPTION_CHANNEL=stable
```

---

## Verification Commands

```bash
# CatalogSource
oc get catalogsource -n openshift-marketplace
oc describe catalogsource aiobs-operator-catalog -n openshift-marketplace

# PackageManifest
oc get packagemanifest | grep aiobs

# Subscription
oc get subscription -n openshift-operators
oc describe subscription aiobs-operator -n openshift-operators

# CSV (ClusterServiceVersion)
oc get csv -n openshift-operators
oc describe csv <csv-name> -n openshift-operators

# Operator Pods
oc get pods -n openshift-operators -l control-plane=controller-manager

# Operator Logs
oc logs -n openshift-operators deployment/aiobs-operator-controller-manager -c manager -f
```

---

## Troubleshooting

### CatalogSource Not Ready
```bash
# Check pod
oc get pods -n openshift-marketplace | grep aiobs-operator-catalog

# Check logs
oc logs -n openshift-marketplace \
  $(oc get pods -n openshift-marketplace -l olm.catalogSource=aiobs-operator-catalog -o name)

# Solution: Make catalog image public or add pull secret
```

### Subscription Not Creating InstallPlan
```bash
# Check subscription status
oc get subscription aiobs-operator -n openshift-operators -o yaml

# Wait for PackageManifest
oc get packagemanifest | grep aiobs

# Delete and retry
oc delete subscription aiobs-operator -n openshift-operators
make install-ai-observability-operator CATALOG_IMG=${CATALOG_IMG}
```

### CSV Installation Failed
```bash
# Check CSV status
csv=$(oc get subscription aiobs-operator -n openshift-operators -o jsonpath='{.status.installedCSV}')
oc get csv $csv -n openshift-operators -o yaml

# Check pod issues
oc get pods -n openshift-operators -l control-plane=controller-manager
oc describe pod -n openshift-operators -l control-plane=controller-manager

# Common: Image pull error - make operator image public
```

---

## One-Liner Commands

```bash
# Full install (after images are pushed)
make install-ai-observability-operator CATALOG_IMG=quay.io/<org>/aiobs-operator-catalog:v0.0.1

# Check if operator is running
oc get csv -n openshift-operators | grep aiobs-operator

# Watch operator logs
oc logs -n openshift-operators -l control-plane=controller-manager -f --tail=50

# Apply sample CR and watch
oc apply -f config/samples/obs_v1alpha1_aiobservabilitysummarizer.yaml && oc get aiobs -A -w

# Full uninstall
oc delete aiobs --all -A && make uninstall-ai-observability-operator
```

---

## Comparison: OLM vs Direct

| Method | Command | Use Case |
|--------|---------|----------|
| **OLM** | `make install-ai-observability-operator` | Production, managed clusters |
| **Direct** | `make deploy` | Development, testing |

### OLM Advantages
- ✅ Production-ready
- ✅ Automatic updates
- ✅ Lifecycle management
- ✅ Multi-tenant support
- ✅ OperatorHub integration

### Direct Deployment Advantages
- ✅ Faster iteration
- ✅ No OLM required
- ✅ Simpler setup
- ✅ Development-friendly

---

## Tips

1. **Make images public** for easier testing (or configure pull secrets)
2. **Wait for CatalogSource** to be fully ready before troubleshooting
3. **Check OLM pods** if installations consistently fail
   ```bash
   oc get pods -n openshift-operator-lifecycle-manager
   ```
4. **Use events** to debug issues
   ```bash
   oc get events -n openshift-marketplace --sort-by='.lastTimestamp'
   oc get events -n openshift-operators --sort-by='.lastTimestamp'
   ```
5. **Check OLM documentation** for advanced scenarios:
   https://olm.operatorframework.io/

---

**Quick Start:**
```bash
# After building images:
make install-ai-observability-operator CATALOG_IMG=${CATALOG_IMG}
oc apply -f config/samples/obs_v1alpha1_aiobservabilitysummarizer.yaml
oc get aiobs -A -w
```

**Quick Cleanup:**
```bash
oc delete aiobs --all -A
make uninstall-ai-observability-operator
```

---

**See:** `OLM_DEPLOYMENT_GUIDE.md` for detailed documentation
