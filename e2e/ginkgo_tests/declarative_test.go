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

// declarative_test.go is the single entry-point for all YAML-driven e2e tests.
//
// Usage:
//
//	# run every enabled test case from the default file
//	go test ./e2e/ginkgo_tests -v -timeout 120m
//
//	# switch to a different suite without touching Go source
//	TEST_CONFIG_FILE=e2e/testcases/testcases_eit_only.yml go test ./e2e/ginkgo_tests -v -timeout 120m
//
// How it works
//
//  1. At suite initialisation LoadTestCases reads the YAML file specified by
//     TEST_CONFIG_FILE (default: e2e/testcases/testcases.yml).
//  2. A single Ginkgo Context iterates the slice and creates one It() per entry.
//  3. If wantToSkip: true the test is registered but immediately skipped — no
//     Jenkins boolean parameter nor --focus flag needed.
//  4. The dispatcher (runTestCase) reads the Input fields and delegates to the
//     same testsuites helpers that the original per-feature *_test.go files used.
package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/IBM/ibmcloud-volume-file-vpc/e2e/testcases"
	"github.com/IBM/ibmcloud-volume-file-vpc/e2e/testsuites"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
	"gopkg.in/yaml.v3"
)

// defaultTestConfigFile is used when TEST_CONFIG_FILE is not set.
const defaultTestConfigFile = "e2e/testcases/testcases.yml"

// ---------------------------------------------------------------------------
// Suite registration
// ---------------------------------------------------------------------------

var _ = Describe("[ics-e2e] [declarative] VPC File CSI — YAML-driven test suite", func() {
	// Load the test case file once at suite setup time.
	testConfigFile := os.Getenv("TEST_CONFIG_FILE")
	if testConfigFile == "" {
		testConfigFile = defaultTestConfigFile
	}

	tcs, loadErr := testcases.LoadTestCases(testConfigFile)
	if loadErr != nil {
		// Fail loudly so the suite abort is obvious in CI logs.
		panic(fmt.Sprintf("declarative_test: failed to load %s: %v", testConfigFile, loadErr))
	}

	fmt.Printf("[declarative] Loaded %d test cases from %s\n", len(tcs), testConfigFile)
	fmt.Printf("[declarative] Platform=%s OS=%s Region=%s Env=%s AddonVer=%s MultiZone=%s\n",
		os.Getenv("PLATFORM"), os.Getenv("OS"),
		os.Getenv("IC_REGION"), os.Getenv("TEST_ENV"),
		os.Getenv("ADDON_VERSION"), os.Getenv("MULTI_ZONE"),
	)

	// One Ginkgo framework per Describe block; individual tests share it.
	f := framework.NewDefaultFramework("ics-e2e-declarative")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	Context("test cases", func() {
		for i := range tcs {
			tc := tcs[i] // capture loop variable

			It(tc.TestCase, func() {
				if tc.Input.WantToSkip {
					Skip("wantToSkip=true in testcases.yml")
				}

				cs := f.ClientSet
				ns := f.Namespace

				// Grant anyuid SCC on OpenShift so pods can run privileged.
				DeferCleanup(grantSCCToServiceAccount(cs, ns.Name))

				// Apply the standard OpenShift namespace labels.
				patchNamespaceLabels(cs, ns.Name)

				// Manage custom StorageClass lifecycle if declared.
				if tc.Input.CustomSC != nil {
					sc, err := createCustomRfsSC(cs, tc.Input.CustomSC.Name, tc.Input.CustomSC.Params)
					Expect(err).NotTo(HaveOccurred(),
						"failed to create custom StorageClass %s", tc.Input.CustomSC.Name)
					DeferCleanup(func() {
						deleteCustomRfsSC(cs, sc.Name) //nolint:errcheck
					})
				}

				runTestCase(tc, cs, ns)
			})
		}
	})
})

// ---------------------------------------------------------------------------
// Top-level dispatcher
// ---------------------------------------------------------------------------

// runTestCase reads the Input fields of tc and delegates to the appropriate
// test runner.  The order of checks mirrors the original per-file suites so
// that behaviour is unchanged.
func runTestCase(tc testcases.TestCase, cs clientset.Interface, ns *v1.Namespace) {
	in := tc.Input

	switch {
	case in.EIT != nil && in.EIT.WorkerPool != "" && in.Workload == nil && in.CapacityRoundoff == nil:
		// EIT installation-only check (no workload).
		runEITInstallationCheck(in, cs)

	case in.CapacityRoundoff != nil:
		runCapacityRoundoffTest(in, cs, ns)

	case in.Snapshot != nil:
		runSnapshotTest(tc, cs, ns)

	case in.EIT != nil && in.EIT.ExpectMountFail:
		// Negative EIT test: ensure mount fails when EIT is disabled.
		runEITMountFailTest(in, cs, ns)

	case in.Resize != nil:
		runResizeTest(in, cs, ns)

	default:
		// Generic workload test (deployment / pod / daemonset / statefulset).
		runWorkloadTest(in, cs, ns, tc.TestCase, tc.Input.TCExpectation)
	}
}

// ---------------------------------------------------------------------------
// Helpers: namespace patch & result file
// ---------------------------------------------------------------------------

func patchNamespaceLabels(cs clientset.Interface, nsName string) {
	payload := `{"metadata": {"labels": {"security.openshift.io/scc.podSecurityLabelSync": "false","pod-security.kubernetes.io/enforce": "privileged"}}}`
	_, err := cs.CoreV1().Namespaces().Patch(
		context.TODO(), nsName,
		types.StrategicMergePatchType, []byte(payload), metav1.PatchOptions{})
	if err != nil {
		// Non-fatal: IKS clusters don't have this API but that's fine.
		fmt.Printf("[declarative] namespace label patch skipped: %v\n", err)
	}
}

func writeTestResult(label string, passed bool) {
	fp, err := os.OpenFile(testResultFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer fp.Close()
	symbol := "✅"
	if !passed {
		symbol = "❌"
	}
	fp.WriteString(fmt.Sprintf("%s %s\n", symbol, label))
}

// ---------------------------------------------------------------------------
// buildVolume converts the YAML PVCSpec into the testsuites.VolumeDetails that
// all existing test runners expect.
// ---------------------------------------------------------------------------

func buildVolume(in testcases.Input) testsuites.VolumeDetails {
	// StorageClass name — prefer explicit storageClass; fall back to customSC.
	scName := in.StorageClass
	if scName == "" && in.CustomSC != nil {
		scName = in.CustomSC.Name
	}

	// Access mode
	am := v1.ReadWriteMany
	if in.PVC.AccessMode != "" {
		switch strings.ToLower(in.PVC.AccessMode) {
		case "readwriteonce":
			am = v1.ReadWriteOnce
		case "readonlymany":
			am = v1.ReadOnlyMany
		}
	}

	reclaimPolicy := v1.PersistentVolumeReclaimDelete
	if strings.ToLower(in.PVC.ReclaimPolicy) == "retain" {
		reclaimPolicy = v1.PersistentVolumeReclaimRetain
	}

	mountPath := in.PVC.MountPath
	if mountPath == "" {
		mountPath = "/mnt/test-1"
	}

	return testsuites.VolumeDetails{
		PVCName:       in.PVC.PVCNamePrefix,
		VolumeType:    scName,
		ClaimSize:     in.PVC.ClaimSize,
		ReclaimPolicy: &reclaimPolicy,
		AccessMode:    &am,
		MountOptions:  in.PVC.MountOptions,
		VolumeMount: testsuites.VolumeMountDetails{
			NameGenerate:      "test-volume-",
			MountPathGenerate: strings.TrimSuffix(mountPath, "-1"),
		},
	}
}

// ---------------------------------------------------------------------------
// Generic workload test
// ---------------------------------------------------------------------------

func runWorkloadTest(in testcases.Input, cs clientset.Interface, ns *v1.Namespace, testName string, expectSuccess bool) {
	vol := buildVolume(in)

	var cmd, expectedOut1, expectedOut2 string
	var readCmd []string
	var nodeSelector map[string]string
	var replicas int32 = 1

	if in.Workload != nil {
		cmd = in.Workload.Cmd
		readCmd = in.Workload.ReadCmd
		expectedOut1 = in.Workload.ExpectedOutput01
		expectedOut2 = in.Workload.ExpectedOutput02
		nodeSelector = in.Workload.NodeSelector
		if in.Workload.Replicas > 0 {
			replicas = in.Workload.Replicas
		}
	}

	podDetails := testsuites.PodDetails{
		Cmd:          cmd,
		CmdExits:     false,
		Volumes:      []testsuites.VolumeDetails{vol},
		NodeSelector: nodeSelector,
	}

	var podCheck *testsuites.PodExecCheck
	if len(readCmd) > 0 {
		podCheck = &testsuites.PodExecCheck{
			Cmd:              readCmd,
			ExpectedString01: expectedOut1,
			ExpectedString02: expectedOut2,
		}
	}

	DeferCleanup(func() {
		writeTestResult(testName, !CurrentSpecReport().Failed())
	})

	workloadType := "deployment"
	if in.Workload != nil {
		workloadType = strings.ToLower(in.Workload.Type)
	}

	switch workloadType {
	case "pod":
		test := testsuites.DynamicallyProvisionePodWithVolTest{
			Pods:     []testsuites.PodDetails{podDetails},
			PodCheck: podCheck,
		}
		test.Run(cs, ns)

	case "daemonset":
		headless := testsuites.NewHeadlessService(cs, "ics-e2e-service-", ns.Name, "test")
		svc := headless.Create()
		defer headless.Cleanup()
		labels := svc.Labels
		test := testsuites.DaemonsetWithVolWRTest{
			Pod:         podDetails,
			PodCheck:    podCheck,
			Labels:      labels,
			ServiceName: svc.Name,
		}
		test.Run(cs, ns, false)

	case "statefulset":
		headless := testsuites.NewHeadlessService(cs, "ics-e2e-sts-service-", ns.Name, "test")
		svc := headless.Create()
		defer headless.Cleanup()
		labels := map[string]string{"app": "ics-e2e-statefulset"}
		test := testsuites.StatefulsetWithVolWRTest{
			Pod:          podDetails,
			PodCheck:     podCheck,
			Labels:       labels,
			ServiceName:  svc.Name,
			ReplicaCount: replicas,
		}
		test.Run(cs, ns, false)

	default: // "deployment" (the common case)
		test := testsuites.DynamicallyProvisioneDeployWithVolWRTest{
			Pod:          podDetails,
			PodCheck:     podCheck,
			ReplicaCount: replicas,
		}
		if expectSuccess {
			test.Run(cs, ns)
		} else {
			test.RunShouldFail(cs, ns)
		}
	}
}

// ---------------------------------------------------------------------------
// Resize test
// ---------------------------------------------------------------------------

func runResizeTest(in testcases.Input, cs clientset.Interface, ns *v1.Namespace) {
	vol := buildVolume(in)

	var cmd, out1, out2 string
	var readCmd []string
	var nodeSelector map[string]string
	if in.Workload != nil {
		cmd = in.Workload.Cmd
		readCmd = in.Workload.ReadCmd
		out1 = in.Workload.ExpectedOutput01
		out2 = in.Workload.ExpectedOutput02
		nodeSelector = in.Workload.NodeSelector
	}

	pods := []testsuites.PodDetails{
		{
			Cmd:          cmd,
			CmdExits:     false,
			Volumes:      []testsuites.VolumeDetails{vol},
			NodeSelector: nodeSelector,
		},
	}

	DeferCleanup(func() {
		writeTestResult("RESIZE: "+in.PVC.ClaimSize+" → "+fmt.Sprintf("%dGi", in.Resize.ExpandToGi),
			!CurrentSpecReport().Failed())
	})

	test := testsuites.DynamicallyProvisionedResizeVolumeTest{
		Pods: pods,
		PodCheck: &testsuites.PodExecCheck{
			Cmd:              readCmd,
			ExpectedString01: out1,
			ExpectedString02: out2,
		},
		ExpandVolSizeG: in.Resize.ExpandToGi,
		ExpandedSize:   in.Resize.ExpectedGi,
	}
	test.Run(cs, ns)
}

// ---------------------------------------------------------------------------
// Snapshot test
// ---------------------------------------------------------------------------

func runSnapshotTest(tc testcases.TestCase, cs clientset.Interface, ns *v1.Namespace) {
	in := tc.Input
	vol := buildVolume(in)

	scName := in.StorageClass
	if scName == "" && in.CustomSC != nil {
		scName = in.CustomSC.Name
	}

	mountPath := in.PVC.MountPath
	if mountPath == "" {
		mountPath = "/mnt/test-1"
	}

	reclaimPolicy := v1.PersistentVolumeReclaimDelete
	restoreVolumeBase := testsuites.VolumeDetails{
		PVCName:       in.PVC.PVCNamePrefix,
		VolumeType:    scName,
		ReclaimPolicy: &reclaimPolicy,
		VolumeMount: testsuites.VolumeMountDetails{
			NameGenerate:      "test-volume-",
			MountPathGenerate: strings.TrimSuffix(mountPath, "-1"),
		},
	}

	var restoreClaimSizes [3]string
	copy(restoreClaimSizes[:], in.Snapshot.RestoreClaimSizes)

	restoredCmd := in.Snapshot.RestoredCmd
	if restoredCmd == "" {
		restoredCmd = "while true; do sleep 2; done"
	}

	var cmd, out1, out2 string
	var readCmd []string
	if in.Workload != nil {
		cmd = in.Workload.Cmd
		readCmd = in.Workload.ReadCmd
		out1 = in.Workload.ExpectedOutput01
		out2 = in.Workload.ExpectedOutput02
	}

	snapshotRCS, err := restClient(testsuites.SnapshotAPIGroup, testsuites.APIVersionv1)
	Expect(err).NotTo(HaveOccurred(), "could not get snapshot REST client")

	DeferCleanup(func() {
		writeTestResult("SNAPSHOT: "+tc.TestCase, !CurrentSpecReport().Failed())
	})

	test := testsuites.DynamicallyProvisionedVolumeSnapshotTest{
		Pod: testsuites.PodDetails{
			Cmd:      cmd,
			CmdExits: false,
			Volumes:  []testsuites.VolumeDetails{vol},
		},
		RestoredPod: testsuites.PodDetails{
			Cmd:     restoredCmd,
			Volumes: []testsuites.VolumeDetails{restoreVolumeBase},
		},
		RestoreClaimSizes: restoreClaimSizes,
		PodCheck: &testsuites.PodExecCheck{
			Cmd:              readCmd,
			ExpectedString01: out1,
			ExpectedString02: out2,
		},
	}

	By("VPC-FILE-CSI-TEST: SNAPSHOT | RESTORE SAME SIZE + LESS + MORE")
	test.RunAllRestoreVariants(cs, snapshotRCS, ns)
}

// ---------------------------------------------------------------------------
// EIT installation check
// ---------------------------------------------------------------------------

// runEITInstallationCheck enables EIT on the declared worker pool, waits for
// package installation, and then verifies the status ConfigMap.
func runEITInstallationCheck(in testcases.Input, cs clientset.Interface) {
	wp := in.EIT.WorkerPool

	By(fmt.Sprintf("Enabling EIT on worker pool: %s", wp))
	patchEITConfigMap(cs, true, wp)
	waitForEITPackages()
	rebootWorkersForRHCOS(cs)

	DeferCleanup(func() {
		writeTestResult("EIT: installation check (pool="+wp+")", !CurrentSpecReport().Failed())
		// Disable EIT after this Describe block completes.
		patchEITConfigMap(cs, false, "")
		waitForEITPackages()
	})

	By("Verifying all default worker-pool nodes appear in EIT_ENABLED_WORKER_NODES")
	nodeList, err := cs.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("ibm-cloud.kubernetes.io/worker-pool-name=%s", wp),
	})
	Expect(err).NotTo(HaveOccurred(), "failed to list nodes in worker pool %s", wp)
	Expect(nodeList.Items).NotTo(BeEmpty(), "no nodes found in worker pool %s", wp)

	poolNodeNames := make([]string, 0, len(nodeList.Items))
	for _, n := range nodeList.Items {
		poolNodeNames = append(poolNodeNames, n.Name)
	}

	cmStatus, err := cs.CoreV1().ConfigMaps("kube-system").Get(
		context.TODO(), "file-csi-driver-status", metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred(), "failed to get file-csi-driver-status ConfigMap")

	raw, exists := cmStatus.Data["EIT_ENABLED_WORKER_NODES"]
	Expect(exists).To(BeTrue(), "EIT_ENABLED_WORKER_NODES key missing from file-csi-driver-status")

	var eitNodes map[string][]string
	Expect(yaml.Unmarshal([]byte(raw), &eitNodes)).To(Succeed())

	eitSet := make(map[string]struct{})
	for _, name := range eitNodes[wp] {
		eitSet[strings.TrimSpace(name)] = struct{}{}
	}

	missing := []string{}
	for _, name := range poolNodeNames {
		if _, found := eitSet[name]; !found {
			missing = append(missing, name)
		}
	}
	Expect(missing).To(BeEmpty(),
		"nodes missing from EIT_ENABLED_WORKER_NODES: %v", missing)
}

// ---------------------------------------------------------------------------
// EIT negative test (mount must fail)
// ---------------------------------------------------------------------------

func runEITMountFailTest(in testcases.Input, cs clientset.Interface, ns *v1.Namespace) {
	// Ensure EIT is disabled before the test.
	patchEITConfigMap(cs, false, "")
	waitForEITPackages()

	DeferCleanup(func() {
		writeTestResult("EIT: mount-fail on disabled pool", !CurrentSpecReport().Failed())
	})

	vol := buildVolume(in)
	var cmd string
	var replicas int32 = 3
	if in.Workload != nil {
		cmd = in.Workload.Cmd
		if in.Workload.Replicas > 0 {
			replicas = in.Workload.Replicas
		}
	}

	pod := testsuites.PodDetails{
		Cmd:      cmd,
		CmdExits: false,
		Volumes:  []testsuites.VolumeDetails{vol},
	}
	test := testsuites.DynamicallyProvisioneDeployWithVolWRTest{
		Pod:          pod,
		ReplicaCount: replicas,
	}
	test.RunShouldFail(cs, ns)
}

// ---------------------------------------------------------------------------
// Capacity roundoff test
// ---------------------------------------------------------------------------

func runCapacityRoundoffTest(in testcases.Input, cs clientset.Interface, ns *v1.Namespace) {
	cr := in.CapacityRoundoff
	scName := in.StorageClass
	pvcName := in.PVC.PVCNamePrefix

	DeferCleanup(func() {
		label := fmt.Sprintf("ROUNDOFF: %s %s → %s", scName, cr.RequestedCapacity, cr.ExpectedCapacity)
		writeTestResult(label, !CurrentSpecReport().Failed())
	})

	pvc := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: ns.Name,
			Labels:    map[string]string{"app": "ics-e2e-tester"},
		},
		Spec: v1.PersistentVolumeClaimSpec{
			StorageClassName: &scName,
			AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteMany},
			Resources: v1.VolumeResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceStorage: resource.MustParse(cr.RequestedCapacity),
				},
			},
		},
	}

	By(fmt.Sprintf("Creating PVC %s with storageclass %s and requested capacity %s",
		pvcName, scName, cr.RequestedCapacity))
	createdPVC, err := cs.CoreV1().PersistentVolumeClaims(ns.Name).Create(
		context.TODO(), pvc, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())

	DeferCleanup(func() {
		_ = cs.CoreV1().PersistentVolumeClaims(ns.Name).Delete(
			context.TODO(), pvcName, metav1.DeleteOptions{})
	})

	if cr.ExpectProvisionFail {
		// Negative case: PVC must NOT reach Bound.
		By("Verifying PVC does NOT reach Bound state")
		var observedPVC *v1.PersistentVolumeClaim
		_ = wait.PollImmediate(5*time.Second, 1*time.Minute, func() (bool, error) {
			observedPVC, _ = cs.CoreV1().PersistentVolumeClaims(ns.Name).Get(
				context.TODO(), createdPVC.Name, metav1.GetOptions{})
			return observedPVC != nil && observedPVC.Status.Phase == v1.ClaimBound, nil
		})
		if observedPVC != nil {
			Expect(observedPVC.Status.Phase).NotTo(Equal(v1.ClaimBound),
				"PVC should NOT reach Bound for out-of-range capacity")
		}

		By("Verifying ProvisioningFailed event")
		var matched bool
		_ = wait.PollImmediate(5*time.Second, 2*time.Minute, func() (bool, error) {
			events, _ := cs.CoreV1().Events(ns.Name).List(context.TODO(), metav1.ListOptions{})
			for _, ev := range events.Items {
				if ev.InvolvedObject.Name == pvcName &&
					ev.InvolvedObject.Kind == "PersistentVolumeClaim" {
					if ev.Reason == "ProvisioningFailed" ||
						strings.Contains(ev.Message, "invalid PVC size") ||
						strings.Contains(ev.Message, "infeasible") ||
						strings.Contains(ev.Message, "shares_profile_capacity_iops_invalid") {
						matched = true
						return true, nil
					}
				}
			}
			return false, nil
		})
		Expect(matched).To(BeTrue(), "expected ProvisioningFailed event not found")
		return
	}

	// Positive case: PVC must reach Bound with the rounded-up capacity.
	By("Waiting for PVC to reach Bound state")
	var boundPVC *v1.PersistentVolumeClaim
	Expect(wait.PollImmediate(5*time.Second, 10*time.Minute, func() (bool, error) {
		boundPVC, err = cs.CoreV1().PersistentVolumeClaims(ns.Name).Get(
			context.TODO(), createdPVC.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return boundPVC.Status.Phase == v1.ClaimBound, nil
	})).To(Succeed(), "PVC failed to reach Bound state")

	expectedQ := resource.MustParse(cr.ExpectedCapacity)

	By(fmt.Sprintf("Verifying PVC capacity is %s", cr.ExpectedCapacity))
	actualCap := boundPVC.Status.Capacity[v1.ResourceStorage]
	Expect(actualCap.Cmp(expectedQ)).To(Equal(0),
		"PVC capacity: expected %s got %s", cr.ExpectedCapacity, actualCap.String())

	By(fmt.Sprintf("Verifying PV capacity is %s", cr.ExpectedCapacity))
	pv, err := cs.CoreV1().PersistentVolumes().Get(
		context.TODO(), boundPVC.Spec.VolumeName, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())
	pvCap := pv.Spec.Capacity[v1.ResourceStorage]
	Expect(pvCap.Cmp(expectedQ)).To(Equal(0),
		"PV capacity: expected %s got %s", cr.ExpectedCapacity, pvCap.String())
}

// ---------------------------------------------------------------------------
// EIT ConfigMap helpers (shared with EIT tests)
// ---------------------------------------------------------------------------

func patchEITConfigMap(cs clientset.Interface, enable bool, workerPool string) {
	data := map[string]string{"ENABLE_EIT": fmt.Sprintf("%v", enable)}
	if enable && workerPool != "" {
		data["EIT_ENABLED_WORKER_POOLS"] = workerPool
	}
	patch := map[string]interface{}{"data": data}
	patchBytes, _ := json.Marshal(patch)
	cm, err := cs.CoreV1().ConfigMaps("kube-system").Patch(
		context.TODO(),
		"addon-vpc-file-csi-driver-configmap",
		types.MergePatchType,
		patchBytes,
		metav1.PatchOptions{},
	)
	if err != nil {
		panic(fmt.Sprintf("failed to patch EIT ConfigMap: %v", err))
	}
	fmt.Printf("[declarative] patched addon-vpc-file-csi-driver-configmap: %v\n", cm.Data)
}

func waitForEITPackages() {
	fmt.Printf("[declarative] sleeping %s for EIT package (un)installation\n", waitForPackageInstallation)
	time.Sleep(waitForPackageInstallation)
}
