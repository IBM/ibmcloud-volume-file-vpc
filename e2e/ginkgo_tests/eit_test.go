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
	"strings"
	"time"

	"github.com/IBM/ibmcloud-volume-file-vpc/e2e/testsuites"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
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
		// Initialize clientset - framework clientset may not be ready in BeforeAll
		// so we need to create it directly
		config, err := framework.LoadConfig()
		if err != nil {
			panic(fmt.Errorf("failed to load kubeconfig: %w", err))
		}
		cs, err = clientset.NewForConfig(config)
		if err != nil {
			panic(fmt.Errorf("failed to create clientset: %w", err))
		}

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
		fmt.Println("Os: ", os.Getenv("WORKER_OS"))
		rebootWorkersForRHCOS(cs)
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

	It("should verify EIT installation: all default worker-pool nodes must appear in EIT_ENABLED_WORKER_NODES", func() {
		fpointer, err = os.OpenFile(testResultFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}
		DeferCleanup(func() {
			if fpointer != nil {
				if CurrentSpecReport().Failed() {
					fpointer.WriteString("❌ DP2 EIT: VERIFYING EIT INSTALLATION ON DEFAULT WORKER POOL\n")
				} else {
					fpointer.WriteString("✅ DP2 EIT: VERIFYING EIT INSTALLATION ON DEFAULT WORKER POOL\n")
				}
				fpointer.Close()
			}
		})

		// Step 1 : Fetch nodes in the "default" worker pool
		nodeList, err := cs.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
			LabelSelector: "ibm-cloud.kubernetes.io/worker-pool-name=default",
		})
		Expect(err).NotTo(HaveOccurred(), "failed to list nodes in default worker pool")
		Expect(nodeList.Items).NotTo(BeEmpty(), "no nodes found in default worker pool")
		defaultPoolNodeNames := []string{}
		for _, node := range nodeList.Items {
			defaultPoolNodeNames = append(defaultPoolNodeNames, node.Name)
		}
		fmt.Printf("Nodes in default worker pool (%d): %v\n", len(defaultPoolNodeNames), defaultPoolNodeNames)

		// Step 2: Read EIT_ENABLED_WORKER_NODES from the status ConfigMap
		cmStatus, err := cs.CoreV1().ConfigMaps("kube-system").Get(context.TODO(), "file-csi-driver-status", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred(), "failed to get file-csi-driver-status ConfigMap")

		eitEnabledWorkerNodes, exists := cmStatus.Data["EIT_ENABLED_WORKER_NODES"]
		Expect(exists).To(BeTrue(), "EIT_ENABLED_WORKER_NODES key not found in file-csi-driver-status ConfigMap")

		fmt.Println("EIT_ENABLED_WORKER_NODES:")
		fmt.Println(eitEnabledWorkerNodes)

		// Parse the YAML value: map of worker-pool-name -> []string of node names
		var eitNodes map[string][]string
		Expect(yaml.Unmarshal([]byte(eitEnabledWorkerNodes), &eitNodes)).To(Succeed(), "failed to parse EIT_ENABLED_WORKER_NODES as YAML")

		defaultEITNodeNames, ok := eitNodes["default"]
		Expect(ok).To(BeTrue(), "key 'default' not found in EIT_ENABLED_WORKER_NODES")

		eitNodeNameSet := map[string]struct{}{}
		for _, name := range defaultEITNodeNames {
			eitNodeNameSet[strings.TrimSpace(name)] = struct{}{}
		}

		// Assert every node in the default pool is present in EIT_ENABLED_WORKER_NODES
		missingNodes := []string{}
		for _, name := range defaultPoolNodeNames {
			if _, found := eitNodeNameSet[name]; !found {
				missingNodes = append(missingNodes, name)
			}
		}
		Expect(missingNodes).To(BeEmpty(),
			"the following default worker-pool nodes are missing from EIT_ENABLED_WORKER_NODES: %v", missingNodes)
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
					fpointer.WriteString("❌ DP2 EIT: VERIFYING DEPLOYMENT WITH 3 REPLICAS MULTI-POD ACCESS\n")
				} else {
					fpointer.WriteString("✅ DP2 EIT: VERIFYING DEPLOYMENT WITH 3 REPLICAS MULTI-POD ACCESS\n")
				}
				fpointer.Close()
			}
		})

		reclaimPolicy := v1.PersistentVolumeReclaimDelete
		var replicaCount = int32(3)
		pod := testsuites.PodDetails{
			Cmd:      "echo 'hello world' >> /mnt/test-1/data && while true; do sleep 2; done",
			CmdExits: false,
			NodeSelector: map[string]string{
				"ibm-cloud.kubernetes.io/worker-pool-name": "default",
			},
			Volumes: []testsuites.VolumeDetails{
				{
					PVCName:       "ics-vol-scale-",
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
					fpointer.WriteString("❌ DP2 EIT: VERIFYING PVC EXPANSION USING POD\n")
				} else {
					fpointer.WriteString("✅ DP2 EIT: VERIFYING PVC EXPANSION USING POD\n")
				}
				fpointer.Close()
			}
		})

		pods := []testsuites.PodDetails{
			{
				Cmd:      "echo 'hello world' > /mnt/test-1/data && while true; do sleep 2; done",
				CmdExits: false,
				NodeSelector: map[string]string{
					"ibm-cloud.kubernetes.io/worker-pool-name": "default",
				},
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
		// Ensure clientset is initialized
		if cs == nil {
			config, err := framework.LoadConfig()
			if err != nil {
				panic(fmt.Errorf("failed to load kubeconfig in AfterAll: %w", err))
			}
			cs, err = clientset.NewForConfig(config)
			if err != nil {
				panic(fmt.Errorf("failed to create clientset in AfterAll: %w", err))
			}
		}

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
					fpointer.WriteString("❌ DP2 EIT: PROVISIONING DEPLOYMENT ON WP WHERE EIT IS NOT ENABLED MUST FAIL\n")
				} else {
					fpointer.WriteString("✅ DP2 EIT: PROVISIONING DEPLOYMENT ON WP WHERE EIT IS NOT ENABLED MUST FAIL\n")
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

// **New EIT-RFS test cases using utility functions**
// Note: For RFS-EIT, stunnel is automatically enabled via storage class parameter.
// No need to enable EIT in ConfigMap - it's handled by the storage class.

var _ = Describe("[ics-e2e] [eit-rfs] [stunnel-verification] EIT Volume with Stunnel Tunnel Verification", func() {
	f := framework.NewDefaultFramework("ics-e2e-eit-verify")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged
	var (
		cs     clientset.Interface
		ns     *v1.Namespace
		config *restclientset.Config
	)
	BeforeEach(func() {
		cs = f.ClientSet
		ns = f.Namespace
		config = f.ClientConfig()
	})

	It("should create EIT volume, verify stunnel tunnel exists, and verify mount uses tunnel", func() {
		payload := `{"metadata": {"labels": {"security.openshift.io/scc.podSecurityLabelSync": "false","pod-security.kubernetes.io/enforce": "privileged"}}}`
		_, labelerr := cs.CoreV1().Namespaces().Patch(context.TODO(), ns.Name, types.StrategicMergePatchType, []byte(payload), metav1.PatchOptions{})
		if labelerr != nil {
			panic(labelerr)
		}

		reclaimPolicy := v1.PersistentVolumeReclaimDelete
		pod := testsuites.PodDetails{
			Cmd: "echo 'stunnel test' > /mnt/test-1/data && while true; do sleep 2; done",
			Volumes: []testsuites.VolumeDetails{
				{
					PVCName:       "eit-rfs-stunnel-verify-",
					VolumeType:    "ibmc-vpc-file-rfs-eit",
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

		test := testsuites.DynamicallyProvisionedEITPodTest{
			Pod: pod,
			PodCheck: &testsuites.PodExecCheck{
				Cmd:              []string{"cat", "/mnt/test-1/data"},
				ExpectedString01: "stunnel test\n",
			},
			Config: config,
		}

		// Run the test - it handles all verification internally
		test.Run(cs, ns)
	})
})

var _ = Describe("[ics-e2e] [eit-rfs] [multi-volume] EIT Pod with Multiple Volumes and Tunnel Verification", func() {
	f := framework.NewDefaultFramework("ics-e2e-eit-multi")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged
	var (
		cs     clientset.Interface
		ns     *v1.Namespace
		config *restclientset.Config
	)
	BeforeEach(func() {
		cs = f.ClientSet
		ns = f.Namespace
		config = f.ClientConfig()
	})

	It("should create pod with 2 EIT volumes and verify both stunnel tunnels", func() {
		payload := `{"metadata": {"labels": {"security.openshift.io/scc.podSecurityLabelSync": "false","pod-security.kubernetes.io/enforce": "privileged"}}}`
		_, labelerr := cs.CoreV1().Namespaces().Patch(context.TODO(), ns.Name, types.StrategicMergePatchType, []byte(payload), metav1.PatchOptions{})
		if labelerr != nil {
			panic(labelerr)
		}

		reclaimPolicy := v1.PersistentVolumeReclaimDelete
		pod := testsuites.PodDetails{
			Cmd: "echo 'vol1' > /mnt/test-1/data && echo 'vol2' > /mnt/test-2/data && while true; do sleep 2; done",
			Volumes: []testsuites.VolumeDetails{
				{
					PVCName:       "eit-rfs-multi-vol1-",
					VolumeType:    "ibmc-vpc-file-rfs-eit",
					ClaimSize:     "10Gi",
					ReclaimPolicy: &reclaimPolicy,
					MountOptions:  []string{"rw"},
					VolumeMount: testsuites.VolumeMountDetails{
						NameGenerate:      "test-volume-",
						MountPathGenerate: "/mnt/test-",
					},
				},
				{
					PVCName:       "eit-rfs-multi-vol2-",
					VolumeType:    "ibmc-vpc-file-rfs-eit",
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

		test := testsuites.DynamicallyProvisionedEITMultiVolPodTest{
			Pod: pod,
			PodCheck: &testsuites.PodExecCheck{
				Cmd:              []string{"cat", "/mnt/test-1/data"},
				ExpectedString01: "vol1\n",
			},
			Config: config,
		}

		// Run the test - it handles multi-volume verification internally
		test.Run(cs, ns)
	})
})

var _ = Describe("[ics-e2e] [eit-rfs] [cleanup] EIT Volume Cleanup and Tunnel Removal Verification", func() {
	f := framework.NewDefaultFramework("ics-e2e-eit-cleanup")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged
	var (
		cs     clientset.Interface
		ns     *v1.Namespace
		config *restclientset.Config
	)
	BeforeEach(func() {
		cs = f.ClientSet
		ns = f.Namespace
		config = f.ClientConfig()
	})

	It("should create EIT volume, verify tunnel, delete pod, and verify tunnel cleanup", func() {
		payload := `{"metadata": {"labels": {"security.openshift.io/scc.podSecurityLabelSync": "false","pod-security.kubernetes.io/enforce": "privileged"}}}`
		_, labelerr := cs.CoreV1().Namespaces().Patch(context.TODO(), ns.Name, types.StrategicMergePatchType, []byte(payload), metav1.PatchOptions{})
		if labelerr != nil {
			panic(labelerr)
		}

		reclaimPolicy := v1.PersistentVolumeReclaimDelete
		pvcName := "eit-rfs-cleanup-test-"
		pod := testsuites.PodDetails{
			Cmd: "echo 'cleanup test' > /mnt/test-1/data && while true; do sleep 2; done",
			Volumes: []testsuites.VolumeDetails{
				{
					PVCName:       pvcName,
					VolumeType:    "ibmc-vpc-file-rfs-eit",
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

		test := testsuites.DynamicallyProvisionedEITPodTest{
			Pod: pod,
			PodCheck: &testsuites.PodExecCheck{
				Cmd:              []string{"cat", "/mnt/test-1/data"},
				ExpectedString01: "cleanup test\n",
			},
			Config: config,
		}

		// Run the test - it handles tunnel verification and cleanup internally
		test.Run(cs, ns)
	})
})

var _ = Describe("[ics-e2e] [eit-rfs] [node-restart] EIT Volume with CSI Node Server Restart", func() {
	f := framework.NewDefaultFramework("ics-e2e-eit-restart")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged
	var (
		cs     clientset.Interface
		ns     *v1.Namespace
		config *restclientset.Config
	)
	BeforeEach(func() {
		cs = f.ClientSet
		ns = f.Namespace
		config = f.ClientConfig()
	})

	It("should maintain EIT volume functionality after CSI node server restart", func() {
		payload := `{"metadata": {"labels": {"security.openshift.io/scc.podSecurityLabelSync": "false","pod-security.kubernetes.io/enforce": "privileged"}}}`
		_, labelerr := cs.CoreV1().Namespaces().Patch(context.TODO(), ns.Name, types.StrategicMergePatchType, []byte(payload), metav1.PatchOptions{})
		if labelerr != nil {
			panic(labelerr)
		}

		reclaimPolicy := v1.PersistentVolumeReclaimDelete
		pvcName := "eit-rfs-restart-test-"
		pod := testsuites.PodDetails{
			Cmd: "echo 'initial data' > /mnt/test-1/data && while true; do sleep 2; done",
			Volumes: []testsuites.VolumeDetails{
				{
					PVCName:       pvcName,
					VolumeType:    "ibmc-vpc-file-rfs-eit",
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

		// Setup pod with volume
		test := testsuites.DynamicallyProvisionedEITPodTest{
			Pod:    pod,
			Config: config,
		}
		tpod, cleanup := test.Pod.SetupWithDynamicVolumes(cs, ns)
		for i := range cleanup {
			defer cleanup[i]()
		}

		fmt.Println("Creating pod with EIT volume...")
		tpod.Create()
		defer tpod.Cleanup()

		fmt.Println("Waiting for pod to be running...")
		tpod.WaitForRunningSlow()

		// Get pod and extract actual PVC name
		podList, err := cs.CoreV1().Pods(ns.Name).List(context.TODO(), metav1.ListOptions{
			LabelSelector: "app=ics-vol-e2e",
		})
		if err != nil || len(podList.Items) == 0 {
			panic(fmt.Errorf("failed to find test pod: %w", err))
		}
		podObj := &podList.Items[0]
		podName := podObj.Name
		fmt.Printf("Test pod name: %s\n", podName)

		// Get actual PVC name from pod spec (Kubernetes adds random suffix)
		if len(podObj.Spec.Volumes) == 0 || podObj.Spec.Volumes[0].PersistentVolumeClaim == nil {
			panic(fmt.Errorf("pod has no PVC volume"))
		}
		actualPVCName := podObj.Spec.Volumes[0].PersistentVolumeClaim.ClaimName
		fmt.Printf("Actual PVC name: %s\n", actualPVCName)

		// Get volume ID
		volumeID, err := testsuites.GetVolumeIDFromPVC(cs, ns.Name, actualPVCName)
		if err != nil {
			panic(fmt.Errorf("failed to get volume ID: %w", err))
		}
		fmt.Printf("Volume ID: %s\n", volumeID)

		// Get node name
		if podObj.Spec.NodeName == "" {
			err = fmt.Errorf("pod not scheduled to any node")
			panic(fmt.Errorf("failed to get pod: %w", err))
		}
		nodeName := podObj.Spec.NodeName
		fmt.Printf("Pod running on node: %s\n", nodeName)

		// Verify initial tunnel and mount
		fmt.Println("Verifying initial stunnel tunnel...")
		port, err := testsuites.VerifyStunnelTunnel(config, cs, ns.Name, podName, volumeID)
		if err != nil {
			panic(fmt.Errorf("initial tunnel verification failed: %w", err))
		}
		fmt.Printf("Initial tunnel verified on port: %d\n", port)

		err = testsuites.VerifyMountUsesStunnel(config, cs, ns.Name, podName, port)
		if err != nil {
			panic(fmt.Errorf("initial mount verification failed: %w", err))
		}
		fmt.Println("Initial mount verified to use stunnel")

		// Verify initial data read
		fmt.Println("Verifying initial data read...")
		tpod.Exec([]string{"cat", "/mnt/test-1/data"}, "initial data\n")

		// Find and restart CSI node server pod on the same node
		fmt.Printf("Finding CSI node server pod on node %s...\n", nodeName)
		csiPod, err := testsuites.GetCSIDriverPod(cs, nodeName)
		if err != nil {
			panic(fmt.Errorf("failed to find CSI node server pod: %w", err))
		}
		fmt.Printf("Found CSI node server pod: %s\n", csiPod.Name)

		// Delete CSI node server pod to trigger restart
		fmt.Println("Restarting CSI node server pod...")
		err = cs.CoreV1().Pods("kube-system").Delete(context.TODO(), csiPod.Name, metav1.DeleteOptions{})
		if err != nil {
			panic(fmt.Errorf("failed to delete CSI node server pod: %w", err))
		}

		// Wait for new CSI node server pod to be ready
		fmt.Println("Waiting for new CSI node server pod to be ready...")
		err = wait.PollImmediate(5*time.Second, 3*time.Minute, func() (bool, error) {
			newCSIPod, err := testsuites.GetCSIDriverPod(cs, nodeName)
			if err != nil {
				return false, nil
			}
			// Check if it's a different pod (new one)
			if newCSIPod.Name == csiPod.Name {
				return false, nil
			}
			// Check if new pod is ready
			for _, condition := range newCSIPod.Status.Conditions {
				if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
					fmt.Printf("New CSI node server pod ready: %s\n", newCSIPod.Name)
					return true, nil
				}
			}
			return false, nil
		})
		if err != nil {
			panic(fmt.Errorf("CSI node server pod did not become ready: %w", err))
		}

		// Wait a bit for stunnel to be fully operational
		fmt.Println("Waiting for stunnel to be fully operational...")
		time.Sleep(10 * time.Second)

		// Verify tunnel still exists after restart
		fmt.Println("Verifying stunnel tunnel after CSI restart...")
		port, err = testsuites.VerifyStunnelTunnel(config, cs, ns.Name, podName, volumeID)
		if err != nil {
			panic(fmt.Errorf("tunnel verification failed after CSI restart: %w", err))
		}
		fmt.Printf("Tunnel verified after restart on port: %d\n", port)

		// Verify mount still uses stunnel
		err = testsuites.VerifyMountUsesStunnel(config, cs, ns.Name, podName, port)
		if err != nil {
			panic(fmt.Errorf("mount verification failed after CSI restart: %w", err))
		}
		fmt.Println("Mount still uses stunnel after restart")

		// Verify can still read existing data
		fmt.Println("Verifying can read existing data after restart...")
		tpod.Exec([]string{"cat", "/mnt/test-1/data"}, "initial data\n")

		// Verify can write new data after restart
		fmt.Println("Writing new data after restart...")
		tpod.Exec([]string{"sh", "-c", "echo 'post-restart data' >> /mnt/test-1/data"}, "")

		// Verify can read new data
		fmt.Println("Verifying can read new data after restart...")
		tpod.Exec([]string{"cat", "/mnt/test-1/data"}, "initial data\npost-restart data\n")

		fmt.Println("All operations successful after CSI node server restart!")

		// Cleanup: Delete pod and verify unmount works
		fmt.Println("Deleting pod to verify unmount works after restart...")
		tpod.Cleanup()

		// Wait for pod deletion
		err = wait.PollImmediate(5*time.Second, 2*time.Minute, func() (bool, error) {
			_, err := cs.CoreV1().Pods(ns.Name).Get(context.TODO(), podName, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		})
		if err != nil {
			panic(fmt.Errorf("pod did not delete in time: %w", err))
		}

		// Verify tunnel cleanup works after restart
		fmt.Println("Verifying tunnel cleanup after restart...")
		err = testsuites.WaitForTunnelCleanup(config, cs, nodeName, volumeID)
		if err != nil {
			panic(fmt.Errorf("tunnel cleanup verification failed after restart: %w", err))
		}
		fmt.Printf("Tunnel cleanup successful after restart for volume %s\n", volumeID)
	})
})
