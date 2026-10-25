#!/bin/bash

#/******************************************************************************
# Copyright 2024 IBM Corp.
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
# *****************************************************************************/

# ---------------------------------------------------------------------------
# Declarative e2e runner for vpc-file-csi-driver
#
# HOW TEST SELECTION WORKS
# ────────────────────────
# The committed file  e2e/testcases/testcases.yml  lists every test case.
# By default every optional-feature test has  wantToSkip: true.
# This script reads the same Jenkins env vars that used to drive separate
# ginkgo --focus calls, and generates a RUNTIME copy of the YAML where the
# relevant entries are flipped to  wantToSkip: false.
# The Go test binary reads that runtime copy via TEST_CONFIG_FILE.
# Nothing in the committed YAML file is ever modified.
#
# Jenkins env vars consumed
# ─────────────────────────
#   RUN_EIT_TEST_CASES              "true" → enable EIT test cases
#   RUN_REGIONAL_PROFILE_TEST_CASES "true" → enable RFS test cases
#   RUN_SNAPSHOT_TEST_CASES         "true" → enable snapshot test cases
#   RUN_CAPACITY_ROUNDOFF_TEST_CASES "true" → enable capacity roundoff test cases
#
#   TEST_ENV          stage | prod
#   IC_REGION         e.g. us-south
#   PLATFORM          iks | ocp
#   OS                e.g. REDHAT_8_64, UBUNTU_22_64
#   ADDON_VERSION     vpc-file-csi-driver addon version
#   MULTI_ZONE        true | false
#   USE_TRUSTED_PROFILE true | false
#
#   IC_LOGIN          Must be "true" — confirms IBM Cloud login is already done
#
# Outputs
# ───────
#   E2E_TEST_RESULT   Pass/fail summary lines (default: $GOPATH/.../e2e-test.out)
#   E2E_TEST_SETUP    Cluster/driver setup info (default: $GOPATH/.../e2e-setup.out)
# ---------------------------------------------------------------------------

set -euo pipefail

GOPATH="${GOPATH:-$(go env GOPATH)}"
VPC_FILE_CSI_HOME="$GOPATH/src/github.com/IBM/ibmcloud-volume-file-vpc"
E2E_TEST_RESULT="${E2E_TEST_RESULT:-$VPC_FILE_CSI_HOME/e2e-test.out}"
E2E_TEST_SETUP="${E2E_TEST_SETUP:-$VPC_FILE_CSI_HOME/e2e-setup.out}"

# Source YAML — committed, never modified
BASE_TESTCASES_FILE="${BASE_TESTCASES_FILE:-$VPC_FILE_CSI_HOME/e2e/testcases/testcases.yml}"

# Jenkins feature-flag env vars (default off = keep committed wantToSkip values)
RUN_EIT_TEST_CASES="${RUN_EIT_TEST_CASES:-false}"
RUN_REGIONAL_PROFILE_TEST_CASES="${RUN_REGIONAL_PROFILE_TEST_CASES:-false}"
RUN_SNAPSHOT_TEST_CASES="${RUN_SNAPSHOT_TEST_CASES:-false}"
RUN_CAPACITY_ROUNDOFF_TEST_CASES="${RUN_CAPACITY_ROUNDOFF_TEST_CASES:-false}"

# Context vars forwarded to the Go test binary for setup-log enrichment
export TEST_ENV="${TEST_ENV:-}"
export IC_REGION="${IC_REGION:-}"
export PLATFORM="${PLATFORM:-}"
export OS="${OS:-}"
export ADDON_VERSION="${ADDON_VERSION:-}"
export MULTI_ZONE="${MULTI_ZONE:-false}"
export USE_TRUSTED_PROFILE="${USE_TRUSTED_PROFILE:-false}"
export STAGE_TRUSTED_PROFILE_ID="${STAGE_TRUSTED_PROFILE_ID:-}"
export PROD_TRUSTED_PROFILE_ID="${PROD_TRUSTED_PROFILE_ID:-}"

export E2E_TEST_RESULT E2E_TEST_SETUP

rm -f "$E2E_TEST_RESULT" "$E2E_TEST_SETUP"

SECRET_CREATION_WAIT=600

echo "**********VPC-File-Volume-Tests**********"                 >  "$E2E_TEST_RESULT"
echo "******** E2E Test Details ********"                        >  "$E2E_TEST_SETUP"
echo "StartTime   : $(date '+%F-%T')"                            >> "$E2E_TEST_SETUP"
echo "Region      : ${IC_REGION:-}"                             >> "$E2E_TEST_SETUP"
echo "Environment : ${TEST_ENV:-}"                              >> "$E2E_TEST_SETUP"
echo "Platform    : ${PLATFORM:-} / OS: ${OS:-}"               >> "$E2E_TEST_SETUP"
echo "Addon Ver   : ${ADDON_VERSION:-}"                         >> "$E2E_TEST_SETUP"
echo "Multi-zone  : ${MULTI_ZONE:-false}"                       >> "$E2E_TEST_SETUP"
echo "EIT tests   : ${RUN_EIT_TEST_CASES}"                      >> "$E2E_TEST_SETUP"
echo "RFS tests   : ${RUN_REGIONAL_PROFILE_TEST_CASES}"         >> "$E2E_TEST_SETUP"
echo "Snapshot    : ${RUN_SNAPSHOT_TEST_CASES}"                 >> "$E2E_TEST_SETUP"
echo "Roundoff    : ${RUN_CAPACITY_ROUNDOFF_TEST_CASES}"        >> "$E2E_TEST_SETUP"

# ---------------------------------------------------------------------------
# Guard: require IBM Cloud login
# ---------------------------------------------------------------------------
IC_LOGIN="${IC_LOGIN:-false}"
if [[ "$IC_LOGIN" != "true" ]]; then
  echo "Error: Not logged into IBM Cloud!!!"
  echo "VPC-FILE-CSI-TEST: Cluster-Setup: FAILED" >> "$E2E_TEST_RESULT"
  exit 1
fi

# ---------------------------------------------------------------------------
# Trusted-profile check
# ---------------------------------------------------------------------------
wait_for_secret() {
  local elapsed=0
  echo "Waiting up to ${SECRET_CREATION_WAIT}s for ibm-cloud-credentials..."
  while [[ $elapsed -lt $SECRET_CREATION_WAIT ]]; do
    if kubectl get secret ibm-cloud-credentials -n kube-system &>/dev/null; then
      echo "ibm-cloud-credentials found."
      return 0
    fi
    sleep 5
    ((elapsed += 5))
  done
  echo "ibm-cloud-credentials not found after ${SECRET_CREATION_WAIT}s."
  return 1
}

check_trusted_profile_status() {
  if [[ "${USE_TRUSTED_PROFILE:-}" != "true" ]]; then return 0; fi
  local expected_profile_id=""
  if [[ "${TEST_ENV:-}" == "stage" ]]; then
    expected_profile_id="${STAGE_TRUSTED_PROFILE_ID:-}"
  else
    expected_profile_id="${PROD_TRUSTED_PROFILE_ID:-}"
  fi
  echo "************************ Trusted Profile Check ***************************" >> "$E2E_TEST_SETUP"
  wait_for_secret
  local encoded decoded profileID
  encoded=$(kubectl get secret ibm-cloud-credentials -n kube-system \
    -o jsonpath='{.data.ibm-credentials\.env}')
  decoded=$(base64 --decode <<< "$encoded")
  profileID=$(grep IBMCLOUD_PROFILEID <<< "$decoded" | cut -d'=' -f3-)
  if [[ "$profileID" == "$expected_profile_id" ]]; then
    echo "VPC-FILE-CSI-TEST: USING TRUSTED_PROFILE: TRUE" >> "$E2E_TEST_SETUP"
  else
    echo "VPC-FILE-CSI-TEST: USING TRUSTED_PROFILE: FAILED" >> "$E2E_TEST_SETUP"
    exit 1
  fi
  echo "***************************************************" >> "$E2E_TEST_SETUP"
}

# ---------------------------------------------------------------------------
# Cluster connectivity & driver health checks
# ---------------------------------------------------------------------------
CLUSTER_DETAIL=$(kubectl get cm cluster-info -n kube-system \
  -o jsonpath='{.data.cluster-config\.json}' \
  | grep -v -e 'crn' -e 'master_public_url' -e 'master_url') || {
  echo "Error: Unable to connect to cluster" >> "$E2E_TEST_SETUP"
  echo "VPC-FILE-CSI-TEST: Cluster-Setup: FAILED" >> "$E2E_TEST_RESULT"
  exit 1
}

check_trusted_profile_status

CLUSTER_KUBE_DETAIL=$(kubectl get nodes \
  -o jsonpath="{range .items[*]}{.metadata.name}:{.status.nodeInfo.kubeletVersion}:{.status.nodeInfo.osImage} {'\n'}")

{
  echo "***************** Cluster Details ******************"
  echo "$CLUSTER_DETAIL"
  echo "----------------------------------------------------"
  echo "$CLUSTER_KUBE_DETAIL"
  echo "----------------------------------------------------"
  echo "Addon Version: ${CLUSTER_ADDON_VER:-${ADDON_VERSION:-}}"
  echo "***************************************************"
} >> "$E2E_TEST_SETUP"

CONTROLLER_POD=$(kubectl get pods -n kube-system 2>/dev/null \
  | grep 'ibm-vpc-file-csi-controller' | grep 'Running' || true)
NODE_POD=$(kubectl get pods -n kube-system 2>/dev/null \
  | grep 'ibm-vpc-file-csi-node' | grep 'Running' || true)

if [[ -z "$CONTROLLER_POD" || -z "$NODE_POD" ]]; then
  echo "VPC-FILE-CSI-TEST: VERIFYING VPC FILE CSI DRIVER HEALTH: FAILED" >> "$E2E_TEST_SETUP"
  echo "VPC-FILE-CSI-TEST: VPC-File-Volume-Tests: FAILED" >> "$E2E_TEST_RESULT"
  exit 1
fi
echo "VPC-FILE-CSI-TEST: VERIFYING VPC FILE CSI DRIVER HEALTH: PASS" >> "$E2E_TEST_SETUP"

# ---------------------------------------------------------------------------
# Generate runtime testcases file
#
# Strategy: copy the committed YAML, then use sed to flip wantToSkip for
# each feature group that Jenkins has enabled.  We identify groups by the
# comment tags embedded in testcases.yml (# group: eit, # group: rfs, etc.)
# and flip only those entries.
#
# Because plain sed can't do conditional multi-line YAML edits reliably,
# we use Python (always available in Jenkins) to do a simple in-memory
# rewrite: parse the YAML, walk every test-case entry, check its group
# annotation (a top-level comment or the testCase name prefix), and set
# wantToSkip accordingly.
# ---------------------------------------------------------------------------

RUNTIME_TESTCASES="${TMPDIR:-/tmp}/vpc_file_e2e_testcases_$$.yml"

python3 - "$BASE_TESTCASES_FILE" "$RUNTIME_TESTCASES" \
  "$RUN_EIT_TEST_CASES" \
  "$RUN_REGIONAL_PROFILE_TEST_CASES" \
  "$RUN_SNAPSHOT_TEST_CASES" \
  "$RUN_CAPACITY_ROUNDOFF_TEST_CASES" <<'EOF'
import sys, re

src, dst = sys.argv[1], sys.argv[2]
enable_eit      = sys.argv[3].lower() == "true"
enable_rfs      = sys.argv[4].lower() == "true"
enable_snapshot = sys.argv[5].lower() == "true"
enable_roundoff = sys.argv[6].lower() == "true"

# Map a testCase name prefix to the feature flag that controls it.
# Uses simple prefix matching against the testCase value.
FEATURE_MAP = {
    "EIT:":      enable_eit,
    "RFS:":      enable_rfs,
    "SNAPSHOT:": enable_snapshot,
    "ROUNDOFF":  enable_roundoff,
}

with open(src) as f:
    lines = f.readlines()

out = []
# Track whether we are inside a test-case block whose wantToSkip should be forced.
force_skip_value = None  # None = no override; True/False = override

tc_name_re = re.compile(r'^- testCase:\s*["\']?(.+?)["\']?\s*$')
skip_re     = re.compile(r'^(\s+wantToSkip:\s*)(\S+)\s*$')

for line in lines:
    m = tc_name_re.match(line)
    if m:
        name = m.group(1)
        force_skip_value = None   # reset for each new test case
        for prefix, enabled in FEATURE_MAP.items():
            if name.startswith(prefix):
                # enabled=True  → Jenkins wants this test ON  → wantToSkip=false
                # enabled=False → leave as committed (no override)
                force_skip_value = not enabled  # True means skip
                break
        out.append(line)
        continue

    m = skip_re.match(line)
    if m and force_skip_value is not None:
        # Rewrite wantToSkip with the Jenkins-derived value.
        out.append(m.group(1) + str(force_skip_value).lower() + "\n")
        continue

    out.append(line)

with open(dst, "w") as f:
    f.writelines(out)

print(f"[e2e.sh] runtime testcases written to {dst}")
EOF

echo "Test config : $RUNTIME_TESTCASES" >> "$E2E_TEST_SETUP"

# Show which tests are active so the build log is self-documenting.
echo "--- Active test cases (wantToSkip: false) ---" >> "$E2E_TEST_SETUP"
grep -B1 'wantToSkip: false' "$RUNTIME_TESTCASES" \
  | grep 'testCase:' | sed 's/.*testCase: /  /' >> "$E2E_TEST_SETUP" || true
echo "---------------------------------------------" >> "$E2E_TEST_SETUP"

# Export so the Go binary picks it up.
export TEST_CONFIG_FILE="$RUNTIME_TESTCASES"
export SC="${SC:-ibmc-vpc-file-min-iops}"
export SC_RETAIN="${SC_RETAIN:-ibmc-vpc-file-retain-500-iops}"

# ---------------------------------------------------------------------------
# Run the full declarative suite — one invocation, no --focus per feature
# ---------------------------------------------------------------------------
go clean -modcache
export GO111MODULE=on
go install -mod=mod github.com/onsi/ginkgo/v2/ginkgo@v2.27.2

set +e
ginkgo -v -nodes=1 \
  --focus="\[ics-e2e\] \[declarative\]" \
  ./e2e/ginkgo_tests \
  -- -e2e-verify-service-account=false
overall_rc=$?
set -e

rm -f "$RUNTIME_TESTCASES"

if [[ $overall_rc -eq 0 ]]; then
  echo "VPC-FILE-CSI-TEST: VPC-File-Volume-Tests: PASS" >> "$E2E_TEST_RESULT"
else
  echo "VPC-FILE-CSI-TEST: VPC-File-Volume-Tests: FAILED" >> "$E2E_TEST_RESULT"
fi

exit $overall_rc
