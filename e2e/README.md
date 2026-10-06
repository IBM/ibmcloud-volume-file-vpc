# VPC File CSI Driver — End-to-End Tests

## Overview

The e2e suite uses a **declarative, YAML-driven** approach.
Every test scenario is an entry in [`testcases/testcases.yml`](testcases/testcases.yml).
Adding, skipping, or modifying a test never requires touching Go source code.

```
e2e/
├── e2e.sh                       # CI entry-point — reads Jenkins env vars, runs tests
├── testcases/
│   ├── parser.go                # Go types + LoadTestCases() loader
│   └── testcases.yml            # ← every test case lives here
└── ginkgo_tests/
    ├── declarative_test.go      # single Ginkgo loop — iterates testcases.yml
    └── test_helpers.go          # shared helpers (SC management, EIT utils, …)
```

---

## How test selection works

`testcases.yml` has one entry per test. Optional-feature tests ship with
`wantToSkip: true`. `e2e.sh` reads the Jenkins env vars and generates a
**runtime copy** of the YAML where the relevant entries are flipped to
`wantToSkip: false`. The committed file is never modified.

| Jenkins param | Controls |
|---|---|
| `RUN_EIT_TEST_CASES=true` | All `EIT:` test cases |
| `RUN_REGIONAL_PROFILE_TEST_CASES=true` | All `RFS:` test cases |
| `RUN_SNAPSHOT_TEST_CASES=true` | All `SNAPSHOT:` test cases |
| `RUN_CAPACITY_ROUNDOFF_TEST_CASES=true` | All `ROUNDOFF` test cases |

DP2 and RESIZE test cases always run (they have `wantToSkip: false` in the committed YAML).

---

## Running locally

### Prerequisites

```bash
# Log in and export kubeconfig
ibmcloud login ...
ibmcloud ks cluster config --cluster <cluster-name>

export GOPATH=<your-gopath>
export KUBECONFIG=<absolute-path-to-kubeconfig>
export IC_API_KEY_PROD=<prod-api-key>   # or IC_API_KEY_STAG for stage
export E2E_TEST_RESULT=$GOPATH/src/github.com/IBM/ibmcloud-volume-file-vpc/e2e-test.out
```

### Run all tests (the same way CI does)

```bash
cd $GOPATH/src/github.com/IBM/ibmcloud-volume-file-vpc

IC_LOGIN=true \
TEST_ENV=stage \
IC_REGION=us-south \
./e2e/e2e.sh
```

### Run with optional feature tests enabled

```bash
IC_LOGIN=true \
TEST_ENV=stage \
IC_REGION=us-south \
RUN_EIT_TEST_CASES=true \
RUN_REGIONAL_PROFILE_TEST_CASES=true \
RUN_SNAPSHOT_TEST_CASES=true \
RUN_CAPACITY_ROUNDOFF_TEST_CASES=true \
./e2e/e2e.sh
```

### Run a specific subset directly with ginkgo (developer workflow)

```bash
cd $GOPATH/src/github.com/IBM/ibmcloud-volume-file-vpc

# Run only the EIT entries from a custom YAML file
TEST_CONFIG_FILE=e2e/testcases/my_eit_only.yml \
IC_LOGIN=true \
./e2e/e2e.sh

# Or run the declarative suite directly without e2e.sh
export TEST_CONFIG_FILE=e2e/testcases/testcases.yml
export E2E_TEST_RESULT=/tmp/e2e-test.out
export SC=ibmc-vpc-file-min-iops
export SC_RETAIN=ibmc-vpc-file-retain-500-iops

ginkgo -v -nodes=1 \
  --focus="\[ics-e2e\] \[declarative\]" \
  ./e2e/ginkgo_tests \
  -- -e2e-verify-service-account=false
```


## Environment variables reference

| Variable | Required | Description |
|---|---|---|
| `IC_LOGIN` | Yes | Must be `"true"` — confirms IBM Cloud login is done |
| `TEST_ENV` | Yes | `stage` or `prod` |
| `IC_REGION` | Yes | IBM Cloud region (e.g. `us-south`) |
| `PLATFORM` | No | `iks` or `ocp` — logged in setup output |
| `OS` | No | Worker OS (e.g. `REDHAT_8_64`) — logged in setup output |
| `ADDON_VERSION` | No | CSI driver addon version — logged in setup output |
| `MULTI_ZONE` | No | `true`/`false` — logged in setup output |
| `USE_TRUSTED_PROFILE` | No | `true` to run the trusted-profile identity check |
| `STAGE_TRUSTED_PROFILE_ID` | No | Expected profile ID for stage trusted-profile check |
| `PROD_TRUSTED_PROFILE_ID` | No | Expected profile ID for prod trusted-profile check |
| `E2E_TEST_RESULT` | No | Output file for pass/fail summary (default: `$GOPATH/.../e2e-test.out`) |
| `E2E_TEST_SETUP` | No | Output file for cluster/driver info (default: `$GOPATH/.../e2e-setup.out`) |
| `SC` | No | Default dp2 StorageClass name (default: `ibmc-vpc-file-min-iops`) |
| `SC_RETAIN` | No | Default retain StorageClass name (default: `ibmc-vpc-file-retain-500-iops`) |
| `TEST_CONFIG_FILE` | No | Path to testcases YAML. Set by `e2e.sh` automatically; override for custom suites |
| `BASE_TESTCASES_FILE` | No | Path to the committed source YAML (default: `e2e/testcases/testcases.yml`) |
