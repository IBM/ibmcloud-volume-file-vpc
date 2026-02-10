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

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
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

func (t *DynamicallyProvisionedVolumeSnapshotTest) MultiSnapshotRestore(client clientset.Interface, restclient restclientset.Interface, namespace *v1.Namespace) {
	By("Executing multi-snapshot restore scenario")

	// Step 1: Create PVC-1 (rfs / dp2)
	volume := t.Pod.Volumes[0]
	tpvc, pvc1Cleanup := volume.SetupDynamicPersistentVolumeClaim(client, namespace, false)

	// Step 2: Create POD-1 with PVC-1 and write data
	tpod := NewTestPod(client, namespace, t.Pod.Cmd)

	tpod.SetupVolume(tpvc.persistentVolumeClaim, volume.VolumeMount.NameGenerate+"1", volume.VolumeMount.MountPathGenerate+"1", volume.VolumeMount.ReadOnly)

	By("Deploying POD-1")
	tpod.Create()

	By("Waiting for POD-1 to complete write")
	tpod.WaitForSuccess()

	// Step 3: Create SnapshotClass
	tvsc, vscCleanup := CreateVolumeSnapshotClass(restclient, namespace)
	defer vscCleanup()

	// Step 4: Create 3 snapshots for PVC-1
	By("Creating 3 snapshots")
	snapshots := make([]*snapshotv1.VolumeSnapshot, 0, 3)

	for i := 1; i <= 3; i++ {
		snap := tvsc.CreateSnapshot(tpvc.persistentVolumeClaim)
		tvsc.ReadyToUse(snap, false)
		snapshots = append(snapshots, snap)
	}

	// Step 5: Delete POD-1
	By("Deleting POD-1")
	tpod.Cleanup()

	// Step 6: Delete snapshot-1 & snapshot-2
	By("Deleting snapshot-1 and snapshot-2")
	tvsc.DeleteSnapshot(snapshots[0])
	tvsc.DeleteSnapshot(snapshots[1])

	// Step 7: Create PVC-2 from snapshot-3
	restoreSnap := snapshots[2]
	t.RestoredPod.Volumes[0].DataSource = &DataSource{
		Name: restoreSnap.Name,
	}

	rvolume := t.RestoredPod.Volumes[0]
	trpvc, pvc2Cleanup := rvolume.SetupDynamicPersistentVolumeClaim(client, namespace, false)

	// Step 8: Create POD-2 with PVC-2 and read data
	trpod := NewTestPod(client, namespace, t.RestoredPod.Cmd)

	trpod.SetupVolume(
		trpvc.persistentVolumeClaim,
		rvolume.VolumeMount.NameGenerate+"1",
		rvolume.VolumeMount.MountPathGenerate+"1",
		rvolume.VolumeMount.ReadOnly,
	)

	By("Deploying POD-2 from snapshot-3")
	trpod.Create()

	trpod.WaitForRunningSlow()
	trpod.Exec(t.PodCheck.Cmd, t.PodCheck.ExpectedString01)

	// Step 9: Delete POD-2
	By("Deleting POD-2")
	trpod.Cleanup()

	// Step 10: Delete PVC-2
	By("Deleting PVC-2")
	for i := len(pvc2Cleanup) - 1; i >= 0; i-- {
		pvc2Cleanup[i]()
	}

	// Step 11: Delete snapshot-3
	By("Deleting snapshot-3")
	tvsc.DeleteSnapshot(restoreSnap)

	// Step 12: Delete PVC-1
	By("Deleting PVC-1")
	for i := len(pvc1Cleanup) - 1; i >= 0; i-- {
		pvc1Cleanup[i]()
	}
}

func (t *DynamicallyProvisionedVolumeSnapshotTest) SnapshotRestoreAfterSourceDeletion(client clientset.Interface, restclient restclientset.Interface, namespace *v1.Namespace) {
	By("Executing NEGATIVE snapshot restore after source PVC deletion scenario")

	// Step 1: Create PVC-1
	volume := t.Pod.Volumes[0]
	tpvc, pvc1Cleanup := volume.SetupDynamicPersistentVolumeClaim(client, namespace, false)

	// Step 2: Create POD-1 and write data
	tpod := NewTestPod(client, namespace, t.Pod.Cmd)
	tpod.SetupVolume(tpvc.persistentVolumeClaim, volume.VolumeMount.NameGenerate+"1", volume.VolumeMount.MountPathGenerate+"1", volume.VolumeMount.ReadOnly)

	By("Deploying POD-1")
	tpod.Create()
	tpod.WaitForSuccess()

	// Step 3: Create Snapshot
	tvsc, vscCleanup := CreateVolumeSnapshotClass(restclient, namespace)
	defer vscCleanup()

	By("Creating Snapshot-1")
	snapshot := tvsc.CreateSnapshot(tpvc.persistentVolumeClaim)
	tvsc.ReadyToUse(snapshot, false)

	// Step 4: Delete POD-1 and PVC-1
	By("Deleting POD-1")
	tpod.Cleanup()

	By("Deleting PVC-1 (source volume)")
	for i := len(pvc1Cleanup) - 1; i >= 0; i-- {
		pvc1Cleanup[i]()
	}

	// Step 5: Attempt restore (EXPECTED TO FAIL)
	By("Attempting restore from snapshot after source PVC deletion (EXPECTED FAILURE)")

	t.RestoredPod.Volumes[0].DataSource = &DataSource{
		Name: snapshot.Name,
	}

	rvolume := t.RestoredPod.Volumes[0]
	restoredPVC, errList := rvolume.SetupDynamicPersistentVolumeClaim(client, namespace, true)

	// ASSERT FAILURE
	if errList == nil || len(errList) == 0 {
		Fail("Expected restore to FAIL after source PVC deletion, but it SUCCEEDED")
	}

	By("Restore failed as expected — marking test as PASS")

	// Cleanup partial PVC if created
	if restoredPVC != nil {
		_ = client.CoreV1().PersistentVolumeClaims(namespace.Name).
			Delete(context.TODO(), restoredPVC.persistentVolumeClaim.Name, metav1.DeleteOptions{})
	}

	// Step 6: Delete Snapshot
	By("Deleting Snapshot-1")
	tvsc.DeleteSnapshot(snapshot)
}
