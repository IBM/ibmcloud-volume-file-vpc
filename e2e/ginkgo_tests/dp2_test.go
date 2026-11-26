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
package e2e

import (
	"context"
	"fmt"
	"os"

	"github.com/IBM/ibmcloud-volume-file-vpc/e2e/testsuites"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
)

var _ = Describe("[ics-e2e] [sc] [with-deploy] [retain] Dynamic Provisioning using retain SC with Deployment", func() {
	f := framework.NewDefaultFramework("ics-e2e-deploy")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged
	var (
		cs        clientset.Interface
		ns        *v1.Namespace
		secretKey string
	)

	secretKey = os.Getenv("E2E_SECRET_ENCRYPTION_KEY")
	if secretKey == "" {
		secretKey = defaultSecret
	}

	BeforeEach(func() {
		cs = f.ClientSet
		ns = f.Namespace
	})

	It("with retain sc: should create a pvc &pv, deployment resources, write and read to volume, delete the pod, write and read to volume again", func() {
		payload := `{"metadata": {"labels": {"security.openshift.io/scc.podSecurityLabelSync": "false","pod-security.kubernetes.io/enforce": "privileged"}}}`
		_, labelerr := cs.CoreV1().Namespaces().Patch(context.TODO(), ns.Name, types.StrategicMergePatchType, []byte(payload), metav1.PatchOptions{})
		if labelerr != nil {
			panic(labelerr)
		}
		reclaimPolicy := v1.PersistentVolumeReclaimRetain
		fpointer, err = os.OpenFile(testResultFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}

		DeferCleanup(func() {
			if fpointer != nil {
				if CurrentSpecReport().Failed() {
					fpointer.WriteString(fmt.Sprintf("❌ DP2: PVC CREATE/DELETE WITH %s STORAGE CLASS\n", sc_retain))
				} else {
					fpointer.WriteString(fmt.Sprintf("✅ DP2: PVC CREATE/DELETE WITH %s STORAGE CLASS\n", sc_retain))
				}
				fpointer.Close()
			}
		})

		var replicaCount = int32(1)
		pod := testsuites.PodDetails{
			Cmd:      "echo 'hello world' >> /mnt/test-1/data && while true; do sleep 2; done",
			CmdExits: false,
			Volumes: []testsuites.VolumeDetails{
				{
					PVCName:       "ics-vol-dp2-",
					VolumeType:    sc_retain,
					ClaimSize:     "15Gi",
					ReclaimPolicy: &reclaimPolicy,
					MountOptions:  []string{"rw"},
					VolumeMount: testsuites.VolumeMountDetails{
						NameGenerate:      "test-volume-",
						MountPathGenerate: "/mnt/test-",
					},
				},
			},
		}
		test := testsuites.DynamicallyProvisioneDeployWithVolWRTest{
			Pod: pod,
			PodCheck: &testsuites.PodExecCheck{
				Cmd:              []string{"cat", "/mnt/test-1/data"},
				ExpectedString01: "hello world\n",
				ExpectedString02: "hello world\nhello world\n",
			},
			ReplicaCount: replicaCount,
		}
		test.Run(cs, ns)
	})
})

var _ = Describe("[ics-e2e] [sc] [with-deploy] Dynamic Provisioning for dp2 SC with Deployment", func() {
	f := framework.NewDefaultFramework("ics-e2e-deploy")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged
	var (
		cs        clientset.Interface
		ns        *v1.Namespace
		secretKey string
	)

	secretKey = os.Getenv("E2E_SECRET_ENCRYPTION_KEY")
	if secretKey == "" {
		secretKey = defaultSecret
	}

	BeforeEach(func() {
		cs = f.ClientSet
		ns = f.Namespace
	})

	It("with dp2 sc: should create a pvc &pv, deployment resources, write and read to volume, delete the pod, write and read to volume again", func() {
		payload := `{"metadata": {"labels": {"security.openshift.io/scc.podSecurityLabelSync": "false","pod-security.kubernetes.io/enforce": "privileged"}}}`
		_, labelerr := cs.CoreV1().Namespaces().Patch(context.TODO(), ns.Name, types.StrategicMergePatchType, []byte(payload), metav1.PatchOptions{})
		if labelerr != nil {
			panic(labelerr)
		}

		reclaimPolicy := v1.PersistentVolumeReclaimDelete
		fpointer, err = os.OpenFile(testResultFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}

		DeferCleanup(func() {
			if fpointer != nil {
				if CurrentSpecReport().Failed() {
					fpointer.WriteString(fmt.Sprintf("❌ DP2: PVC CREATE/DELETE WITH %s STORAGE CLASS\n", sc))
				} else {
					fpointer.WriteString(fmt.Sprintf("✅ DP2: PVC CREATE/DELETE WITH %s STORAGE CLASS\n", sc))
				}
				fpointer.Close()
			}
		})

		var replicaCount = int32(1)
		pod := testsuites.PodDetails{
			Cmd:      "echo 'hello world' >> /mnt/test-1/data && while true; do sleep 2; done",
			CmdExits: false,
			Volumes: []testsuites.VolumeDetails{
				{
					PVCName:       "ics-vol-dp2-",
					VolumeType:    sc,
					ClaimSize:     "15Gi",
					ReclaimPolicy: &reclaimPolicy,
					MountOptions:  []string{"rw"},
					VolumeMount: testsuites.VolumeMountDetails{
						NameGenerate:      "test-volume-",
						MountPathGenerate: "/mnt/test-",
					},
				},
			},
		}
		test := testsuites.DynamicallyProvisioneDeployWithVolWRTest{
			Pod: pod,
			PodCheck: &testsuites.PodExecCheck{
				Cmd:              []string{"cat", "/mnt/test-1/data"},
				ExpectedString01: "hello world\n",
				ExpectedString02: "hello world\nhello world\n",
			},
			ReplicaCount: replicaCount,
		}
		test.Run(cs, ns)
	})
})

var _ = Describe("[ics-e2e] [sc] [same-node] [with-deploy] Dynamic Provisioning for dp2 SC with Deployment running multiple pods on same node", func() {
	f := framework.NewDefaultFramework("ics-e2e-deploy")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged
	var (
		cs        clientset.Interface
		ns        *v1.Namespace
		secretKey string
	)

	secretKey = os.Getenv("E2E_SECRET_ENCRYPTION_KEY")
	if secretKey == "" {
		secretKey = defaultSecret
	}

	BeforeEach(func() {
		cs = f.ClientSet
		ns = f.Namespace
	})

	It("with dp2 sc: should create a pvc &pv, deployment resources, write and read to volume, delete the pod, write and read to volume again", func() {
		payload := `{"metadata": {"labels": {"security.openshift.io/scc.podSecurityLabelSync": "false","pod-security.kubernetes.io/enforce": "privileged"}}}`
		_, labelerr := cs.CoreV1().Namespaces().Patch(context.TODO(), ns.Name, types.StrategicMergePatchType, []byte(payload), metav1.PatchOptions{})
		if labelerr != nil {
			panic(labelerr)
		}
		reclaimPolicy := v1.PersistentVolumeReclaimDelete
		fpointer, err = os.OpenFile(testResultFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}

		DeferCleanup(func() {
			if fpointer != nil {
				if CurrentSpecReport().Failed() {
					fpointer.WriteString("❌ DP2: MULTI-POD READ/WRITE ON SAME NODE BY USING DEPLOYMENT\n")
				} else {
					fpointer.WriteString("✅ DP2: MULTI-POD READ/WRITE ON SAME NODE BY USING DEPLOYMENT\n")
				}
				fpointer.Close()
			}
		})

		var replicaCount = int32(4)
		pod := testsuites.PodDetails{
			Cmd:      "echo 'hello world' >> /mnt/test-1/data && while true; do sleep 2; done",
			CmdExits: false,
			Volumes: []testsuites.VolumeDetails{
				{
					PVCName:       "ics-vol-dp2-",
					VolumeType:    sc,
					ClaimSize:     "15Gi",
					ReclaimPolicy: &reclaimPolicy,
					MountOptions:  []string{"rw"},
					VolumeMount: testsuites.VolumeMountDetails{
						NameGenerate:      "test-volume-",
						MountPathGenerate: "/mnt/test-",
					},
				},
			},
		}
		test := testsuites.DynamicallyProvisioneDeployWithVolWRTest{
			Pod: pod,
			PodCheck: &testsuites.PodExecCheck{
				Cmd:              []string{"cat", "/mnt/test-1/data"},
				ExpectedString01: "hello world\n",
				ExpectedString02: "hello world\nhello world\n",
			},
			ReplicaCount: replicaCount,
		}
		test.Run(cs, ns)
	})
})

var _ = Describe("[ics-e2e] [sc] [rwo] [with-deploy] Dynamic Provisioning for dp2 SC in RWO Mode with Deployment", func() {
	f := framework.NewDefaultFramework("ics-e2e-deploy")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged
	var (
		cs           clientset.Interface
		ns           *v1.Namespace
		cleanupFuncs []func()
		volList      []testsuites.VolumeDetails
		cmdLongLife  string
		maxPVC       int
		maxPOD       int
		secretKey    string
	)

	secretKey = os.Getenv("E2E_SECRET_ENCRYPTION_KEY")
	if secretKey == "" {
		secretKey = defaultSecret
	}

	maxPOD = 4
	BeforeEach(func() {
		cs = f.ClientSet
		ns = f.Namespace
		cleanupFuncs = make([]func(), 0)

		cmdLongLife = "df -h; echo 'hello world' > /mnt/test-1/data && while true; do sleep 2; done"

		reclaimPolicy := v1.PersistentVolumeReclaimDelete
		accessMode := v1.ReadWriteOnce

		volList = []testsuites.VolumeDetails{
			{
				PVCName:       "ics-vol-dp2-",
				VolumeType:    sc,
				AccessMode:    &accessMode,
				ClaimSize:     "15Gi",
				ReclaimPolicy: &reclaimPolicy,
				MountOptions:  []string{"rw"},
				VolumeMount: testsuites.VolumeMountDetails{
					NameGenerate:      "test-volume-",
					MountPathGenerate: "/mnt/test-",
				},
			},
		}
	})

	It("with dp2 sc: should create a pvc &pv with RWO mode , deployment resources, write and read to volume, delete the pod, write and read to volume again", func() {
		payload := `{"metadata": {"labels": {"security.openshift.io/scc.podSecurityLabelSync": "false","pod-security.kubernetes.io/enforce": "privileged"}}}`
		_, labelerr := cs.CoreV1().Namespaces().Patch(context.TODO(), ns.Name, types.StrategicMergePatchType, []byte(payload), metav1.PatchOptions{})
		if labelerr != nil {
			panic(labelerr)
		}

		fpointer, err = os.OpenFile(testResultFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}

		DeferCleanup(func() {
			if fpointer != nil {
				if CurrentSpecReport().Failed() {
					fpointer.WriteString("❌ DP2: PVC WITH RWO MODE\n")
				} else {
					fpointer.WriteString("✅ DP2: PVC WITH RWO MODE\n")
				}
				fpointer.Close()
			}
		})

		var execCmd string
		var cmdExits bool
		var vols []testsuites.VolumeDetails
		var pods []testsuites.PodDetails

		maxPVC = 1
		vollistLen := len(volList)
		fmt.Println("vollistLen", vollistLen)
		vols = make([]testsuites.VolumeDetails, 0)
		xi := 0
		for i := 0; vollistLen > 0 && i < maxPVC; i++ {
			if xi >= vollistLen {
				xi = 0
			}
			vol := volList[xi]
			vols = append(vols, vol)
			xi = xi + 1
		}

		execCmd = cmdLongLife
		cmdExits = false
		for n := range vols {
			_, funcs := vols[n].SetupDynamicPersistentVolumeClaim(cs, ns, false)
			cleanupFuncs = append(cleanupFuncs, funcs...)
		}

		for i := range cleanupFuncs {
			defer cleanupFuncs[i]()
		}

		pods = make([]testsuites.PodDetails, 0)
		for i := 0; i < maxPOD; i++ {
			pod := testsuites.PodDetails{
				Cmd:      execCmd,
				CmdExits: cmdExits,
				Volumes:  vols,
			}
			pods = append(pods, pod)
		}
		test := testsuites.DynamicallyProvisioneMultiPodWithVolTest{
			Pods:     pods,
			PodCheck: nil,
		}

		test.RunAsync(cs, ns)
	})
})

var _ = Describe("[ics-e2e] [sc] [with-daemonset] Dynamic Provisioning using daemonsets", func() {
	f := framework.NewDefaultFramework("ics-e2e-daemonsets")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged
	var (
		cs clientset.Interface
		ns *v1.Namespace
	)
	BeforeEach(func() {
		cs = f.ClientSet
		ns = f.Namespace
	})
	It("With 5iops sc: should creat daemonset resources, write and read to volume", func() {
		payload := `{"metadata": {"labels": {"security.openshift.io/scc.podSecurityLabelSync": "false","pod-security.kubernetes.io/enforce": "privileged"}}}`
		_, labelerr := cs.CoreV1().Namespaces().Patch(context.TODO(), ns.Name, types.StrategicMergePatchType, []byte(payload), metav1.PatchOptions{})
		if labelerr != nil {
			panic(labelerr)
		}
		fpointer, err = os.OpenFile(testResultFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}

		DeferCleanup(func() {
			if fpointer != nil {
				if CurrentSpecReport().Failed() {
					fpointer.WriteString("❌ DP2: MULTI-ZONE/MULTI-NODE READ/WRITE BY USING DAEMONSET\n")
				} else {
					fpointer.WriteString("✅ DP2: MULTI-ZONE/MULTI-NODE READ/WRITE BY USING DAEMONSET\n")
				}
				fpointer.Close()
			}
		})

		headlessService := testsuites.NewHeadlessService(cs, "ics-e2e-service-", ns.Name, "test")
		service := headlessService.Create()
		defer headlessService.Cleanup()

		reclaimPolicy := v1.PersistentVolumeReclaimDelete
		pod := testsuites.PodDetails{
			Cmd: "echo 'hello world' > /mnt/test-1/data && while true; do sleep 2; done",
			Volumes: []testsuites.VolumeDetails{
				{
					PVCName:       "ics-vol-dp2-",
					VolumeType:    sc,
					ClaimSize:     "20Gi",
					ReclaimPolicy: &reclaimPolicy,
					MountOptions:  []string{"rw"},
					VolumeMount: testsuites.VolumeMountDetails{
						NameGenerate:      "test-volume-",
						MountPathGenerate: "/mnt/test-",
					},
				},
			},
		}

		test := testsuites.DaemonsetWithVolWRTest{
			Pod: pod,
			PodCheck: &testsuites.PodExecCheck{
				Cmd:              []string{"cat", "/mnt/test-1/data"},
				ExpectedString01: "hello world\n",
			},
			Labels:      service.Labels,
			ServiceName: service.Name,
		}
		test.Run(cs, ns, false)
	})
})

// =============================================================================
// VolumeAttributesClass (VAC) tests — dp2 profile
//
// The driver does not check the volume's profile. It forwards iops/throughput
// to VPC, and VPC rejects parameters the profile does not accept
// (shares_profile_bandwidth_not_allowed for dp2). The driver returns that
// backend error as gRPC InvalidArgument and csi-resizer records a
// VolumeModifyFailed event on the PVC; the specs assert on that code (see
// waitForVACRejected) and that the share is left unchanged.
//
// Scenario IDs: SC1–SC4 in rfs_test.go (rfs), SC5–SC8 here (dp2).
//
// SC5 — dp2 PVC + VAC(iops + throughput) → rejected, throughput is not accepted
// SC6 — dp2 PVC + VAC(throughput only)   → rejected, throughput is not accepted
// SC7 — dp2 PVC + VAC(iops 99999999)     → rejected, iops out of range
// SC8 — dp2 PVC + VAC(iops 500)          → applied, share iops is 500
// =============================================================================

var dp2VACCases = []vacCase{
	{id: "SC5", profile: "dp2", summary: "iops+throughput", params: map[string]string{"iops": "500", "throughput": "512"}, reject: true},
	{id: "SC6", profile: "dp2", summary: "throughput only", params: map[string]string{"throughput": "500"}, reject: true},
	{id: "SC7", profile: "dp2", summary: "iops out-of-range", params: map[string]string{"iops": "99999999"}, reject: true},
	{id: "SC8", profile: "dp2", summary: "iops only", params: map[string]string{"iops": "500"}, wantIops: 500},
}

var _ = Describe("[ics-e2e] [vac] [dp2] VolumeAttributesClass tests for dp2 profile", func() {
	f := framework.NewDefaultFramework("ics-e2e-vac-dp2")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var (
		cs clientset.Interface
		ns *v1.Namespace
	)

	// dp2 StorageClass name — read from env so it can be overridden per cluster.
	// Falls back to the SC env var used by the existing dp2 tests.
	dp2SC := os.Getenv("DP2_SC")
	if dp2SC == "" {
		dp2SC = sc
	}

	BeforeEach(func() {
		cs = f.ClientSet
		ns = f.Namespace
		Expect(dp2SC).NotTo(BeEmpty(), "set DP2_SC (or SC) to a dp2 StorageClass; an empty name would leave the PVC Pending for the full bind timeout")
		skipIfVACUnsupported(cs)
		prepareVACNamespace(cs, ns.Name)
	})

	for _, c := range dp2VACCases {
		It(c.title(), func() {
			runVACSpec(cs, ns.Name, dp2SC, c)
		})
	}
})
