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

// DynamicallyProvisionedVolumeSnapshotTest provisions a source PVC and snapshot,
// then restores to three target sizes: same, smaller (negative), and larger.
// RestoreClaimSizes must contain exactly three values: [same, less, more].
type DynamicallyProvisionedVolumeSnapshotTest struct {
	Pod               PodDetails
	RestoredPod       PodDetails
	RestoreClaimSizes [3]string
	PodCheck          *PodExecCheck
}

// RunAllRestoreVariants creates one source PVC, one pod, and one snapshot, then
// exercises all three restore size variants (same / less / more) against it.
func (t *DynamicallyProvisionedVolumeSnapshotTest) RunAllRestoreVariants(client clientset.Interface, restclient restclientset.Interface, namespace *v1.Namespace) {
	By("Executing snapshot lifecycle: single source PVC + snapshot, three restore variants")

	// ── Step 1: source PVC ───────────────────────────────────────────────────
	volume := t.Pod.Volumes[0]
	tpvc, pvcCleanup := volume.SetupDynamicPersistentVolumeClaim(client, namespace, false)
	for i := len(pvcCleanup) - 1; i >= 0; i-- {
		defer pvcCleanup[i]()
	}

	// ── Step 2: source POD — writes data ─────────────────────────────────────
	tpod := NewTestPod(client, namespace, t.Pod.Cmd)
	tpod.SetupVolume(tpvc.persistentVolumeClaim, volume.VolumeMount.NameGenerate+"1", volume.VolumeMount.MountPathGenerate+"1", volume.VolumeMount.ReadOnly)
	By("Deploying POD-1")
	tpod.Create()
	defer tpod.Cleanup()
	By("Checking that POD-1 command exits with no error")
	tpod.WaitForSuccess()

	// ── Step 3: Snapshot ─────────────────────────────────────────────────────
	By("Taking snapshot")
	tvsc, vscCleanup := CreateVolumeSnapshotClass(restclient, namespace)
	defer vscCleanup()
	snapshot := tvsc.CreateSnapshot(tpvc.persistentVolumeClaim)
	defer tvsc.DeleteSnapshot(snapshot)
	tvsc.ReadyToUse(snapshot, false)
	By("Snapshot creation completed")

	rVol := t.RestoredPod.Volumes[0]

	// ── Variant A: restore same size (positive) ──────────────────────────────
	By("RESTORE SAME SIZE: Creating PVC from snapshot")
	sameVol := rVol
	sameVol.ClaimSize = t.RestoreClaimSizes[0]
	sameVol.DataSource = &DataSource{Name: snapshot.Name}
	trpvcSame, rpvcCleanupSame := sameVol.SetupDynamicPersistentVolumeClaim(client, namespace, false)
	for i := len(rpvcCleanupSame) - 1; i >= 0; i-- {
		defer rpvcCleanupSame[i]()
	}
	trpodSame := NewTestPod(client, namespace, t.RestoredPod.Cmd)
	trpodSame.SetupVolume(trpvcSame.persistentVolumeClaim, rVol.VolumeMount.NameGenerate+"1", rVol.VolumeMount.MountPathGenerate+"1", rVol.VolumeMount.ReadOnly)
	By("Deploying POD-2 (same-size restore)")
	trpodSame.Create()
	defer trpodSame.Cleanup()
	trpodSame.WaitForRunningSlow()
	trpodSame.Exec(t.PodCheck.Cmd, t.PodCheck.ExpectedString01)

	// ── Variant B: restore smaller size (negative — expected to stay Pending) ─
	By("RESTORE SIZE LESS: Attempting restore to smaller PVC (should stay Pending)")
	lessVol := rVol
	lessVol.ClaimSize = t.RestoreClaimSizes[1]
	lessVol.DataSource = &DataSource{Name: snapshot.Name}
	restoredPVCLess, _ := lessVol.SetupDynamicPersistentVolumeClaim(client, namespace, true)
	By("PVC stayed Pending as expected — cleaning up")
	if restoredPVCLess != nil {
		_ = client.CoreV1().PersistentVolumeClaims(namespace.Name).
			Delete(context.TODO(), restoredPVCLess.persistentVolumeClaim.Name, metav1.DeleteOptions{})
	}

	// ── Variant C: restore larger size (positive) ────────────────────────────
	By("RESTORE SIZE MORE: Creating PVC from snapshot with larger size")
	moreVol := rVol
	moreVol.ClaimSize = t.RestoreClaimSizes[2]
	moreVol.DataSource = &DataSource{Name: snapshot.Name}
	trpvcMore, rpvcCleanupMore := moreVol.SetupDynamicPersistentVolumeClaim(client, namespace, false)
	for i := len(rpvcCleanupMore) - 1; i >= 0; i-- {
		defer rpvcCleanupMore[i]()
	}
	trpodMore := NewTestPod(client, namespace, t.RestoredPod.Cmd)
	trpodMore.SetupVolume(trpvcMore.persistentVolumeClaim, rVol.VolumeMount.NameGenerate+"1", rVol.VolumeMount.MountPathGenerate+"1", rVol.VolumeMount.ReadOnly)
	By("Deploying POD-3 (larger-size restore)")
	trpodMore.Create()
	defer trpodMore.Cleanup()
	trpodMore.WaitForRunningSlow()
	trpodMore.Exec(t.PodCheck.Cmd, t.PodCheck.ExpectedString01)
}
