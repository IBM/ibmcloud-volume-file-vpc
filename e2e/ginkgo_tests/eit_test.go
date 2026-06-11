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
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/IBM/ibmcloud-volume-file-vpc/e2e/testsuites"
	. "github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
)

var _ = Describe("[ics-e2e] [eit] Dynamic Provisioning for ibmc-vpc-file-eit SC with DaemonSet and Resize", Ordered, func() {
	f := framework.NewDefaultFramework("ics-e2e-eit")
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

	BeforeAll(func() {
		cs = f.ClientSet
		wp_list := "default"
		cmData := map[string]interface{}{
			"data": map[string]string{
				"ENABLE_EIT":               "true",
				"EIT_ENABLED_WORKER_POOLS": wp_list,
			},
		}
		cmDataBytes, err := json.Marshal(cmData)
		if err != nil {
			panic(err)
		}

		var cm *v1.ConfigMap
		cm, err = cs.CoreV1().ConfigMaps("kube-system").Patch(context.TODO(), "addon-vpc-file-csi-driver-configmap", types.MergePatchType, cmDataBytes, metav1.PatchOptions{})
		if err != nil {
			panic(err)
		}

		fmt.Println("Updated ConfigMap 'addon-vpc-file-csi-driver-configmap': ", cm.Data)

		fmt.Printf("Sleep for %s to install EIT packages...\n", waitForPackageInstallation)
		time.Sleep(waitForPackageInstallation)
		rebootWorkersForRHCOS()
		cm_status, err := cs.CoreV1().ConfigMaps("kube-system").Get(context.TODO(), "file-csi-driver-status", metav1.GetOptions{})
		if err != nil {
			panic(err)
		}
		eitEnabledWorkerNodes, exists := cm_status.Data["EIT_ENABLED_WORKER_NODES"]
		if !exists {
			fmt.Println("EIT_ENABLED_WORKER_NODES not found in ConfigMap")
			err = fmt.Errorf("unknown problem with 'file-csi-driver-status' configmap")
			panic(err)
		}
		fmt.Println("EIT_ENABLED_WORKER_NODES:")
		fmt.Println(eitEnabledWorkerNodes)
	})

	BeforeEach(func() {
		cs = f.ClientSet
		ns = f.Namespace
	})

	It("should scale deployment to 3 replicas and verify multi-pod access to EIT volume", func() {
		payload := `{"metadata": {"labels": {"security.openshift.io/scc.podSecurityLabelSync": "false","pod-security.kubernetes.io/enforce": "privileged"}}}`
		_, labelerr := cs.CoreV1().Namespaces().Patch(context.TODO(), ns.Name, types.StrategicMergePatchType, []byte(payload), metav1.PatchOptions{})
		if labelerr != nil {
			panic(labelerr)
		}
		fpointer, err = os.OpenFile(testResultFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}

		// Use DeferCleanup to ensure result is written even on failure
		DeferCleanup(func() {
			if fpointer != nil {
				if CurrentSpecReport().Failed() {
					fpointer.WriteString("❌ EIT: VERIFYING DEPLOYMENT WITH 3 REPLICAS MULTI-POD ACCESS : FAIL\n")
				} else {
					fpointer.WriteString("✅ EIT: VERIFYING DEPLOYMENT WITH 3 REPLICAS MULTI-POD ACCESS : PASS\n")
				}
				fpointer.Close()
			}
		})

		reclaimPolicy := v1.PersistentVolumeReclaimDelete
		var replicaCount = int32(3)
		pod := testsuites.PodDetails{
			Cmd:      "echo 'hello world' >> /mnt/test-1/data && while true; do sleep 2; done",
			CmdExits: false,
			Volumes: []testsuites.VolumeDetails{
				{
					PVCName:       "ics-vol-scale-",
					VolumeType:    "ibmc-vpc-file-eit",
					FSType:        "ibmshare",
					ClaimSize:     "10Gi",
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

	It("should create pv, pvc and pod resources, and resize the volume", func() {
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

		// Use DeferCleanup to ensure result is written even on failure
		DeferCleanup(func() {
			if fpointer != nil {
				if CurrentSpecReport().Failed() {
					fpointer.WriteString("❌ EIT: VERIFYING PVC EXPANSION USING POD : FAIL\n")
				} else {
					fpointer.WriteString("✅ EIT: VERIFYING PVC EXPANSION USING POD : PASS\n")
				}
				fpointer.Close()
			}
		})

		pods := []testsuites.PodDetails{
			{
				Cmd:      "echo 'hello world' > /mnt/test-1/data && while true; do sleep 2; done",
				CmdExits: false,
				Volumes: []testsuites.VolumeDetails{
					{
						PVCName:       "ics-vol-resize-",
						VolumeType:    "ibmc-vpc-file-eit",
						ClaimSize:     "10Gi",
						ReclaimPolicy: &reclaimPolicy,
						MountOptions:  []string{"rw"},
						VolumeMount: testsuites.VolumeMountDetails{
							NameGenerate:      "test-volume-",
							MountPathGenerate: "/mnt/test-",
						},
					},
				},
			},
		}
		test := testsuites.DynamicallyProvisionedResizeVolumeTest{
			Pods: pods,
			PodCheck: &testsuites.PodExecCheck{
				Cmd:              []string{"cat", "/mnt/test-1/data"},
				ExpectedString01: "hello world\n",
				ExpectedString02: "hello world\nhello world\n",
			},
			ExpandVolSizeG: 40,
			ExpandedSize:   40,
		}
		test.Run(cs, ns)
	})

	AfterAll(func() {
		cmData := map[string]interface{}{
			"data": map[string]string{
				"ENABLE_EIT": "false",
			},
		}
		cmDataBytes, err := json.Marshal(cmData)
		if err != nil {
			panic(err)
		}

		_, err = cs.CoreV1().ConfigMaps("kube-system").Patch(context.TODO(), "addon-vpc-file-csi-driver-configmap", types.MergePatchType, cmDataBytes, metav1.PatchOptions{})
		if err != nil {
			panic(err)
		}

		fmt.Printf("Sleep for %s to uninstall EIT packages...\n", waitForPackageInstallation)
		time.Sleep(waitForPackageInstallation)
	})
})

var _ = Describe("[ics-e2e] [eit] Dynamic Provisioning on worker-pool where EIT is not enabled,", func() {
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
		fpointer, err = os.OpenFile(testResultFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}

		cs = f.ClientSet
		ns = f.Namespace

		// Disable EIT entirely
		cmData := map[string]interface{}{
			"data": map[string]string{
				"ENABLE_EIT": "false",
			},
		}
		cmDataBytes, err := json.Marshal(cmData)
		if err != nil {
			panic(err)
		}

		var cm *v1.ConfigMap
		cm, err = cs.CoreV1().ConfigMaps("kube-system").Patch(context.TODO(), "addon-vpc-file-csi-driver-configmap", types.MergePatchType, cmDataBytes, metav1.PatchOptions{})
		if err != nil {
			panic(err)
		}

		fmt.Println("Updated ConfigMap 'addon-vpc-file-csi-driver-configmap' to disable EIT: ", cm.Data)

		fmt.Printf("Sleep for %s to ensure EIT is disabled...", waitForPackageInstallation)
		time.Sleep(waitForPackageInstallation)
	})

	It("should create pv, pvc, deployment resources. Pod should be stuck in 'ContainerCreating' state", func() {
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

		// Use DeferCleanup to ensure result is written even on failure
		DeferCleanup(func() {
			if fpointer != nil {
				if CurrentSpecReport().Failed() {
					fpointer.WriteString("❌ EIT: PROVISIONING DEPLOYMENT ON WP WHERE EIT IS NOT ENABLED MUST FAIL : FAIL\n")
				} else {
					fpointer.WriteString("✅ EIT: PROVISIONING DEPLOYMENT ON WP WHERE EIT IS NOT ENABLED MUST FAIL : PASS\n")
				}
				fpointer.Close()
			}
		})

		var replicaCount = int32(3)
		pod := testsuites.PodDetails{
			Cmd:      "echo 'hello world' >> /mnt/test-1/data && while true; do sleep 2; done",
			CmdExits: false,
			Volumes: []testsuites.VolumeDetails{
				{
					PVCName:       "ics-vol-dp2-",
					VolumeType:    "ibmc-vpc-file-eit",
					FSType:        "ibmshare",
					ClaimSize:     "10Gi",
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
		test.RunShouldFail(cs, ns)
	})
})
