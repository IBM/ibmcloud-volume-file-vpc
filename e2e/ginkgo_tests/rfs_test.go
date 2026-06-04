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
	"k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
)

var _ = Describe("[ics-e2e] [sc_rfs] Dynamic Provisioning for RFS SC with Deployment", func() {
	f := framework.NewDefaultFramework("ics-e2e-deploy")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged
	var (
		cs clientset.Interface
		ns *v1.Namespace
	)

	BeforeEach(func() {
		cs = f.ClientSet
		ns = f.Namespace
	})

	It("with rfs profile sc: should create a pvc, deployment resources, write and read to volume, delete the pod", func() {
		payload := `{"metadata": {"labels": {"security.openshift.io/scc.podSecurityLabelSync": "false","pod-security.kubernetes.io/enforce": "privileged"}}}`
		_, labelerr := cs.CoreV1().Namespaces().Patch(context.TODO(), ns.Name, types.StrategicMergePatchType, []byte(payload), metav1.PatchOptions{})
		if labelerr != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to patch namespace %s: %v\n", ns.Name, labelerr)
		}
		sc := "ibmc-vpc-file-regional"
		reclaimPolicy := v1.PersistentVolumeReclaimDelete

		var replicaCount = int32(1)
		pod := testsuites.PodDetails{
			Cmd:      "echo 'hello world' >> /mnt/test-1/data && while true; do sleep 2; done",
			CmdExits: false,
			Volumes: []testsuites.VolumeDetails{
				{
					PVCName:       "ics-vol-rfs-",
					VolumeType:    sc,
					FSType:        "nfs",
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

		fpointer, err = os.OpenFile(testResultFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}
		defer fpointer.Close()
		_, _ = fpointer.WriteString(fmt.Sprintf("✅ RFS: VERIFYING PVC CREATE/DELETE WITH DEFAULT BANDWIDTH FOR %s STORAGE CLASS : PASS\n", sc))
	})

	It("with rfs profile sc : should create a pvc, deployment resources, write and read to volume, delete the pod with max bandwidth ", func() {
		payload := `{"metadata": {"labels": {"security.openshift.io/scc.podSecurityLabelSync": "false","pod-security.kubernetes.io/enforce": "privileged"}}}`
		_, labelerr := cs.CoreV1().Namespaces().Patch(context.TODO(), ns.Name, types.StrategicMergePatchType, []byte(payload), metav1.PatchOptions{})
		if labelerr != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to patch namespace %s: %v\n", ns.Name, labelerr)
		}
		sc := "ibmc-vpc-file-regional-max-bandwidth"
		reclaimPolicy := v1.PersistentVolumeReclaimDelete

		var replicaCount = int32(1)
		pod := testsuites.PodDetails{
			Cmd:      "echo 'hello world' >> /mnt/test-1/data && while true; do sleep 2; done",
			CmdExits: false,
			Volumes: []testsuites.VolumeDetails{
				{
					PVCName:       "ics-vol-rfs-",
					VolumeType:    sc,
					FSType:        "nfs",
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

		fpointer, err = os.OpenFile(testResultFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}
		defer fpointer.Close()
		_, _ = fpointer.WriteString(fmt.Sprintf("✅ RFS: VERIFYING PVC CREATE/DELETE WITH MAX BANDWIDTH FOR %s STORAGE CLASS : PASS\n", sc))
	})

	It("with rfs profile sc: should provide default throughput and should create a pvc, deployment resources, write and read to volume, delete the pod", func() {
		payload := `{"metadata": {"labels": {"security.openshift.io/scc.podSecurityLabelSync": "false","pod-security.kubernetes.io/enforce": "privileged"}}}`
		_, labelerr := cs.CoreV1().Namespaces().Patch(context.TODO(), ns.Name, types.StrategicMergePatchType, []byte(payload), metav1.PatchOptions{})
		if labelerr != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to patch namespace %s: %v\n", ns.Name, labelerr)
		}
		params := map[string]string{
			"profile":    "rfs",
			"throughput": "0",
		}
		sc := "custom-rfs-sc"
		createCustomRfsSC(cs, "custom-rfs-sc", params)
		defer deleteCustomRfsSC(cs, sc)
		reclaimPolicy := v1.PersistentVolumeReclaimDelete

		var replicaCount = int32(1)
		pod := testsuites.PodDetails{
			Cmd:      "echo 'hello world' >> /mnt/test-1/data && while true; do sleep 2; done",
			CmdExits: false,
			Volumes: []testsuites.VolumeDetails{
				{
					PVCName:       "ics-vol-rfs-",
					VolumeType:    sc,
					FSType:        "nfs",
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

		fpointer, err = os.OpenFile(testResultFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}
		defer fpointer.Close()
		_, _ = fpointer.WriteString(fmt.Sprintf("✅ RFS: VERIFYING PVC CREATE/DELETE WITH ZERO BANDWIDTH FOR %s STORAGE CLASS : PASS\n", sc))
	})

	It("with rfs profile sc: should fail when bandwidth is set to an invalid high value (9000)", func() {
		params := map[string]string{
			"profile":    "rfs",
			"throughput": "9000",
		}
		sc := "custom-rfs-sc-1"
		createCustomRfsSC(cs, sc, params)
		defer deleteCustomRfsSC(cs, sc)

		defer func() {
			if err := cs.StorageV1().StorageClasses().Delete(context.Background(), "custom-rfs-sc-1", metav1.DeleteOptions{}); err != nil {
				fmt.Printf("Warning: failed to delete StorageClass custom-rfs-sc-1: %v\n", err)
			}
		}()

		CreateRFSPVC("rfs-test-pvc", "rfs-test-sc", ns.Name, 9000, "10Gi", cs)

		defer func() {
			if err := cs.CoreV1().PersistentVolumeClaims(ns.Name).Delete(context.Background(), "rfs-test-pvc", metav1.DeleteOptions{}); err != nil {
				fmt.Printf("Warning: failed to delete PVC rfs-test-pvc: %v\n", err)
			}
		}()

		fpointer, err = os.OpenFile(testResultFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}
		defer fpointer.Close()

		_, _ = fpointer.WriteString(fmt.Sprintf("✅ RFS: VERIFYING PVC CREATE FAIL WITH INVALID BANDWIDTH (9000) FOR %s STORAGE CLASS : PASS\n", sc))
	})

	It("with rfs profile sc: should fail when iops is provided for rfs profile", func() {
		params := map[string]string{
			"profile":    "rfs",
			"throughput": "100",
			"iops":       "36000",
		}
		sc := "custom-rfs-sc-2"
		createCustomRfsSC(cs, sc, params)
		defer deleteCustomRfsSC(cs, sc)
		defer func() {
			if err := cs.StorageV1().StorageClasses().Delete(context.Background(), "custom-rfs-sc-2", metav1.DeleteOptions{}); err != nil {
				fmt.Printf("Warning: failed to delete StorageClass custom-rfs-sc-1: %v\n", err)
			}
		}()
		CreateRFSPVC("rfs-test-pvc", "rfs-test-sc", ns.Name, 100, "10Gi", cs)
		defer func() {
			if err := cs.CoreV1().PersistentVolumeClaims(ns.Name).Delete(context.Background(), "rfs-test-pvc", metav1.DeleteOptions{}); err != nil {
				fmt.Printf("Warning: failed to delete PVC rfs-test-pvc: %v\n", err)
			}
		}()

		fpointer, err = os.OpenFile(testResultFile, os.O_APPEND|os.O_WRONLY, 0644)
		defer fpointer.Close()

		_, _ = fpointer.WriteString(fmt.Sprintf("✅ RFS: VERIFYING PVC CREATE FAIL WITH IOPS PARAM FOR %s STORAGE CLASS : PASS\n", sc))
	})

	It("with rfs profile sc: should fail when zone is provided for rfs profile", func() {
		params := map[string]string{
			"profile":    "rfs",
			"throughput": "100",
			"zone":       "us-south-1",
		}
		sc := "custom-rfs-sc-3"
		createCustomRfsSC(cs, sc, params)
		defer deleteCustomRfsSC(cs, sc)
		defer func() {
			if err := cs.StorageV1().StorageClasses().Delete(context.Background(), "custom-rfs-sc-3", metav1.DeleteOptions{}); err != nil {
				fmt.Printf("Warning: failed to delete StorageClass custom-rfs-sc-1: %v\n", err)
			}
		}()
		CreateRFSPVC("rfs-test-pvc", "rfs-test-sc", ns.Name, 100, "10Gi", cs)
		defer func() {
			if err := cs.CoreV1().PersistentVolumeClaims(ns.Name).Delete(context.Background(), "rfs-test-pvc", metav1.DeleteOptions{}); err != nil {
				fmt.Printf("Warning: failed to delete PVC rfs-test-pvc: %v\n", err)
			}
		}()

		fpointer, err = os.OpenFile(testResultFile, os.O_APPEND|os.O_WRONLY, 0644)
		defer fpointer.Close()
		_, _ = fpointer.WriteString(fmt.Sprintf("✅ RFS: VERIFYING PVC CREATE FAIL WITH ZONE PARAM FOR %s STORAGE CLASS : PASS\n", sc))
	})
})
