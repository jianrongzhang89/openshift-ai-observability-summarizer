# VERSION defines the project version for the bundle.
# Update this value when you upgrade the version of your project.
# To re-generate a bundle for another specific version without changing the standard setup, you can:
# - use the VERSION as arg of the bundle target (e.g make bundle VERSION=0.0.2)
# - use environment variables to overwrite this value (e.g export VERSION=0.0.2)
VERSION ?= 0.0.42

REGISTRY ?= quay.io
ORG ?= ecosystem-appeng
IMAGE_PREFIX ?= ai-observability-operator
# CHANNELS define the bundle channels used in the bundle.
# Add a new line here if you would like to change its default config. (E.g CHANNELS = "candidate,fast,stable")
# To re-generate a bundle for other specific channels without changing the standard setup, you can:
# - use the CHANNELS as arg of the bundle target (e.g make bundle CHANNELS=candidate,fast,stable)
# - use environment variables to overwrite this value (e.g export CHANNELS="candidate,fast,stable")
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif

# DEFAULT_CHANNEL defines the default channel used in the bundle.
# Add a new line here if you would like to change its default config. (E.g DEFAULT_CHANNEL = "stable")
# To re-generate a bundle for any other default channel without changing the default setup, you can:
# - use the DEFAULT_CHANNEL as arg of the bundle target (e.g make bundle DEFAULT_CHANNEL=stable)
# - use environment variables to overwrite this value (e.g export DEFAULT_CHANNEL="stable")
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# IMAGE_TAG_BASE defines the docker.io namespace and part of the image name for remote images.
# This variable is used to construct full image tags for bundle and catalog images.
#
# For example, running 'make bundle-build bundle-push catalog-build catalog-push' will build and push both
# redhat.com/operator-bundle:$VERSION and redhat.com/operator-catalog:$VERSION.
IMAGE_TAG_BASE ?= $(REGISTRY)/$(ORG)/$(IMAGE_PREFIX)

# BUNDLE_IMG defines the image:tag used for the bundle.
# You can use it as an arg. (E.g make bundle-build BUNDLE_IMG=<some-registry>/<project-name-bundle>:<tag>)
BUNDLE_IMG ?= $(IMAGE_TAG_BASE)-bundle:v$(VERSION)

# BUNDLE_GEN_FLAGS are the flags passed to the operator-sdk generate bundle command
BUNDLE_GEN_FLAGS ?= -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)

# The image tag given to the resulting catalog image (e.g. make catalog-build CATALOG_IMG=example.com/operator-catalog:v0.2.0).
CATALOG_IMG ?= $(IMAGE_TAG_BASE)-catalog:v$(VERSION)

# USE_IMAGE_DIGESTS defines if images are resolved via tags or digests
# You can enable this value if you would like to use SHA Based Digests
# To enable set flag to true
USE_IMAGE_DIGESTS ?= false
ifeq ($(USE_IMAGE_DIGESTS), true)
	BUNDLE_GEN_FLAGS += --use-image-digests
endif

# Set the Operator SDK version to use. By default, what is installed on the system is used.
# This is useful for CI or a project to utilize a specific version of the operator-sdk toolkit.
OPERATOR_SDK_VERSION ?= v1.39.1
# Image URL to use all building/pushing image targets
IMG ?= $(IMAGE_TAG_BASE):$(VERSION)
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.31.0

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# CONTAINER_TOOL defines the container tool to be used for building images.
# Default is podman. Set to docker if needed: make docker-build CONTAINER_TOOL=docker
CONTAINER_TOOL ?= podman

# PLATFORM defines the target platform for image builds.
# Default is linux/amd64 for OpenShift compatibility.
# Override with: make docker-build PLATFORM=linux/arm64
PLATFORM ?= linux/amd64

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out

# Utilize Kind or modify the e2e tests to load the image locally, enabling compatibility with other vendors.
.PHONY: test-e2e  # Run the e2e tests against a Kind k8s instance that is spun up.
test-e2e:
	go test ./test/e2e/ -v -ginkgo.v

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	$(GOLANGCI_LINT) run --fix

##@ Build

.PHONY: build
build: manifests generate fmt vet ## Build manager binary.
	go build -o bin/manager cmd/main.go

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./cmd/main.go

# Build container image with the manager.
# Default platform is linux/amd64 for OpenShift. Override with PLATFORM variable.
# Example: make docker-build IMG=quay.io/myorg/operator:v0.0.1 PLATFORM=linux/arm64
.PHONY: docker-build
docker-build: ## Build container image with the manager.
	$(CONTAINER_TOOL) build --platform=$(PLATFORM) -f Dockerfile -t ${IMG} ..

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push ${IMG}

# PLATFORMS defines the target platforms for multi-platform builds.
# Default includes common architectures for OpenShift.
# Example: make docker-buildx IMG=quay.io/myorg/operator:v0.0.1
PLATFORMS ?= linux/amd64
.PHONY: docker-buildx
docker-buildx: ## Build and push multi-platform image (requires buildx for docker, manifest for podman)
ifeq ($(CONTAINER_TOOL),podman)
	# Podman multi-platform build using manifest
	@echo "Building multi-platform images with podman..."
	@for platform in $(shell echo $(PLATFORMS) | tr ',' ' '); do \
		echo "Building for $$platform..."; \
		$(CONTAINER_TOOL) build --platform=$$platform -t ${IMG}-$$platform .; \
		$(CONTAINER_TOOL) push ${IMG}-$$platform; \
	done
	$(CONTAINER_TOOL) manifest create ${IMG} $(shell echo $(PLATFORMS) | sed 's/,/ /g' | sed 's~\([^ ]*\)~${IMG}-\1~g')
	$(CONTAINER_TOOL) manifest push ${IMG}
else
	# Docker buildx multi-platform build
	@echo "Building multi-platform images with docker buildx..."
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name operator-builder
	$(CONTAINER_TOOL) buildx use operator-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm operator-builder
	rm Dockerfile.cross
endif

.PHONY: build-installer
build-installer: manifests generate kustomize ## Generate a consolidated YAML with CRDs and deployment.
	mkdir -p dist
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default > dist/install.yaml

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | $(KUBECTL) apply -f -

.PHONY: undeploy
undeploy: kustomize ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

##@ OLM Deployment

# OLM deployment configuration
CATALOG_NAMESPACE ?= openshift-marketplace
OPERATOR_NAMESPACE ?= openshift-operators
SUBSCRIPTION_CHANNEL ?= alpha

.PHONY: install-ai-observability-operator
install-ai-observability-operator: ## Install operator via OLM using custom catalog source
	@echo "=================================================="
	@echo "Installing AI Observability Operator via OLM"
	@echo "=================================================="
	@echo "Catalog Image: $(CATALOG_IMG)"
	@echo "Operator Namespace: $(OPERATOR_NAMESPACE)"
	@echo "Channel: $(SUBSCRIPTION_CHANNEL)"
	@echo ""
	@echo "==> Creating CatalogSource..."
	@printf '%s\n' \
		'apiVersion: operators.coreos.com/v1alpha1' \
		'kind: CatalogSource' \
		'metadata:' \
		'  name: aiobs-operator-catalog' \
		'  namespace: $(CATALOG_NAMESPACE)' \
		'spec:' \
		'  sourceType: grpc' \
		'  image: $(CATALOG_IMG)' \
		'  displayName: AI Observability Operator' \
		'  publisher: Red Hat' \
		'  updateStrategy:' \
		'    registryPoll:' \
		'      interval: 10m' \
		| $(KUBECTL) apply -f - || true
	@echo "✓ CatalogSource created"
	@echo ""
	@echo "==> Waiting for CatalogSource to be ready..."
	@timeout=300; \
	elapsed=0; \
	while [ $$elapsed -lt $$timeout ]; do \
		status=$$($(KUBECTL) get catalogsource aiobs-operator-catalog -n $(CATALOG_NAMESPACE) -o jsonpath='{.status.connectionState.lastObservedState}' 2>/dev/null || echo ""); \
		if [ "$$status" = "READY" ]; then \
			echo "✓ CatalogSource is READY"; \
			break; \
		fi; \
		echo "  Waiting for CatalogSource ($$elapsed/$$timeout seconds)..."; \
		sleep 5; \
		elapsed=$$((elapsed + 5)); \
	done; \
	if [ $$elapsed -ge $$timeout ]; then \
		echo "✗ Timeout waiting for CatalogSource to be ready"; \
		$(KUBECTL) get catalogsource aiobs-operator-catalog -n $(CATALOG_NAMESPACE); \
		$(KUBECTL) describe catalogsource aiobs-operator-catalog -n $(CATALOG_NAMESPACE); \
		exit 1; \
	fi
	@echo ""
	@echo "==> Verifying PackageManifest..."
	@timeout=60; \
	elapsed=0; \
	while [ $$elapsed -lt $$timeout ]; do \
		if $(KUBECTL) get packagemanifest aiobs-operator -n $(CATALOG_NAMESPACE) >/dev/null 2>&1; then \
			echo "✓ PackageManifest found"; \
			$(KUBECTL) get packagemanifest aiobs-operator -n $(CATALOG_NAMESPACE); \
			break; \
		fi; \
		echo "  Waiting for PackageManifest ($$elapsed/$$timeout seconds)..."; \
		sleep 5; \
		elapsed=$$((elapsed + 5)); \
	done; \
	if [ $$elapsed -ge $$timeout ]; then \
		echo "⚠ Warning: PackageManifest not found (may need more time)"; \
	fi
	@echo ""
	@echo "==> Creating Subscription..."
	@printf '%s\n' \
		'apiVersion: operators.coreos.com/v1alpha1' \
		'kind: Subscription' \
		'metadata:' \
		'  name: aiobs-operator' \
		'  namespace: $(OPERATOR_NAMESPACE)' \
		'spec:' \
		'  channel: $(SUBSCRIPTION_CHANNEL)' \
		'  name: aiobs-operator' \
		'  source: aiobs-operator-catalog' \
		'  sourceNamespace: $(CATALOG_NAMESPACE)' \
		'  installPlanApproval: Automatic' \
		| $(KUBECTL) apply -f -
	@echo "✓ Subscription created"
	@echo ""
	@echo "==> Waiting for InstallPlan..."
	@sleep 10
	@installplan=$$($(KUBECTL) get subscription aiobs-operator -n $(OPERATOR_NAMESPACE) -o jsonpath='{.status.installplan.name}' 2>/dev/null || echo ""); \
	if [ -n "$$installplan" ]; then \
		echo "✓ InstallPlan created: $$installplan"; \
		$(KUBECTL) get installplan $$installplan -n $(OPERATOR_NAMESPACE); \
	else \
		echo "⚠ Warning: InstallPlan not yet created (check subscription status)"; \
	fi
	@echo ""
	@echo "==> Waiting for ClusterServiceVersion..."
	@timeout=300; \
	elapsed=0; \
	while [ $$elapsed -lt $$timeout ]; do \
		csv=$$($(KUBECTL) get subscription aiobs-operator -n $(OPERATOR_NAMESPACE) -o jsonpath='{.status.installedCSV}' 2>/dev/null || echo ""); \
		if [ -n "$$csv" ]; then \
			phase=$$($(KUBECTL) get csv $$csv -n $(OPERATOR_NAMESPACE) -o jsonpath='{.status.phase}' 2>/dev/null || echo ""); \
			echo "  CSV: $$csv, Phase: $$phase"; \
			if [ "$$phase" = "Succeeded" ]; then \
				echo "✓ Operator installed successfully!"; \
				break; \
			fi; \
		fi; \
		echo "  Waiting for operator installation ($$elapsed/$$timeout seconds)..."; \
		sleep 10; \
		elapsed=$$((elapsed + 10)); \
	done; \
	if [ $$elapsed -ge $$timeout ]; then \
		echo "✗ Timeout waiting for operator installation"; \
		$(KUBECTL) get subscription aiobs-operator -n $(OPERATOR_NAMESPACE) -o yaml; \
		exit 1; \
	fi
	@echo ""
	@echo "==> Operator Installation Summary"
	@echo "---"
	@$(KUBECTL) get catalogsource aiobs-operator-catalog -n $(CATALOG_NAMESPACE)
	@echo "---"
	@$(KUBECTL) get subscription aiobs-operator -n $(OPERATOR_NAMESPACE)
	@echo "---"
	@csv=$$($(KUBECTL) get subscription aiobs-operator -n $(OPERATOR_NAMESPACE) -o jsonpath='{.status.installedCSV}'); \
	if [ -n "$$csv" ]; then \
		$(KUBECTL) get csv $$csv -n $(OPERATOR_NAMESPACE); \
	fi
	@echo ""
	@echo "=================================================="
	@echo "✓ AI Observability Operator installed via OLM!"
	@echo "=================================================="
	@echo ""
	@echo "Next steps:"
	@echo "  1. Check operator deployment:"
	@echo "     $(KUBECTL) get csv -n $(OPERATOR_NAMESPACE)"
	@echo "     $(KUBECTL) get pods -n $(OPERATOR_NAMESPACE) -l control-plane=controller-manager"
	@echo ""
	@echo "  2. Create AIObservabilitySummarizer CR:"
	@echo "     $(KUBECTL) apply -f config/samples/obs_v1alpha1_aiobservabilitysummarizer.yaml"
	@echo ""

.PHONY: uninstall-ai-observability-operator
uninstall-ai-observability-operator: ## Uninstall operator deployed via OLM
	@echo "Uninstalling AI Observability Operator via OLM..."
	@csv=$$($(KUBECTL) get subscription aiobs-operator -n $(OPERATOR_NAMESPACE) -o jsonpath='{.status.installedCSV}' 2>/dev/null || echo ""); \
	if [ -n "$$csv" ]; then \
		echo "Deleting CSV: $$csv"; \
		$(KUBECTL) delete csv $$csv -n $(OPERATOR_NAMESPACE) --ignore-not-found=true; \
	fi
	@echo "Deleting Subscription..."
	@$(KUBECTL) delete subscription aiobs-operator -n $(OPERATOR_NAMESPACE) --ignore-not-found=true
	@echo "Deleting CatalogSource..."
	@$(KUBECTL) delete catalogsource aiobs-operator-catalog -n $(CATALOG_NAMESPACE) --ignore-not-found=true
	@echo "✓ Operator uninstalled"

##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUBECTL ?= kubectl
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint

## Tool Versions
KUSTOMIZE_VERSION ?= v5.4.3
CONTROLLER_TOOLS_VERSION ?= v0.16.1
ENVTEST_VERSION ?= release-0.19
GOLANGCI_LINT_VERSION ?= v1.59.1

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f $(1) || true ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv $(1) $(1)-$(3) ;\
} ;\
ln -sf $(1)-$(3) $(1)
endef

.PHONY: operator-sdk
OPERATOR_SDK ?= $(LOCALBIN)/operator-sdk
operator-sdk: ## Download operator-sdk locally if necessary.
ifeq (,$(wildcard $(OPERATOR_SDK)))
ifeq (, $(shell which operator-sdk 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPERATOR_SDK)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPERATOR_SDK) https://github.com/operator-framework/operator-sdk/releases/download/$(OPERATOR_SDK_VERSION)/operator-sdk_$${OS}_$${ARCH} ;\
	chmod +x $(OPERATOR_SDK) ;\
	}
else
OPERATOR_SDK = $(shell which operator-sdk)
endif
endif

.PHONY: bundle
bundle: manifests kustomize operator-sdk ## Generate bundle manifests and metadata, then validate generated files.
	$(OPERATOR_SDK) generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | $(OPERATOR_SDK) generate bundle $(BUNDLE_GEN_FLAGS)
	$(OPERATOR_SDK) bundle validate ./bundle

.PHONY: bundle-build
bundle-build: ## Build the bundle image.
	$(CONTAINER_TOOL) build --platform=$(PLATFORM) -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: bundle-push
bundle-push: ## Push the bundle image.
	$(MAKE) docker-push IMG=$(BUNDLE_IMG)

.PHONY: opm
OPM = $(LOCALBIN)/opm
opm: ## Download opm locally if necessary.
ifeq (,$(wildcard $(OPM)))
ifeq (,$(shell which opm 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPM)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPM) https://github.com/operator-framework/operator-registry/releases/download/v1.23.0/$${OS}-$${ARCH}-opm ;\
	chmod +x $(OPM) ;\
	}
else
OPM = $(shell which opm)
endif
endif

# A comma-separated list of bundle images (e.g. make catalog-build BUNDLE_IMGS=example.com/operator-bundle:v0.1.0,example.com/operator-bundle:v0.2.0).
# These images MUST exist in a registry and be pull-able.
BUNDLE_IMGS ?= $(BUNDLE_IMG)

# Set CATALOG_BASE_IMG to an existing catalog image tag to add $BUNDLE_IMGS to that image.
ifneq ($(origin CATALOG_BASE_IMG), undefined)
FROM_INDEX_OPT := --from-index $(CATALOG_BASE_IMG)
endif

# Build a catalog image by adding bundle images to an empty catalog using the operator package manager tool, 'opm'.
# This recipe invokes 'opm' in 'semver' bundle add mode. For more information on add modes, see:
# https://github.com/operator-framework/community-operators/blob/7f1438c/docs/packaging-operator.md#updating-your-existing-operator
.PHONY: catalog-build
catalog-build: opm ## Build a catalog image.
	@echo "Building catalog for platform $(PLATFORM)..."
	@# Pull the platform-specific opm base image first
	$(CONTAINER_TOOL) pull --platform=$(PLATFORM) quay.io/operator-framework/opm:latest
	@# Build catalog using platform-specific base
	$(OPM) index add --build-tool $(CONTAINER_TOOL) --pull-tool $(CONTAINER_TOOL) \
		--mode semver --tag $(CATALOG_IMG) \
		--bundles $(BUNDLE_IMGS) $(FROM_INDEX_OPT) \
		--binary-image quay.io/operator-framework/opm:latest
	@echo "✓ Catalog image built for $(PLATFORM)"

# Push the catalog image.
.PHONY: catalog-push
catalog-push: ## Push a catalog image.
	$(MAKE) docker-push IMG=$(CATALOG_IMG)
