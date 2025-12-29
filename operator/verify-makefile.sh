#!/bin/bash
# Verify Makefile Configuration for Podman & AMD64 Builds
# Run this script to validate the Makefile setup before deploying to OpenShift

set -e

echo "=================================================="
echo "Makefile Configuration Verification"
echo "=================================================="
echo ""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test counter
TESTS_PASSED=0
TESTS_FAILED=0

# Helper function for tests
test_check() {
    local test_name="$1"
    local test_cmd="$2"

    echo -n "Testing: $test_name ... "
    if eval "$test_cmd" > /dev/null 2>&1; then
        echo -e "${GREEN}✓ PASS${NC}"
        ((TESTS_PASSED++))
        return 0
    else
        echo -e "${RED}✗ FAIL${NC}"
        ((TESTS_FAILED++))
        return 1
    fi
}

# 1. Check podman is installed
echo "==> Checking Prerequisites"
test_check "Podman installed" "which podman"

if which podman > /dev/null 2>&1; then
    PODMAN_VERSION=$(podman --version | awk '{print $3}')
    echo -e "    ${GREEN}Podman version: ${PODMAN_VERSION}${NC}"
fi

# 2. Check podman machine (Mac only)
if [[ "$OSTYPE" == "darwin"* ]]; then
    echo ""
    echo "==> Checking Podman Machine (Mac)"

    if podman machine ls 2>/dev/null | grep -q "running"; then
        echo -e "    ${GREEN}✓ Podman machine is running${NC}"
        ((TESTS_PASSED++))
    else
        echo -e "    ${YELLOW}⚠ Podman machine not running${NC}"
        echo "    Run: podman machine init && podman machine start"
        ((TESTS_FAILED++))
    fi
fi

# 3. Check Makefile variables
echo ""
echo "==> Checking Makefile Configuration"

# Extract CONTAINER_TOOL default
CONTAINER_TOOL_DEFAULT=$(grep -m1 "^CONTAINER_TOOL ?=" Makefile | awk '{print $3}')
if [[ "$CONTAINER_TOOL_DEFAULT" == "podman" ]]; then
    echo -e "    ${GREEN}✓ CONTAINER_TOOL defaults to podman${NC}"
    ((TESTS_PASSED++))
else
    echo -e "    ${RED}✗ CONTAINER_TOOL is: ${CONTAINER_TOOL_DEFAULT} (expected: podman)${NC}"
    ((TESTS_FAILED++))
fi

# Extract PLATFORM default
PLATFORM_DEFAULT=$(grep -m1 "^PLATFORM ?=" Makefile | awk '{print $3}')
if [[ "$PLATFORM_DEFAULT" == "linux/amd64" ]]; then
    echo -e "    ${GREEN}✓ PLATFORM defaults to linux/amd64${NC}"
    ((TESTS_PASSED++))
else
    echo -e "    ${RED}✗ PLATFORM is: ${PLATFORM_DEFAULT} (expected: linux/amd64)${NC}"
    ((TESTS_FAILED++))
fi

# 4. Check build targets
echo ""
echo "==> Checking Build Targets"

# Check docker-build uses PLATFORM
if grep -A1 "^docker-build:" Makefile | grep -q "build --platform=\$(PLATFORM)"; then
    echo -e "    ${GREEN}✓ docker-build uses --platform flag${NC}"
    ((TESTS_PASSED++))
else
    echo -e "    ${RED}✗ docker-build missing --platform flag${NC}"
    ((TESTS_FAILED++))
fi

# Check bundle-build uses CONTAINER_TOOL and PLATFORM
if grep -A1 "^bundle-build:" Makefile | grep -q "build --platform=\$(PLATFORM)"; then
    echo -e "    ${GREEN}✓ bundle-build uses CONTAINER_TOOL and PLATFORM${NC}"
    ((TESTS_PASSED++))
else
    echo -e "    ${RED}✗ bundle-build not properly configured${NC}"
    ((TESTS_FAILED++))
fi

# Check catalog-build uses CONTAINER_TOOL
if grep -A1 "^catalog-build:" Makefile | grep -q "container-tool \$(CONTAINER_TOOL)"; then
    echo -e "    ${GREEN}✓ catalog-build uses CONTAINER_TOOL${NC}"
    ((TESTS_PASSED++))
else
    echo -e "    ${RED}✗ catalog-build not properly configured${NC}"
    ((TESTS_FAILED++))
fi

# 5. Check operator code compiles
echo ""
echo "==> Checking Operator Code"

if go build -o /tmp/manager-test cmd/main.go 2>/dev/null; then
    echo -e "    ${GREEN}✓ Operator code compiles${NC}"
    rm -f /tmp/manager-test
    ((TESTS_PASSED++))
else
    echo -e "    ${RED}✗ Operator code has compilation errors${NC}"
    ((TESTS_FAILED++))
fi

# 6. Check CRD manifests are up-to-date
echo ""
echo "==> Checking CRD Manifests"

if ls config/crd/bases/obs.redhat.com_aiobservabilitysummarizers.yaml > /dev/null 2>&1; then
    echo -e "    ${GREEN}✓ CRD manifest exists${NC}"
    ((TESTS_PASSED++))
else
    echo -e "    ${YELLOW}⚠ CRD manifest not found (run: make manifests)${NC}"
    ((TESTS_FAILED++))
fi

# 7. Test Makefile syntax
echo ""
echo "==> Testing Makefile Syntax"

if make help > /dev/null 2>&1; then
    echo -e "    ${GREEN}✓ Makefile syntax is valid${NC}"
    ((TESTS_PASSED++))
else
    echo -e "    ${RED}✗ Makefile has syntax errors${NC}"
    ((TESTS_FAILED++))
fi

# 8. Verify required tools
echo ""
echo "==> Checking Required Tools"

declare -a tools=("go" "kubectl" "make")
for tool in "${tools[@]}"; do
    if which $tool > /dev/null 2>&1; then
        VERSION=$($tool version 2>&1 | head -1)
        echo -e "    ${GREEN}✓ $tool installed${NC}"
        ((TESTS_PASSED++))
    else
        echo -e "    ${RED}✗ $tool not found${NC}"
        ((TESTS_FAILED++))
    fi
done

# Summary
echo ""
echo "=================================================="
echo "Verification Summary"
echo "=================================================="
echo -e "Tests Passed: ${GREEN}${TESTS_PASSED}${NC}"
echo -e "Tests Failed: ${RED}${TESTS_FAILED}${NC}"
echo ""

if [[ $TESTS_FAILED -eq 0 ]]; then
    echo -e "${GREEN}✓ All checks passed! Ready to build and deploy.${NC}"
    echo ""
    echo "Next steps:"
    echo "  1. Set environment variables:"
    echo "     export IMG=quay.io/<your-org>/aiobs-operator:v0.0.1"
    echo "  2. Build operator image:"
    echo "     make docker-build IMG=\${IMG}"
    echo "  3. Push to registry:"
    echo "     make docker-push IMG=\${IMG}"
    echo "  4. Deploy to OpenShift:"
    echo "     make deploy IMG=\${IMG}"
    echo ""
    exit 0
else
    echo -e "${RED}✗ Some checks failed. Please fix the issues above.${NC}"
    echo ""
    exit 1
fi
