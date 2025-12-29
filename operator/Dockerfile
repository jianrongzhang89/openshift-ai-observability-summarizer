# Build the manager binary
FROM --platform=$BUILDPLATFORM golang:1.22 AS builder
ARG TARGETOS
ARG TARGETARCH
ARG BUILDPLATFORM
ARG TARGETPLATFORM

WORKDIR /workspace
# Copy the Go Modules manifests
COPY operator/go.mod go.mod
COPY operator/go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
# Use native platform for go mod download to avoid QEMU emulation issues
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy the go source
COPY operator/cmd/main.go cmd/main.go
COPY operator/api/ api/
COPY operator/internal/ internal/

# Build
# Cross-compile for the target platform
# TARGETOS and TARGETARCH are set automatically by BuildKit when using --platform
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -a -ldflags="-w -s" -o manager cmd/main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
# Use target platform for the final image
FROM --platform=$TARGETPLATFORM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
# Copy Helm charts into the container
COPY --chown=65532:65532 deploy/helm /helm
USER 65532:65532

ENTRYPOINT ["/manager"]
