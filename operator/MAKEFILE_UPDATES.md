# Makefile Updates for Podman & AMD64 Builds on Mac

**Date:** December 27, 2025
**Purpose:** Configure operator build system for OpenShift deployment using podman on Mac

## Changes Summary

### 1. Default Container Tool Changed to Podman ✅

**Before:**
```makefile
CONTAINER_TOOL ?= docker
```

**After:**
```makefile
CONTAINER_TOOL ?= podman
```

**Impact:**
- All container operations now use `podman` by default
- Can override with: `make docker-build CONTAINER_TOOL=docker`
- Compatible with Mac, Linux, and containers-as-a-service

### 2. Platform Targeting for AMD64 ✅

**Added:**
```makefile
# PLATFORM defines the target platform for image builds.
# Default is linux/amd64 for OpenShift compatibility.
# Override with: make docker-build PLATFORM=linux/arm64
PLATFORM ?= linux/amd64
```

**Impact:**
- Builds AMD64 images by default (OpenShift standard)
- Works on Mac ARM (M1/M2/M3) via QEMU emulation
- Can override for other platforms

### 3. Updated docker-build Target ✅

**Before:**
```makefile
docker-build:
	$(CONTAINER_TOOL) build -t ${IMG} .
```

**After:**
```makefile
docker-build: ## Build container image with the manager.
	$(CONTAINER_TOOL) build --platform=$(PLATFORM) -t ${IMG} .
```

**Impact:**
- Explicit platform specification
- Builds AMD64 images on any host architecture
- Mac ARM users get cross-platform builds automatically

### 4. Enhanced docker-buildx for Multi-Platform ✅

**Changes:**
- Added conditional logic for podman vs docker
- Podman uses manifest-based multi-platform builds
- Docker uses buildx (existing logic)

**Podman Multi-Platform Logic:**
```makefile
ifeq ($(CONTAINER_TOOL),podman)
	@for platform in $(PLATFORMS); do \
		$(CONTAINER_TOOL) build --platform=$$platform -t ${IMG}-$$platform .; \
		$(CONTAINER_TOOL) push ${IMG}-$$platform; \
	done
	$(CONTAINER_TOOL) manifest create ${IMG} ...
	$(CONTAINER_TOOL) manifest push ${IMG}
endif
```

**Impact:**
- Works with podman on Mac without requiring buildx
- Supports multiple architectures: AMD64, ARM64, PPC64LE, S390X
- Creates proper manifest lists

### 5. Updated bundle-build Target ✅

**Before:**
```makefile
bundle-build:
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .
```

**After:**
```makefile
bundle-build:
	$(CONTAINER_TOOL) build --platform=$(PLATFORM) -f bundle.Dockerfile -t $(BUNDLE_IMG) .
```

**Impact:**
- Bundle images also target AMD64 platform
- Uses CONTAINER_TOOL variable (podman/docker)

### 6. Updated catalog-build Target ✅

**Before:**
```makefile
catalog-build:
	$(OPM) index add --container-tool docker ...
```

**After:**
```makefile
catalog-build:
	$(OPM) index add --container-tool $(CONTAINER_TOOL) ...
```

**Impact:**
- OLM catalog builds use podman
- Consistent with other build targets

## Testing on Mac

### Prerequisites

```bash
# Install podman
brew install podman

# Initialize podman machine (creates VM on Mac)
podman machine init
podman machine start

# Verify
podman info
```

### Test Build (AMD64 on Mac ARM)

```bash
# Set test image
export IMG=test-operator:v0.0.1

# Build AMD64 image on Mac (any architecture)
make docker-build IMG=${IMG}

# Should output:
# podman build --platform=linux/amd64 -t test-operator:v0.0.1 .
# [+] Building with QEMU emulation...

# Verify image architecture
podman inspect ${IMG} | grep Architecture
# Output: "Architecture": "amd64"
```

### Test with Docker (Override)

```bash
# Use docker instead of podman
make docker-build IMG=${IMG} CONTAINER_TOOL=docker

# Or set environment variable
export CONTAINER_TOOL=docker
make docker-build IMG=${IMG}
```

## Build Examples

### Basic Operator Build

```bash
# Default: podman, AMD64
make docker-build IMG=quay.io/myorg/operator:v0.0.1

# Equivalent to:
podman build --platform=linux/amd64 -t quay.io/myorg/operator:v0.0.1 .
```

### Multi-Platform Build

```bash
# Build for multiple architectures
make docker-buildx IMG=quay.io/myorg/operator:v0.0.1

# Builds for: linux/amd64, linux/arm64, linux/ppc64le, linux/s390x
# Uses podman manifest on Mac
```

### Custom Platform

```bash
# Build ARM64 image
make docker-build IMG=quay.io/myorg/operator:v0.0.1 PLATFORM=linux/arm64
```

### Bundle & Catalog

```bash
# Build bundle (AMD64)
make bundle-build BUNDLE_IMG=quay.io/myorg/operator-bundle:v0.0.1

# Build catalog (uses podman)
make catalog-build CATALOG_IMG=quay.io/myorg/operator-catalog:v0.0.1
```

## Mac-Specific Considerations

### QEMU Emulation
- Mac ARM uses QEMU to build AMD64 images
- Automatic with podman machine
- Slower than native builds (~2-3x)
- Fully compatible with OpenShift AMD64 nodes

### Podman Machine
- Required on Mac (no native container runtime)
- Similar to Docker Desktop
- Resource limits can be configured:
  ```bash
  podman machine set --cpus 4 --memory 8192
  ```

### Rootful vs Rootless
- Default: rootless (recommended)
- For some operations, may need rootful:
  ```bash
  podman machine set --rootful
  ```

## Verification Checklist

- [x] ✅ Makefile uses `CONTAINER_TOOL ?= podman`
- [x] ✅ Default `PLATFORM ?= linux/amd64`
- [x] ✅ `docker-build` includes `--platform` flag
- [x] ✅ `docker-buildx` supports podman manifests
- [x] ✅ `bundle-build` uses `CONTAINER_TOOL` and `PLATFORM`
- [x] ✅ `catalog-build` uses `CONTAINER_TOOL`
- [x] ✅ Makefile tested with `make help`
- [x] ✅ Podman installed and verified (v5.7.0)

## Complete Build Pipeline Example

```bash
# Setup
export REGISTRY=quay.io
export ORG=ecosystem-appeng
export VERSION=0.0.1
export IMG=${REGISTRY}/${ORG}/aiobs-operator:v${VERSION}
export BUNDLE_IMG=${REGISTRY}/${ORG}/aiobs-operator-bundle:v${VERSION}
export CATALOG_IMG=${REGISTRY}/${ORG}/aiobs-operator-catalog:v${VERSION}

# Login to registry
podman login ${REGISTRY}

# Build operator (AMD64 on Mac)
make docker-build IMG=${IMG}

# Push operator
make docker-push IMG=${IMG}

# Generate bundle
make bundle IMG=${IMG}

# Build bundle
make bundle-build BUNDLE_IMG=${BUNDLE_IMG}

# Push bundle
podman push ${BUNDLE_IMG}

# Build catalog
make catalog-build CATALOG_IMG=${CATALOG_IMG}

# Push catalog
make catalog-push CATALOG_IMG=${CATALOG_IMG}

# Deploy to OpenShift
make deploy IMG=${IMG}
```

## Architecture Matrix

| Build Host | Target Platform | Method | Performance |
|------------|-----------------|--------|-------------|
| Mac ARM (M1/M2/M3) | linux/amd64 | QEMU emulation | Slower (~2-3x) |
| Mac ARM | linux/arm64 | Native | Fast |
| Mac Intel | linux/amd64 | Native | Fast |
| Linux AMD64 | linux/amd64 | Native | Fast |
| Linux ARM64 | linux/amd64 | QEMU emulation | Slower |

## Benefits

1. **Cross-Platform Compatibility**
   - Build AMD64 images on any Mac (ARM or Intel)
   - Deploy to standard OpenShift AMD64 clusters

2. **Podman First**
   - Open source alternative to Docker
   - Better suited for OpenShift workflows
   - No Docker Desktop license required

3. **Flexibility**
   - Easy to override CONTAINER_TOOL
   - Easy to override PLATFORM
   - Works with existing CI/CD

4. **Consistency**
   - All build targets use same variables
   - Platform specification is explicit
   - Less "works on my machine" issues

## Troubleshooting

### Build Fails with "exec format error"

**Problem:** Trying to run AMD64 binary on ARM system

**Solution:**
```bash
# Don't run the binary locally, push to registry and deploy to cluster
make docker-build IMG=${IMG}
make docker-push IMG=${IMG}
make deploy IMG=${IMG}
```

### "Cannot connect to Podman socket"

**Problem:** Podman machine not running

**Solution:**
```bash
podman machine start
```

### "platform not supported"

**Problem:** Old podman version

**Solution:**
```bash
brew upgrade podman
podman machine stop
podman machine rm
podman machine init --now
```

### Slow builds on Mac ARM

**Expected behavior:** Cross-compilation to AMD64 uses emulation

**Optimization:**
- Use build cache: `make docker-build` reuses layers
- Consider multi-stage builds: Already implemented
- Use smaller base images: Already using distroless

## References

- **Podman Documentation:** https://podman.io/docs
- **Podman Machine:** https://docs.podman.io/en/latest/markdown/podman-machine.1.html
- **Multi-Platform Builds:** https://docs.podman.io/en/latest/markdown/podman-manifest.1.html
- **OpenShift Operator SDK:** https://sdk.operatorframework.io/

---

**Updated:** December 27, 2025
**Tested on:** macOS 14.x (Sonoma), Podman 5.7.0
**OpenShift Target:** 4.14+
