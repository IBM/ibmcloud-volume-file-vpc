/**
 * Copyright 2025 IBM Corp.
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

package testsuites

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
)

// DynamicallyProvisionedEITPodTest tests EIT (Encryption in Transit) volume provisioning
// Testing if the Pod can mount EIT-enabled volumes through stunnel tunnel
type DynamicallyProvisionedEITPodTest struct {
	Pod      PodDetails
	PodCheck *PodExecCheck
	Config   *restclient.Config
}

func (t *DynamicallyProvisionedEITPodTest) Run(client clientset.Interface, namespace *v1.Namespace) {
	tpod, cleanup := t.Pod.SetupWithDynamicVolumes(client, namespace)
	// defer must be called here for resources not get removed before using them
	for i := range cleanup {
		defer cleanup[i]()
	}

	By("deploying the pod with EIT volume")
	tpod.Create()
	defer tpod.Cleanup()

	By("checking that the pod is running")
	tpod.WaitForRunningSlow()

	// Get the actual PVC name from the pod (Kubernetes adds random suffix)
	By("getting actual PVC name from pod")
	pod, err := client.CoreV1().Pods(namespace.Name).Get(context.TODO(), tpod.pod.Name, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred(), "Failed to get pod")
	Expect(len(pod.Spec.Volumes)).To(BeNumerically(">", 0), "Pod should have at least one volume")
	Expect(pod.Spec.Volumes[0].PersistentVolumeClaim).NotTo(BeNil(), "First volume should be a PVC")

	pvcName := pod.Spec.Volumes[0].PersistentVolumeClaim.ClaimName
	By(fmt.Sprintf("getting volume ID from PVC %s", pvcName))
	volumeID, err := GetVolumeIDFromPVC(client, namespace.Name, pvcName)
	Expect(err).NotTo(HaveOccurred(), "Failed to get volume ID from PVC")
	By(fmt.Sprintf("Volume ID: %s", volumeID))

	// Verify stunnel tunnel is created
	By("verifying stunnel tunnel is created")
	port, err := VerifyStunnelTunnel(t.Config, client, namespace.Name, tpod.pod.Name, volumeID)
	Expect(err).NotTo(HaveOccurred(), "Stunnel tunnel should be created")
	By(fmt.Sprintf("Stunnel tunnel created on port: %d", port))

	// Verify mount uses stunnel
	By("verifying mount uses stunnel tunnel")
	err = VerifyMountUsesStunnel(t.Config, client, namespace.Name, tpod.pod.Name, port)
	Expect(err).NotTo(HaveOccurred(), "Mount should use stunnel tunnel")

	// Run pod exec check if provided
	if t.PodCheck != nil {
		By("checking pod exec - write and read data")
		tpod.Exec(t.PodCheck.Cmd, t.PodCheck.ExpectedString01)
	}

	// Get node name for cleanup verification
	nodeName, err := GetPodNode(client, namespace.Name, tpod.pod.Name)
	Expect(err).NotTo(HaveOccurred(), "Failed to get pod node")

	// Delete pod and verify tunnel cleanup
	By("deleting pod and verifying tunnel cleanup")
	tpod.Cleanup()

	By("waiting for tunnel config to be removed")
	err = WaitForTunnelCleanup(t.Config, client, nodeName, volumeID)
	Expect(err).NotTo(HaveOccurred(), "Tunnel config should be removed after pod deletion")

	// Verify stunnel is in clean state (only main daemon, one listener, no configs)
	By("verifying stunnel is in clean state")
	err = VerifyStunnelCleanState(t.Config, client, nodeName)
	Expect(err).NotTo(HaveOccurred(), "Stunnel should be in clean state after all volumes unmounted")
}

// DynamicallyProvisionedEITMultiVolPodTest tests pod with multiple EIT volumes
type DynamicallyProvisionedEITMultiVolPodTest struct {
	Pod      PodDetails
	PodCheck *PodExecCheck
	Config   *restclient.Config
}

func (t *DynamicallyProvisionedEITMultiVolPodTest) Run(client clientset.Interface, namespace *v1.Namespace) {
	tpod, cleanup := t.Pod.SetupWithDynamicVolumes(client, namespace)
	for i := range cleanup {
		defer cleanup[i]()
	}

	By("deploying pod with multiple EIT volumes")
	tpod.Create()
	defer tpod.Cleanup()

	By("checking that the pod is running")
	tpod.WaitForRunningSlow()

	// Get pod name from the namespace
	podList, err := client.CoreV1().Pods(namespace.Name).List(context.TODO(), metav1.ListOptions{
		LabelSelector: "app=ics-vol-e2e",
	})
	Expect(err).NotTo(HaveOccurred(), "Failed to list pods")
	Expect(len(podList.Items)).To(BeNumerically(">", 0), "No test pods found")
	podName := podList.Items[0].Name

	// Get actual PVC names from the pod's volumes
	podObj := &podList.Items[0]

	// Verify tunnels for all volumes
	ports := make(map[string]int)
	for i, vol := range podObj.Spec.Volumes {
		if vol.PersistentVolumeClaim != nil {
			pvcName := vol.PersistentVolumeClaim.ClaimName
			By(fmt.Sprintf("verifying tunnel for PVC %s (volume %d)", pvcName, i+1))
			volumeID, err := GetVolumeIDFromPVC(client, namespace.Name, pvcName)
			Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Failed to get volume ID for PVC %s", pvcName))

			port, err := VerifyStunnelTunnel(t.Config, client, namespace.Name, podName, volumeID)
			Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Stunnel tunnel should be created for PVC %s", pvcName))
			ports[volumeID] = port
			By(fmt.Sprintf("PVC %s: tunnel on port %d", pvcName, port))
		}
	}

	// Verify all ports are unique
	By("verifying all ports are unique")
	portSet := make(map[int]bool)
	for _, port := range ports {
		Expect(portSet[port]).To(BeFalse(), fmt.Sprintf("Port %d is duplicated", port))
		portSet[port] = true
	}

	// Run pod exec check if provided
	if t.PodCheck != nil {
		By("checking pod exec - write and read data on all volumes")
		tpod.Exec(t.PodCheck.Cmd, t.PodCheck.ExpectedString01)
	}
}

// DynamicallyProvisionedEITRWXTest tests ReadWriteMany with EIT volumes
type DynamicallyProvisionedEITRWXTest struct {
	Pod1     PodDetails
	Pod2     PodDetails
	PodCheck *PodExecCheck
	Config   *restclient.Config
}

func (t *DynamicallyProvisionedEITRWXTest) Run(client clientset.Interface, namespace *v1.Namespace) {
	// Setup PVC first
	volume := t.Pod1.Volumes[0]
	tpvc, cleanupFuncs := volume.SetupDynamicPersistentVolumeClaim(client, namespace, false)
	for i := range cleanupFuncs {
		defer cleanupFuncs[i]()
	}

	volumeID, err := GetVolumeIDFromPVC(client, namespace.Name, tpvc.name)
	Expect(err).NotTo(HaveOccurred(), "Failed to get volume ID")

	// Create first pod
	By("creating first pod with EIT volume")
	tpod1, cleanup1 := t.Pod1.SetupWithPVC(client, namespace, "eit-rwx-pod-1")
	for i := range cleanup1 {
		defer cleanup1[i]()
	}
	tpod1.Create()
	defer tpod1.Cleanup()

	By("waiting for first pod to be running")
	tpod1.WaitForRunningSlow()

	// Verify tunnel for pod1
	By("verifying stunnel tunnel for first pod")
	port1, err := VerifyStunnelTunnel(t.Config, client, namespace.Name, tpod1.pod.Name, volumeID)
	Expect(err).NotTo(HaveOccurred(), "Stunnel tunnel should be created for pod1")
	By(fmt.Sprintf("Pod1 tunnel on port: %d", port1))

	// Write data from pod1
	if t.PodCheck != nil {
		By("writing data from first pod")
		tpod1.Exec(t.PodCheck.Cmd, t.PodCheck.ExpectedString01)
	}

	// Create second pod with same PVC
	By("creating second pod with same EIT volume")
	tpod2, cleanup2 := t.Pod2.SetupWithPVC(client, namespace, "eit-rwx-pod-2")
	for i := range cleanup2 {
		defer cleanup2[i]()
	}
	tpod2.Create()
	defer tpod2.Cleanup()

	By("waiting for second pod to be running")
	tpod2.WaitForRunningSlow()

	// Verify tunnel for pod2
	By("verifying stunnel tunnel for second pod")
	port2, err := VerifyStunnelTunnel(t.Config, client, namespace.Name, tpod2.pod.Name, volumeID)
	Expect(err).NotTo(HaveOccurred(), "Stunnel tunnel should be created for pod2")
	By(fmt.Sprintf("Pod2 tunnel on port: %d", port2))

	// Verify both pods can access the data
	if t.PodCheck != nil {
		By("reading data from second pod")
		tpod2.Exec(t.PodCheck.Cmd, t.PodCheck.ExpectedString02)
	}

	// Verify ports are different (each pod gets its own tunnel)
	By("verifying each pod has its own tunnel port")
	Expect(port1).NotTo(Equal(port2), "Each pod should have its own tunnel port")
}

// DynamicallyProvisionedEITPodRestartTest tests pod restart with EIT volume
type DynamicallyProvisionedEITPodRestartTest struct {
	Pod      PodDetails
	PodCheck *PodExecCheck
	Config   *restclient.Config
}

func (t *DynamicallyProvisionedEITPodRestartTest) Run(client clientset.Interface, namespace *v1.Namespace) {
	// Setup PVC first
	volume := t.Pod.Volumes[0]
	tpvc, cleanupFuncs := volume.SetupDynamicPersistentVolumeClaim(client, namespace, false)
	for i := range cleanupFuncs {
		defer cleanupFuncs[i]()
	}

	volumeID, err := GetVolumeIDFromPVC(client, namespace.Name, tpvc.name)
	Expect(err).NotTo(HaveOccurred(), "Failed to get volume ID")

	// Create first pod
	By("creating pod with EIT volume")
	tpod1, cleanup1 := t.Pod.SetupWithPVC(client, namespace, "eit-restart-pod-1")
	for i := range cleanup1 {
		defer cleanup1[i]()
	}
	tpod1.Create()

	By("waiting for pod to be running")
	tpod1.WaitForRunningSlow()

	// Verify tunnel
	By("verifying stunnel tunnel is created")
	port1, err := VerifyStunnelTunnel(t.Config, client, namespace.Name, tpod1.pod.Name, volumeID)
	Expect(err).NotTo(HaveOccurred(), "Stunnel tunnel should be created")
	By(fmt.Sprintf("Initial tunnel on port: %d", port1))

	// Write data
	if t.PodCheck != nil {
		By("writing test data")
		tpod1.Exec(t.PodCheck.Cmd, t.PodCheck.ExpectedString01)
	}

	// Get node name for cleanup verification
	nodeName, err := GetPodNode(client, namespace.Name, tpod1.pod.Name)
	Expect(err).NotTo(HaveOccurred(), "Failed to get pod node")

	// Delete first pod
	By("deleting first pod")
	tpod1.Cleanup()

	// Wait for tunnel cleanup
	By("waiting for tunnel cleanup")
	err = WaitForTunnelCleanup(t.Config, client, nodeName, volumeID)
	Expect(err).NotTo(HaveOccurred(), "Tunnel should be cleaned up")

	// Create second pod with same PVC
	By("creating new pod with same PVC")
	tpod2, cleanup2 := t.Pod.SetupWithPVC(client, namespace, "eit-restart-pod-2")
	for i := range cleanup2 {
		defer cleanup2[i]()
	}
	tpod2.Create()
	defer tpod2.Cleanup()

	By("waiting for new pod to be running")
	tpod2.WaitForRunningSlow()

	// Verify new tunnel
	By("verifying new stunnel tunnel is created")
	port2, err := VerifyStunnelTunnel(t.Config, client, namespace.Name, tpod2.pod.Name, volumeID)
	Expect(err).NotTo(HaveOccurred(), "New stunnel tunnel should be created")
	By(fmt.Sprintf("New tunnel on port: %d", port2))

	// Verify data persists
	if t.PodCheck != nil {
		By("verifying data persists after pod restart")
		tpod2.Exec(t.PodCheck.Cmd, t.PodCheck.ExpectedString02)
	}
}

// Made with Bob
