/**
 * Copyright 2026 IBM Corp.
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
	"strings"
	"time"

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
)

var _ = Describe("[ics-e2e] [roundoff] Dynamic Provisioning with allowCapacityRoundoff", func() {
	f := framework.NewDefaultFramework("ics-e2e-roundoff")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged
	var (
		cs clientset.Interface
		ns *v1.Namespace
	)

	BeforeEach(func() {
		cs = f.ClientSet
		ns = f.Namespace
	})

	It("TC-1: with ibmc-vpc-file-ocpvirt-3000-iops sc: should round off requested capacity from 60Gi to 80Gi and reach Bound state", func() {
		payload := `{"metadata": {"labels": {"security.openshift.io/scc.podSecurityLabelSync": "false","pod-security.kubernetes.io/enforce": "privileged"}}}`
		_, labelerr := cs.CoreV1().Namespaces().Patch(context.TODO(), ns.Name, types.StrategicMergePatchType, []byte(payload), metav1.PatchOptions{})
		if labelerr != nil {
			panic(labelerr)
		}

		scName := "ibmc-vpc-file-ocpvirt-3000-iops"
		requestedCapacity := "60Gi"
		expectedCapacity := "80Gi"
		pvcName := "ics-vol-roundoff-3000-60gi"

		fpointer, err = os.OpenFile(testResultFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}

		var boundPVC *v1.PersistentVolumeClaim
		DeferCleanup(func() {
			if fpointer != nil {
				provisionedCap := "unknown"
				if boundPVC != nil {
					cap := boundPVC.Status.Capacity[v1.ResourceStorage]
					provisionedCap = cap.String()
				}
				if CurrentSpecReport().Failed() {
					fpointer.WriteString(fmt.Sprintf("❌ CAPACITY ROUNDOFF: 3000 IOPS (60Gi -> 80Gi) WITH %s STORAGE CLASS (provisioned: %s)\n", scName, provisionedCap))
				} else {
					fpointer.WriteString(fmt.Sprintf("✅ CAPACITY ROUNDOFF: 3000 IOPS (60Gi -> 80Gi) WITH %s STORAGE CLASS (provisioned: %s)\n", scName, provisionedCap))
				}
				fpointer.Close()
			}
		})

		pvc := &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: ns.Name,
				Labels:    map[string]string{"app": "ics-e2e-tester"},
			},
			Spec: v1.PersistentVolumeClaimSpec{
				StorageClassName: &scName,
				AccessModes: []v1.PersistentVolumeAccessMode{
					v1.ReadWriteMany,
				},
				Resources: v1.VolumeResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceName(v1.ResourceStorage): resource.MustParse(requestedCapacity),
					},
				},
			},
		}

		By(fmt.Sprintf("Creating PVC %s with storageclass %s and requested capacity %s", pvcName, scName, requestedCapacity))
		createdPVC, err := cs.CoreV1().PersistentVolumeClaims(ns.Name).Create(context.TODO(), pvc, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		DeferCleanup(func() {
			By(fmt.Sprintf("Cleaning up PVC %s", pvcName))
			_ = cs.CoreV1().PersistentVolumeClaims(ns.Name).Delete(context.TODO(), pvcName, metav1.DeleteOptions{})
		})

		By("Waiting for PVC to reach Bound state")
		err = wait.PollImmediate(5*time.Second, 10*time.Minute, func() (bool, error) {
			boundPVC, err = cs.CoreV1().PersistentVolumeClaims(ns.Name).Get(context.TODO(), createdPVC.Name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			return boundPVC.Status.Phase == v1.ClaimBound, nil
		})
		Expect(err).NotTo(HaveOccurred(), "PVC failed to reach Bound state")

		By("Verifying the provisioned volume/PVC capacity is rounded up to 80Gi")
		actualCap := boundPVC.Status.Capacity[v1.ResourceStorage]
		expectedCapQuantity := resource.MustParse(expectedCapacity)
		Expect(actualCap.Cmp(expectedCapQuantity)).To(Equal(0),
			fmt.Sprintf("Expected capacity to be rounded to %s, but got %s", expectedCapacity, actualCap.String()))

		By("Verifying the bound PV capacity is 80Gi")
		pv, err := cs.CoreV1().PersistentVolumes().Get(context.TODO(), boundPVC.Spec.VolumeName, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		pvCap := pv.Spec.Capacity[v1.ResourceStorage]
		Expect(pvCap.Cmp(expectedCapQuantity)).To(Equal(0),
			fmt.Sprintf("Expected PV capacity to be %s, but got %s", expectedCapacity, pvCap.String()))
	})

	It("TC-2: with ibmc-vpc-file-ocpvirt-1000-iops sc: should round off requested capacity from 5Gi to 10Gi and reach Bound state", func() {
		payload := `{"metadata": {"labels": {"security.openshift.io/scc.podSecurityLabelSync": "false","pod-security.kubernetes.io/enforce": "privileged"}}}`
		_, labelerr := cs.CoreV1().Namespaces().Patch(context.TODO(), ns.Name, types.StrategicMergePatchType, []byte(payload), metav1.PatchOptions{})
		if labelerr != nil {
			panic(labelerr)
		}

		scName := "ibmc-vpc-file-ocpvirt-1000-iops"
		requestedCapacity := "5Gi"
		expectedCapacity := "10Gi"
		pvcName := "ics-vol-roundoff-1000-5gi"

		fpointer, err = os.OpenFile(testResultFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}

		var boundPVC *v1.PersistentVolumeClaim
		DeferCleanup(func() {
			if fpointer != nil {
				provisionedCap := "unknown"
				if boundPVC != nil {
					cap := boundPVC.Status.Capacity[v1.ResourceStorage]
					provisionedCap = cap.String()
				}
				if CurrentSpecReport().Failed() {
					fpointer.WriteString(fmt.Sprintf("❌ CAPACITY ROUNDOFF: 1000 IOPS (5Gi -> 10Gi) WITH %s STORAGE CLASS (provisioned: %s)\n", scName, provisionedCap))
				} else {
					fpointer.WriteString(fmt.Sprintf("✅ CAPACITY ROUNDOFF: 1000 IOPS (5Gi -> 10Gi) WITH %s STORAGE CLASS (provisioned: %s)\n", scName, provisionedCap))
				}
				fpointer.Close()
			}
		})

		pvc := &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: ns.Name,
				Labels:    map[string]string{"app": "ics-e2e-tester"},
			},
			Spec: v1.PersistentVolumeClaimSpec{
				StorageClassName: &scName,
				AccessModes: []v1.PersistentVolumeAccessMode{
					v1.ReadWriteMany,
				},
				Resources: v1.VolumeResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceName(v1.ResourceStorage): resource.MustParse(requestedCapacity),
					},
				},
			},
		}

		By(fmt.Sprintf("Creating PVC %s with storageclass %s and requested capacity %s", pvcName, scName, requestedCapacity))
		createdPVC, err := cs.CoreV1().PersistentVolumeClaims(ns.Name).Create(context.TODO(), pvc, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		DeferCleanup(func() {
			By(fmt.Sprintf("Cleaning up PVC %s", pvcName))
			_ = cs.CoreV1().PersistentVolumeClaims(ns.Name).Delete(context.TODO(), pvcName, metav1.DeleteOptions{})
		})

		By("Waiting for PVC to reach Bound state")
		err = wait.PollImmediate(5*time.Second, 10*time.Minute, func() (bool, error) {
			boundPVC, err = cs.CoreV1().PersistentVolumeClaims(ns.Name).Get(context.TODO(), createdPVC.Name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			return boundPVC.Status.Phase == v1.ClaimBound, nil
		})
		Expect(err).NotTo(HaveOccurred(), "PVC failed to reach Bound state")

		By("Verifying the provisioned volume/PVC capacity is rounded up to 10Gi")
		actualCap := boundPVC.Status.Capacity[v1.ResourceStorage]
		expectedCapQuantity := resource.MustParse(expectedCapacity)
		Expect(actualCap.Cmp(expectedCapQuantity)).To(Equal(0),
			fmt.Sprintf("Expected capacity to be rounded to %s, but got %s", expectedCapacity, actualCap.String()))

		By("Verifying the bound PV capacity is 10Gi")
		pv, err := cs.CoreV1().PersistentVolumes().Get(context.TODO(), boundPVC.Spec.VolumeName, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		pvCap := pv.Spec.Capacity[v1.ResourceStorage]
		Expect(pvCap.Cmp(expectedCapQuantity)).To(Equal(0),
			fmt.Sprintf("Expected PV capacity to be %s, but got %s", expectedCapacity, pvCap.String()))
	})

	It("TC-3: with ibmc-vpc-file-ocpvirt-3000-iops sc: should keep requested capacity at 100Gi without roundoff and reach Bound state", func() {
		payload := `{"metadata": {"labels": {"security.openshift.io/scc.podSecurityLabelSync": "false","pod-security.kubernetes.io/enforce": "privileged"}}}`
		_, labelerr := cs.CoreV1().Namespaces().Patch(context.TODO(), ns.Name, types.StrategicMergePatchType, []byte(payload), metav1.PatchOptions{})
		if labelerr != nil {
			panic(labelerr)
		}

		scName := "ibmc-vpc-file-ocpvirt-3000-iops"
		requestedCapacity := "100Gi"
		expectedCapacity := "100Gi"
		pvcName := "ics-vol-roundoff-3000-100gi"

		fpointer, err = os.OpenFile(testResultFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}

		var boundPVC *v1.PersistentVolumeClaim
		DeferCleanup(func() {
			if fpointer != nil {
				provisionedCap := "unknown"
				if boundPVC != nil {
					cap := boundPVC.Status.Capacity[v1.ResourceStorage]
					provisionedCap = cap.String()
				}
				if CurrentSpecReport().Failed() {
					fpointer.WriteString(fmt.Sprintf("❌ CAPACITY ROUNDOFF: 3000 IOPS (100Gi ABOVE MINIMUM) WITH %s STORAGE CLASS (provisioned: %s)\n", scName, provisionedCap))
				} else {
					fpointer.WriteString(fmt.Sprintf("✅ CAPACITY ROUNDOFF: 3000 IOPS (100Gi ABOVE MINIMUM) WITH %s STORAGE CLASS (provisioned: %s)\n", scName, provisionedCap))
				}
				fpointer.Close()
			}
		})

		pvc := &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: ns.Name,
				Labels:    map[string]string{"app": "ics-e2e-tester"},
			},
			Spec: v1.PersistentVolumeClaimSpec{
				StorageClassName: &scName,
				AccessModes: []v1.PersistentVolumeAccessMode{
					v1.ReadWriteMany,
				},
				Resources: v1.VolumeResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceName(v1.ResourceStorage): resource.MustParse(requestedCapacity),
					},
				},
			},
		}

		By(fmt.Sprintf("Creating PVC %s with storageclass %s and requested capacity %s", pvcName, scName, requestedCapacity))
		createdPVC, err := cs.CoreV1().PersistentVolumeClaims(ns.Name).Create(context.TODO(), pvc, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		DeferCleanup(func() {
			By(fmt.Sprintf("Cleaning up PVC %s", pvcName))
			_ = cs.CoreV1().PersistentVolumeClaims(ns.Name).Delete(context.TODO(), pvcName, metav1.DeleteOptions{})
		})

		By("Waiting for PVC to reach Bound state")
		err = wait.PollImmediate(5*time.Second, 10*time.Minute, func() (bool, error) {
			boundPVC, err = cs.CoreV1().PersistentVolumeClaims(ns.Name).Get(context.TODO(), createdPVC.Name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			return boundPVC.Status.Phase == v1.ClaimBound, nil
		})
		Expect(err).NotTo(HaveOccurred(), "PVC failed to reach Bound state")

		By("Verifying the provisioned volume/PVC capacity remains 100Gi")
		actualCap := boundPVC.Status.Capacity[v1.ResourceStorage]
		expectedCapQuantity := resource.MustParse(expectedCapacity)
		Expect(actualCap.Cmp(expectedCapQuantity)).To(Equal(0),
			fmt.Sprintf("Expected capacity to remain %s, but got %s", expectedCapacity, actualCap.String()))

		By("Verifying the bound PV capacity is 100Gi")
		pv, err := cs.CoreV1().PersistentVolumes().Get(context.TODO(), boundPVC.Spec.VolumeName, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		pvCap := pv.Spec.Capacity[v1.ResourceStorage]
		Expect(pvCap.Cmp(expectedCapQuantity)).To(Equal(0),
			fmt.Sprintf("Expected PV capacity to be %s, but got %s", expectedCapacity, pvCap.String()))
	})

	It("TC-4: with ibmc-vpc-file-ocpvirt-1000-iops sc: should fail provisioning when requested capacity (97000Gi) is outside maximum supported range", func() {
		payload := `{"metadata": {"labels": {"security.openshift.io/scc.podSecurityLabelSync": "false","pod-security.kubernetes.io/enforce": "privileged"}}}`
		_, labelerr := cs.CoreV1().Namespaces().Patch(context.TODO(), ns.Name, types.StrategicMergePatchType, []byte(payload), metav1.PatchOptions{})
		if labelerr != nil {
			panic(labelerr)
		}

		scName := "ibmc-vpc-file-ocpvirt-1000-iops"
		requestedCapacity := "97000Gi"
		pvcName := "ics-vol-roundoff-1000-97000gi"

		fpointer, err = os.OpenFile(testResultFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}

		DeferCleanup(func() {
			if fpointer != nil {
				if CurrentSpecReport().Failed() {
					fpointer.WriteString(fmt.Sprintf("❌ CAPACITY ROUNDOFF: 1000 IOPS (97000Gi OUTSIDE RANGE MUST FAIL) WITH %s STORAGE CLASS\n", scName))
				} else {
					fpointer.WriteString(fmt.Sprintf("✅ CAPACITY ROUNDOFF: 1000 IOPS (97000Gi OUTSIDE RANGE MUST FAIL) WITH %s STORAGE CLASS\n", scName))
				}
				fpointer.Close()
			}
		})

		pvc := &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: ns.Name,
				Labels:    map[string]string{"app": "ics-e2e-tester"},
			},
			Spec: v1.PersistentVolumeClaimSpec{
				StorageClassName: &scName,
				AccessModes: []v1.PersistentVolumeAccessMode{
					v1.ReadWriteMany,
				},
				Resources: v1.VolumeResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceName(v1.ResourceStorage): resource.MustParse(requestedCapacity),
					},
				},
			},
		}

		By(fmt.Sprintf("Creating PVC %s with storageclass %s and unsupported capacity %s", pvcName, scName, requestedCapacity))
		createdPVC, err := cs.CoreV1().PersistentVolumeClaims(ns.Name).Create(context.TODO(), pvc, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		DeferCleanup(func() {
			By(fmt.Sprintf("Cleaning up PVC %s", pvcName))
			_ = cs.CoreV1().PersistentVolumeClaims(ns.Name).Delete(context.TODO(), pvcName, metav1.DeleteOptions{})
		})

		By("Verifying that PVC does not reach Bound state and stays in Pending/fails")
		var observedPVC *v1.PersistentVolumeClaim
		_ = wait.PollImmediate(5*time.Second, 1*time.Minute, func() (bool, error) {
			observedPVC, _ = cs.CoreV1().PersistentVolumeClaims(ns.Name).Get(context.TODO(), createdPVC.Name, metav1.GetOptions{})
			if observedPVC != nil && observedPVC.Status.Phase == v1.ClaimBound {
				return true, nil
			}
			return false, nil
		})
		if observedPVC != nil {
			Expect(observedPVC.Status.Phase).NotTo(Equal(v1.ClaimBound), "PVC should not reach Bound state for capacity outside maximum range")
		}

		By("Verifying the provisioning failure event for the PVC")
		var matchedEvent bool
		_ = wait.PollImmediate(5*time.Second, 2*time.Minute, func() (bool, error) {
			events, err := cs.CoreV1().Events(ns.Name).List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				return false, nil
			}
			for _, ev := range events.Items {
				if ev.InvolvedObject.Name == pvcName && ev.InvolvedObject.Kind == "PersistentVolumeClaim" {
					if ev.Reason == "ProvisioningFailed" || strings.Contains(ev.Message, "shares_profile_capacity_iops_invalid") || strings.Contains(ev.Message, "invalid PVC size") || strings.Contains(ev.Message, "infeasible error") {
						matchedEvent = true
						return true, nil
					}
				}
			}
			return false, nil
		})
		Expect(matchedEvent).To(BeTrue(), "Expected provisioning failure event containing capacity/IOPS error for PVC")
	})
})
