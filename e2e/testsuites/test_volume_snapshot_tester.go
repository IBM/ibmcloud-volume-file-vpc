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

	. "github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	restclientset "k8s.io/client-go/rest"
)

// DynamicallyProvisionedVolumeSnapshotTest will provision required StorageClass(es),VolumeSnapshotClass(es), PVC(s) and Pod(s)
// Waiting for the PV provisioner to create a new PV
// Testing if the Pod(s) can write and read to mounted volumes
// Create a snapshot, validate the data is still on the disk, and then write and read to it again
// And finally delete the snapshot
// This test only supports a single volume

type DynamicallyProvisionedVolumeSnapshotTest struct {
	Pod         PodDetails
	RestoredPod PodDetails
	PodCheck    *PodExecCheck
	PVCFail     bool
}

func (t *DynamicallyProvisionedVolumeSnapshotTest) Run(client clientset.Interface, restclient restclientset.Interface, namespace *v1.Namespace) {
	By("Executing Positive test scenario for volume snapshot")

	// --- Step 1: Create POD-1 with PVC-1 ---
	tpod := NewTestPod(client, namespace, t.Pod.Cmd)
	volume := t.Pod.Volumes[0]

	// 1. PVC-1 + PV
	tpvc, pvcCleanup := volume.SetupDynamicPersistentVolumeClaim(client, namespace, false)

	// Defer PVC-1 cleanup last
	for i := len(pvcCleanup) - 1; i >= 0; i-- {
		defer pvcCleanup[i]()
	}

	tpod.SetupVolume(tpvc.persistentVolumeClaim, volume.VolumeMount.NameGenerate+"1", volume.VolumeMount.MountPathGenerate+"1", volume.VolumeMount.ReadOnly)

	// 2. POD-1 creation
	By("Deploying POD-1")
	tpod.Create()
	defer tpod.Cleanup() // POD-1 cleanup (will run before PVC-1)

	By("Checking that POD-1 command exits with no error")
	tpod.WaitForSuccess()

	// --- Step 2: Create Snapshot-1 ---
	By("Taking snapshots")
	tvsc, cleanup := CreateVolumeSnapshotClass(restclient, namespace)
	defer cleanup() // snapshot class cleanup (runs after snapshot deletion)

	snapshot := tvsc.CreateSnapshot(tpvc.persistentVolumeClaim)
	defer tvsc.DeleteSnapshot(snapshot) // snapshot-1 deletion

	tvsc.ReadyToUse(snapshot, false)
	By("Snapshot Creation Completed")

	// --- Step 3: Restore snapshot-1 to PVC-2 ---
	t.RestoredPod.Volumes[0].DataSource = &DataSource{Name: snapshot.Name}
	trpod := NewTestPod(client, namespace, t.RestoredPod.Cmd)
	rvolume := t.RestoredPod.Volumes[0]

	By("Creating PersistentVolumeClaim from a Volume Snapshot")
	trpvc, rpvcCleanup := rvolume.SetupDynamicPersistentVolumeClaim(client, namespace, false)

	// Defer PVC-2 cleanup before snapshot deletion
	for i := len(rpvcCleanup) - 1; i >= 0; i-- {
		defer rpvcCleanup[i]()
	}

	trpod.SetupVolume(trpvc.persistentVolumeClaim, rvolume.VolumeMount.NameGenerate+"1", rvolume.VolumeMount.MountPathGenerate+"1", rvolume.VolumeMount.ReadOnly)

	By("Deploying POD-2 with a volume restored from the snapshot")
	trpod.Create()
	defer trpod.Cleanup() // POD-2 cleanup (runs first)

	By("Checking that POD-2 command exits with no error")
	trpod.WaitForRunningSlow()
	trpod.Exec(t.PodCheck.Cmd, t.PodCheck.ExpectedString01)
}

func (t *DynamicallyProvisionedVolumeSnapshotTest) VolumeSizeLess(client clientset.Interface, restclient restclientset.Interface, namespace *v1.Namespace) {
	By("Executing Negative test scenario for snapshot restore with smaller PVC size")

	volume := t.Pod.Volumes[0]

	// 1. Create PVC-1
	tpvc, pvc1Cleanup := volume.SetupDynamicPersistentVolumeClaim(client, namespace, false)

	for i := len(pvc1Cleanup) - 1; i >= 0; i-- {
		defer pvc1Cleanup[i]()
	}

	// 2. Create POD-1 with PVC-1 and write data
	tpod := NewTestPod(client, namespace, t.Pod.Cmd)
	tpod.SetupVolume(tpvc.persistentVolumeClaim, volume.VolumeMount.NameGenerate+"1", volume.VolumeMount.MountPathGenerate+"1", volume.VolumeMount.ReadOnly)

	By("Deploying POD-1")
	tpod.Create()
	defer tpod.Cleanup()

	By("Waiting for POD-1 to succeed")
	tpod.WaitForSuccess()

	// 3. Create snapshot-1
	tvsc, vscCleanup := CreateVolumeSnapshotClass(restclient, namespace)
	defer vscCleanup()

	By("Creating snapshot-1")
	snapshot := tvsc.CreateSnapshot(tpvc.persistentVolumeClaim)
	defer tvsc.DeleteSnapshot(snapshot)

	tvsc.ReadyToUse(snapshot, false)
	By("Snapshot-1 is ready")

	// 4. Attempt restore to smaller PVC-2 (expected failure)
	By("Attempting restore to PVC-2 with smaller size (should fail)")

	t.RestoredPod.Volumes[0].DataSource = &DataSource{Name: snapshot.Name}
	rvolume := t.RestoredPod.Volumes[0]

	restoredPVC, errList := rvolume.SetupDynamicPersistentVolumeClaim(client, namespace, true)

	// EXPECTED FAIL
	if errList == nil || len(errList) == 0 {
		Fail("Expected PVC restore with smaller size to fail, but it succeeded")
	}

	By("Restore FAILED as expected — marking test as PASS")

	// cleanup partially created PVC if any
	if restoredPVC != nil {
		By("Cleaning up partially created PVC-2")
		_ = client.CoreV1().PersistentVolumeClaims(namespace.Name).
			Delete(context.TODO(), restoredPVC.persistentVolumeClaim.Name, metav1.DeleteOptions{})
	}
	return

	// Cleanup of POD-1, snapshot-1, PVC-1 handled by defer
}
