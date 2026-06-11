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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
	restclientset "k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
)

var _ = Describe("[ics-e2e] [snapshot] Dynamic Provisioning of Snapshot for dp2 and rfs profile ", func() {
	f := framework.NewDefaultFramework("ics-e2e-vpcfile-snap")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var (
		cs          clientset.Interface
		snapshotrcs restclientset.Interface
		ns          *v1.Namespace
	)

	BeforeEach(func() {
		cs = f.ClientSet
		ns = f.Namespace

		var err error
		snapshotrcs, err = restClient(testsuites.SnapshotAPIGroup, testsuites.APIVersionv1)
		if err != nil {
			Fail(fmt.Sprintf("could not get rest clientset: %v", err))
		}
	})

	It("should run snapshot lifecycle tests for RFS VPC File", func() {
		payload := `{"metadata": {"labels": {"security.openshift.io/scc.podSecurityLabelSync": "false","pod-security.kubernetes.io/enforce": "privileged"}}}`
		_, labelErr := cs.CoreV1().Namespaces().Patch(context.TODO(), ns.Name,
			types.StrategicMergePatchType, []byte(payload), metav1.PatchOptions{})
		if labelErr != nil {
			panic(labelErr)
		}

		fpointer, err = os.OpenFile(testResultFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}
		defer fpointer.Close()

		reclaimPolicy := v1.PersistentVolumeReclaimDelete

		pod := testsuites.PodDetails{
			Cmd:      "echo 'hello world' >> /mnt/test-1/data && grep 'hello world' /mnt/test-1/data && sync",
			CmdExits: false,
			Volumes: []testsuites.VolumeDetails{
				{
					PVCName:       "ics-vpcfile-rfs-snap-",
					VolumeType:    "ibmc-vpc-file-regional",
					FSType:        "nfs",
					ClaimSize:     "20Gi",
					ReclaimPolicy: &reclaimPolicy,
					VolumeMount: testsuites.VolumeMountDetails{
						NameGenerate:      "test-volume-",
						MountPathGenerate: "/mnt/test-",
					},
				},
			},
		}

		restoredPodSame := testsuites.PodDetails{
			Cmd: "grep 'hello world' /mnt/test-1/data && while true; do sleep 2; done",
			Volumes: []testsuites.VolumeDetails{
				{
					PVCName:       "ics-vpcfile-rfs-snap-",
					VolumeType:    "ibmc-vpc-file-regional",
					FSType:        "nfs",
					ClaimSize:     "20Gi",
					ReclaimPolicy: &reclaimPolicy,
					VolumeMount: testsuites.VolumeMountDetails{
						NameGenerate:      "test-volume-",
						MountPathGenerate: "/mnt/test-",
					},
				},
			},
		}

		test1 := testsuites.DynamicallyProvisionedVolumeSnapshotTest{
			Pod:         pod,
			RestoredPod: restoredPodSame,
			PodCheck: &testsuites.PodExecCheck{
				Cmd:              []string{"cat", "/mnt/test-1/data"},
				ExpectedString01: "hello world\n",
				ExpectedString02: "hello world\nhello world\n",
			},
		}

		By("VPC-FILE-CSI-TEST: RFS PROFILE | SNAPSHOT | RESTORE SAME SIZE")
		test1.Run(cs, snapshotrcs, ns)

		if _, err = fpointer.WriteString("✅ RFS PROFILE | VOLUME CREATION | SNAPSHOT CREATION | RESTORE VOLUME SAME CLAIM SIZE | DELETE SNAPSHOT: PASS\n"); err != nil {
			panic(err)
		}

		restoredPodLess := testsuites.PodDetails{
			Cmd: "grep 'hello world' /mnt/test-1/data && while true; do sleep 2; done",
			Volumes: []testsuites.VolumeDetails{
				{
					PVCName:       "ics-vpcfile-rfs-snap-",
					VolumeType:    "ibmc-vpc-file-regional",
					FSType:        "nfs",
					ClaimSize:     "10Gi",
					ReclaimPolicy: &reclaimPolicy,
					VolumeMount: testsuites.VolumeMountDetails{
						NameGenerate:      "test-volume-",
						MountPathGenerate: "/mnt/test-",
					},
				},
			},
		}

		test2 := testsuites.DynamicallyProvisionedVolumeSnapshotTest{
			Pod:         pod,
			RestoredPod: restoredPodLess,
			PodCheck:    test1.PodCheck,
		}

		By("VPC-FILE-CSI-TEST: RFS PROFILE | SNAPSHOT | RESTORE CLAIM SIZE LESS")
		test2.VolumeSizeLess(cs, snapshotrcs, ns)

		if _, err = fpointer.WriteString("✅ RFS PROFILE | VOLUME CREATION | SNAPSHOT CREATION | RESTORE VOLUME CLAIM SIZE LESS | DELETE SNAPSHOT : PASS\n"); err != nil {
			panic(err)
		}

		restoredPodMore := testsuites.PodDetails{
			Cmd: "grep 'hello world' /mnt/test-1/data && while true; do sleep 2; done",
			Volumes: []testsuites.VolumeDetails{
				{
					PVCName:       "ics-vpcfile-rfs-snap-",
					VolumeType:    "ibmc-vpc-file-regional",
					FSType:        "nfs",
					ClaimSize:     "30Gi",
					ReclaimPolicy: &reclaimPolicy,
					VolumeMount: testsuites.VolumeMountDetails{
						NameGenerate:      "test-volume-",
						MountPathGenerate: "/mnt/test-",
					},
				},
			},
		}

		test3 := testsuites.DynamicallyProvisionedVolumeSnapshotTest{
			Pod:         pod,
			RestoredPod: restoredPodMore,
			PodCheck:    test1.PodCheck,
		}

		By("VPC-FILE-CSI-TEST: RFS PROFILE | SNAPSHOT | RESTORE CLAIM SIZE MORE")
		test3.Run(cs, snapshotrcs, ns)

		if _, err = fpointer.WriteString("✅ RFS PROFILE | VOLUME CREATION | SNAPSHOT CREATION | RESTORE VOLUME CLAIM SIZE MORE | DELETE SNAPSHOT: PASS\n"); err != nil {
			panic(err)
		}
	})

	It("should run snapshot lifecycle tests for DP2 VPC File", func() {
		payload := `{"metadata": {"labels": {"security.openshift.io/scc.podSecurityLabelSync": "false","pod-security.kubernetes.io/enforce": "privileged"}}}`
		_, labelErr := cs.CoreV1().Namespaces().Patch(context.TODO(), ns.Name,
			types.StrategicMergePatchType, []byte(payload), metav1.PatchOptions{})
		if labelErr != nil {
			panic(labelErr)
		}

		fpointer, err = os.OpenFile(testResultFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}
		defer fpointer.Close()

		reclaimPolicy := v1.PersistentVolumeReclaimDelete

		pod := testsuites.PodDetails{
			Cmd:      "echo 'hello world' >> /mnt/test-1/data && grep 'hello world' /mnt/test-1/data && sync",
			CmdExits: false,
			Volumes: []testsuites.VolumeDetails{
				{
					PVCName:       "ics-vpcfile-dp2-snap-",
					VolumeType:    "ibmc-vpc-file-min-iops",
					FSType:        "nfs",
					ClaimSize:     "20Gi",
					ReclaimPolicy: &reclaimPolicy,
					VolumeMount: testsuites.VolumeMountDetails{
						NameGenerate:      "test-volume-",
						MountPathGenerate: "/mnt/test-",
					},
				},
			},
		}

		restoredPodSame := testsuites.PodDetails{
			Cmd: "grep 'hello world' /mnt/test-1/data && while true; do sleep 2; done",
			Volumes: []testsuites.VolumeDetails{
				{
					PVCName:       "ics-vpcfile-dp2-snap-",
					VolumeType:    "ibmc-vpc-file-min-iops",
					FSType:        "nfs",
					ClaimSize:     "20Gi",
					ReclaimPolicy: &reclaimPolicy,
					VolumeMount: testsuites.VolumeMountDetails{
						NameGenerate:      "test-volume-",
						MountPathGenerate: "/mnt/test-",
					},
				},
			},
		}

		test1 := testsuites.DynamicallyProvisionedVolumeSnapshotTest{
			Pod:         pod,
			RestoredPod: restoredPodSame,
			PodCheck: &testsuites.PodExecCheck{
				Cmd:              []string{"cat", "/mnt/test-1/data"},
				ExpectedString01: "hello world\n",
				ExpectedString02: "hello world\nhello world\n",
			},
		}

		By("VPC-FILE-CSI-TEST: DP2 PROFILE | SNAPSHOT | RESTORE SAME SIZE")
		test1.Run(cs, snapshotrcs, ns)

		if _, err = fpointer.WriteString("✅ DP2 PROFILE | VOLUME CREATION | SNAPSHOT CREATION | RESTORE VOLUME SAME CLAIM SIZE | DELETE SNAPSHOT: PASS\n"); err != nil {
			panic(err)
		}

		restoredPodLess := testsuites.PodDetails{
			Cmd: "grep 'hello world' /mnt/test-1/data && while true; do sleep 2; done",
			Volumes: []testsuites.VolumeDetails{
				{
					PVCName:       "ics-vpcfile-dp2-snap-",
					VolumeType:    "ibmc-vpc-file-min-iops",
					FSType:        "nfs",
					ClaimSize:     "10Gi",
					ReclaimPolicy: &reclaimPolicy,
					VolumeMount: testsuites.VolumeMountDetails{
						NameGenerate:      "test-volume-",
						MountPathGenerate: "/mnt/test-",
					},
				},
			},
		}

		test2 := testsuites.DynamicallyProvisionedVolumeSnapshotTest{
			Pod:         pod,
			RestoredPod: restoredPodLess,
			PodCheck:    test1.PodCheck,
		}

		By("VPC-FILE-CSI-TEST: DP2 PROFILE | SNAPSHOT | RESTORE CLAIM SIZE LESS")
		test2.VolumeSizeLess(cs, snapshotrcs, ns)

		if _, err = fpointer.WriteString("✅ DP2 PROFILE | VOLUME CREATION | SNAPSHOT CREATION | RESTORE VOLUME CLAIM SIZE LESS | DELETE SNAPSHOT : PASS\n"); err != nil {
			panic(err)
		}

		restoredPodMore := testsuites.PodDetails{
			Cmd: "grep 'hello world' /mnt/test-1/data && while true; do sleep 2; done",
			Volumes: []testsuites.VolumeDetails{
				{
					PVCName:       "ics-vpcfile-dp2-snap-",
					VolumeType:    "ibmc-vpc-file-min-iops",
					FSType:        "nfs",
					ClaimSize:     "30Gi",
					ReclaimPolicy: &reclaimPolicy,
					VolumeMount: testsuites.VolumeMountDetails{
						NameGenerate:      "test-volume-",
						MountPathGenerate: "/mnt/test-",
					},
				},
			},
		}

		test3 := testsuites.DynamicallyProvisionedVolumeSnapshotTest{
			Pod:         pod,
			RestoredPod: restoredPodMore,
			PodCheck:    test1.PodCheck,
		}

		By("VPC-FILE-CSI-TEST: DP2 PROFILE | SNAPSHOT | RESTORE CLAIM SIZE MORE")
		test3.Run(cs, snapshotrcs, ns)

		if _, err = fpointer.WriteString("✅ DP2 PROFILE | VOLUME CREATION | SNAPSHOT CREATION | RESTORE VOLUME CLAIM SIZE MORE | DELETE SNAPSHOT: PASS\n"); err != nil {
			panic(err)
		}
	})
})
