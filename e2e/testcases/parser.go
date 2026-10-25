/**
 * Copyright 2024 IBM Corp.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package testcases provides types and a loader for the declarative YAML-driven
// e2e test suite for the vpc-file-csi-driver. Each YAML entry maps 1:1 to a
// Ginkgo It() node so that adding, skipping, or modifying a test never requires
// touching Go source code.
package testcases

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// Top-level test case
// ---------------------------------------------------------------------------

// TestCase is the root object for every entry in testcases.yml.
type TestCase struct {
	// TestCase is the human-readable name shown in Ginkgo output.
	TestCase string `yaml:"testCase"`
	// Description is an optional free-text explanation of the scenario.
	Description string `yaml:"description,omitempty"`
	// Input holds all parameters that drive test execution.
	Input Input `yaml:"input"`
}

// ---------------------------------------------------------------------------
// Input — everything a test case declares
// ---------------------------------------------------------------------------

// Input groups all parameters that control what a test does.
type Input struct {
	// StorageClass is the name of the pre-existing Kubernetes StorageClass to
	// use for PVC provisioning (e.g. "ibmc-vpc-file-min-iops").
	// Leave empty to use a CustomSC instead.
	StorageClass string `yaml:"storageClass,omitempty"`

	// CustomSC, when set, causes the runner to create a StorageClass on the
	// fly with the supplied provisioner parameters before the test runs.
	CustomSC *CustomStorageClass `yaml:"customSC,omitempty"`

	// PVC describes the PersistentVolumeClaim to create.
	PVC PVCSpec `yaml:"pvc"`

	// Workload describes the Kubernetes workload (pod/deployment/daemonset/
	// statefulset) that mounts the PVC.
	Workload *WorkloadSpec `yaml:"workload,omitempty"`

	// Resize, when set, adds an online volume expansion assertion after the
	// workload is running.
	Resize *ResizeSpec `yaml:"resize,omitempty"`

	// Snapshot, when set, creates a VolumeSnapshot and then restores it into
	// one or more new PVCs to verify data integrity.
	Snapshot *SnapshotSpec `yaml:"snapshot,omitempty"`

	// EIT configures the Encrypted-in-Transit feature for the test.
	EIT *EITSpec `yaml:"eit,omitempty"`

	// CapacityRoundoff, when set, verifies the allowCapacityRoundoff
	// behaviour of a StorageClass.
	CapacityRoundoff *CapacityRoundoffSpec `yaml:"capacityRoundoff,omitempty"`

	// TCExpectation declares whether this test case is expected to succeed
	// (true) or fail (false). Defaults to true.
	TCExpectation bool `yaml:"testCaseExpectation"`

	// WantToSkip causes the test to be registered but immediately skipped.
	// Replaces the old Jenkins boolean params (e2e_eit_test_case, etc.).
	WantToSkip bool `yaml:"wantToSkip"`
}

// ---------------------------------------------------------------------------
// StorageClass helpers
// ---------------------------------------------------------------------------

// CustomStorageClass instructs the runner to create a StorageClass before the
// test and delete it afterwards.
type CustomStorageClass struct {
	// Name is the StorageClass name to create.
	Name string `yaml:"name"`
	// Params are the key/value parameters passed to the vpc.file.csi.ibm.io
	// provisioner (e.g. profile, throughput, iops, zone).
	Params map[string]string `yaml:"params"`
}

// ---------------------------------------------------------------------------
// PVC spec
// ---------------------------------------------------------------------------

// PVCSpec describes the PVC to create for a test.
type PVCSpec struct {
	// PVCNamePrefix is prepended to a generated suffix (e.g. "ics-vol-dp2-").
	PVCNamePrefix string `yaml:"pvcNamePrefix"`
	// ClaimSize is the requested storage capacity (e.g. "15Gi").
	ClaimSize string `yaml:"claimSize"`
	// AccessMode is "ReadWriteMany", "ReadWriteOnce", or "ReadOnlyMany".
	// Defaults to ReadWriteMany.
	AccessMode string `yaml:"accessMode,omitempty"`
	// ReclaimPolicy is "Delete" or "Retain". Defaults to Delete.
	ReclaimPolicy string `yaml:"reclaimPolicy,omitempty"`
	// MountOptions are passed through to the PVC/PV (e.g. ["rw"]).
	MountOptions []string `yaml:"mountOptions,omitempty"`
	// MountPath is the container path where the volume is mounted
	// (e.g. "/mnt/test-1"). Defaults to "/mnt/test-1".
	MountPath string `yaml:"mountPath,omitempty"`
}

// ---------------------------------------------------------------------------
// Workload spec
// ---------------------------------------------------------------------------

// WorkloadSpec describes the Kubernetes workload that exercises the PVC.
type WorkloadSpec struct {
	// Type is one of: "deployment", "pod", "daemonset", "statefulset".
	Type string `yaml:"type"`
	// Replicas is the desired replica count (deployment/statefulset only).
	Replicas int32 `yaml:"replicas,omitempty"`
	// Cmd is the shell command run inside the container.
	Cmd string `yaml:"cmd"`
	// ReadCmd is the command used to verify data written by Cmd.
	ReadCmd []string `yaml:"readCmd,omitempty"`
	// ExpectedOutput01 is checked on the first read.
	ExpectedOutput01 string `yaml:"expectedOutput01,omitempty"`
	// ExpectedOutput02 is checked on the second read (after a re-mount).
	ExpectedOutput02 string `yaml:"expectedOutput02,omitempty"`
	// NodeSelector constrains the workload to specific nodes
	// (e.g. {"ibm-cloud.kubernetes.io/worker-pool-name": "default"}).
	NodeSelector map[string]string `yaml:"nodeSelector,omitempty"`
}

// ---------------------------------------------------------------------------
// Resize spec
// ---------------------------------------------------------------------------

// ResizeSpec instructs the runner to expand the PVC after the workload starts.
type ResizeSpec struct {
	// ExpandToGi is the new size in GiB to request.
	ExpandToGi int64 `yaml:"expandToGi"`
	// ExpectedGi is the size in GiB that should be reported after expansion.
	ExpectedGi int64 `yaml:"expectedGi"`
}

// ---------------------------------------------------------------------------
// Snapshot spec
// ---------------------------------------------------------------------------

// SnapshotSpec instructs the runner to snapshot the PVC and restore it.
type SnapshotSpec struct {
	// RestoreClaimSizes lists the PVC sizes to use for each restore attempt
	// (e.g. ["20Gi", "10Gi", "30Gi"] for same/less/more).
	RestoreClaimSizes []string `yaml:"restoreClaimSizes"`
	// RestoredCmd is run in the pod mounted to the restored PVC to verify
	// data integrity (e.g. "grep 'hello world' /mnt/test-1/data && while true; do sleep 2; done").
	RestoredCmd string `yaml:"restoredCmd,omitempty"`
}

// ---------------------------------------------------------------------------
// EIT spec
// ---------------------------------------------------------------------------

// EITSpec carries Encrypted-in-Transit configuration for the test.
type EITSpec struct {
	// WorkerPool is the worker-pool label value to enable EIT on
	// (e.g. "default").
	WorkerPool string `yaml:"workerPool"`
	// ExpectMountFail, when true, asserts that pod mounting should fail
	// (negative test: EIT SC used on a pool that has EIT disabled).
	ExpectMountFail bool `yaml:"expectMountFail,omitempty"`
}

// ---------------------------------------------------------------------------
// CapacityRoundoff spec
// ---------------------------------------------------------------------------

// CapacityRoundoffSpec verifies the allowCapacityRoundoff behaviour.
type CapacityRoundoffSpec struct {
	// RequestedCapacity is what the PVC asks for (e.g. "60Gi").
	RequestedCapacity string `yaml:"requestedCapacity"`
	// ExpectedCapacity is what the PV/PVC should actually provision
	// (e.g. "80Gi"). Set equal to RequestedCapacity when no roundoff is
	// expected.
	ExpectedCapacity string `yaml:"expectedCapacity,omitempty"`
	// ExpectProvisionFail, when true, asserts that the PVC must NOT reach
	// Bound state (negative test).
	ExpectProvisionFail bool `yaml:"expectProvisionFail,omitempty"`
}

// ---------------------------------------------------------------------------
// Loader
// ---------------------------------------------------------------------------

// LoadTestCases reads a YAML file at path and returns the parsed test cases.
// path may be absolute or relative to the working directory.
func LoadTestCases(path string) ([]TestCase, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read test cases file %s: %w", path, err)
	}

	var tcs []TestCase
	if err := yaml.Unmarshal(data, &tcs); err != nil {
		return nil, fmt.Errorf("failed to parse test cases from %s: %w", path, err)
	}
	return tcs, nil
}
