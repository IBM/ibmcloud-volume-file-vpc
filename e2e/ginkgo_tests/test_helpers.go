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
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/ibmcloud-volume-file-vpc/e2e/testsuites"
	. "github.com/onsi/ginkgo/v2"
	ginkgotypes "github.com/onsi/ginkgo/v2/types"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	runtime "k8s.io/apimachinery/pkg/runtime"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	serializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	clientset "k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	restclientset "k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	defaultSecret              = ""
	waitForPackageInstallation = 5 * time.Minute
	waitForNodeReadyTimeout    = 15 * time.Minute
	waitForNodeReadyInterval   = 15 * time.Second
)

var (
	testResultFile = os.Getenv("E2E_TEST_RESULT")
	err            error
	fpointer       *os.File
	sc             = os.Getenv("SC")
	sc_retain      = os.Getenv("SC_RETAIN")
)

func rebootWorkersForRHCOS(cs clientset.Interface) {
	if os.Getenv("WORKER_OS") != "RHCOS" {
		log.Print("os is not RHCOS")
		return
	}

	execPath := os.Getenv("EXECPATH")
	if execPath == "" {
		execPath = "."
	}

	clusterName := os.Getenv("CLUSTER_NAME")
	if clusterName == "" {
		panic("cluster name is not set for worker reboot")
	}

	testEnv := os.Getenv("TEST_ENV")
	if testEnv == "" {
		testEnv = os.Getenv("CLUSTER_ENV")
	}
	if testEnv == "" {
		panic("test environment is not set for worker reboot")
	}

	cmd := exec.Command(execPath+"/e2e/iks-cluster", "-e", testEnv, "-c", clusterName, "--worker-reboot")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	if err := cmd.Run(); err != nil {
		panic(fmt.Sprintf("worker reboot command failed: %v", err))
	}

	fmt.Printf("Workers rebooted, waiting up to %s for all nodes to be Ready...\n", waitForNodeReadyTimeout)
	if err := waitForAllNodesReady(cs); err != nil {
		panic(fmt.Sprintf("nodes did not become Ready after reboot: %v", err))
	}
	fmt.Println("All nodes are Ready.")
}

// waitForAllNodesReady polls until every node in the cluster reports Ready=True.
func waitForAllNodesReady(cs clientset.Interface) error {
	return wait.PollImmediate(waitForNodeReadyInterval, waitForNodeReadyTimeout, func() (bool, error) {
		nodes, err := cs.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return false, err
		}
		for _, node := range nodes.Items {
			if !isNodeReady(&node) {
				fmt.Printf("Node %s is not Ready yet\n", node.Name)
				return false, nil
			}
		}
		return true, nil
	})
}

// isNodeReady returns true if the node has a Ready condition with status True.
func isNodeReady(node *corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

func createCustomRfsSC(cs clientset.Interface, name string, params map[string]string) (*storagev1.StorageClass, error) {
	sc := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Provisioner: "vpc.file.csi.ibm.io",
		Parameters:  params,
	}
	reclaimPolicy := v1.PersistentVolumeReclaimDelete
	volumeBindingMode := storagev1.VolumeBindingImmediate
	sc.ReclaimPolicy = &reclaimPolicy
	sc.VolumeBindingMode = &volumeBindingMode

	return cs.StorageV1().StorageClasses().Create(context.TODO(), sc, metav1.CreateOptions{})
}

func deleteCustomRfsSC(cs clientset.Interface, name string) error {
	err := cs.StorageV1().StorageClasses().Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			fmt.Printf("StorageClass %s already deleted\n", name)
			return nil
		}
		return fmt.Errorf("failed to delete StorageClass %q: %w", name, err)
	}
	fmt.Printf("StorageClass %s deleted successfully\n", name)
	return nil
}

func CreateRFSPVC(pvcName, sc, namespace string, throughput int, rfsPvcSize string, cs kubernetes.Interface) {
	_ = cs.CoreV1().PersistentVolumeClaims(namespace).Delete(context.Background(), pvcName, metav1.DeleteOptions{})

	customSCName := sc

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &customSCName,
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(rfsPvcSize),
				},
			},
		},
	}

	_, err := cs.CoreV1().PersistentVolumeClaims(namespace).Create(context.TODO(), pvc, metav1.CreateOptions{})
	if err != nil {
		panic(fmt.Sprintf("Failed to create PVC: %v", err))
	}

	pollInterval := 5 * time.Second
	pollTimeout := 5 * time.Minute
	err = wait.PollImmediate(pollInterval, pollTimeout, func() (bool, error) {
		updatedPVC, err := cs.CoreV1().PersistentVolumeClaims(namespace).Get(context.TODO(), pvcName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return updatedPVC.Status.Phase == corev1.ClaimBound, nil
	})
}

func restClient(group string, version string) (restclientset.Interface, error) {
	config, err := framework.LoadConfig()
	if err != nil {
		Fail(fmt.Sprintf("could not load config: %v", err))
	}
	gv := schema.GroupVersion{Group: group, Version: version}
	config.GroupVersion = &gv
	config.APIPath = "/apis"
	config.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{CodecFactory: serializer.NewCodecFactory(runtime.NewScheme())}
	return restclientset.RESTClientFor(config)
}

// isOpenShiftCluster returns true when the cluster exposes the OpenShift
// security.openshift.io API group (present on ROKS/OCP, absent on IKS).
func isOpenShiftCluster(cs clientset.Interface) bool {
	groups, err := cs.Discovery().ServerGroups()
	if err != nil {
		return false
	}
	for _, g := range groups.Groups {
		if g.Name == "security.openshift.io" {
			return true
		}
	}
	return false
}

// grantSCCToServiceAccount binds the OpenShift "anyuid" SCC ClusterRole to the
// "default" ServiceAccount in the given namespace so that deployment pods can
// run as uid=0 (required because RFS shares are owned root:root mode 755 and
// OpenShift's restricted SCC only grants a non-zero UID by default).
// On non-OpenShift clusters (e.g. IKS) the call is a no-op.
// It returns a cleanup function that removes the ClusterRoleBinding.
func grantSCCToServiceAccount(cs clientset.Interface, namespace string) func() {
	if !isOpenShiftCluster(cs) {
		return func() {} // no-op on IKS
	}

	crbName := fmt.Sprintf("rfs-e2e-anyuid-%s", namespace)
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: crbName,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "system:openshift:scc:anyuid",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "default",
				Namespace: namespace,
			},
		},
	}
	_, err := cs.RbacV1().ClusterRoleBindings().Create(context.TODO(), crb, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		panic(fmt.Sprintf("failed to create ClusterRoleBinding %q: %v", crbName, err))
	}
	return func() {
		if err := cs.RbacV1().ClusterRoleBindings().Delete(context.TODO(), crbName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			fmt.Printf("Warning: failed to delete ClusterRoleBinding %q: %v\n", crbName, err)
		}
	}
}

// =============================================================================
// VAC (VolumeAttributesClass) helpers
//
// The VAC specs (rfs_test.go, dp2_test.go) assume the StorageClass under test
// uses Immediate volume binding: no pod is ever created, so a
// WaitForFirstConsumer class would leave the PVC Pending.
// =============================================================================

const (
	vacPollInterval  = 5 * time.Second
	vacPollTimeout   = 2 * time.Minute
	pvcDeleteTimeout = 5 * time.Minute
	// pvcBoundTimeout must exceed the driver's own mount target wait
	// (40 attempts x 10s) plus share creation, otherwise rfs PVCs time out
	// here before the driver does.
	pvcBoundTimeout = 10 * time.Minute

	vacDriverName = "vpc.file.csi.ibm.io"
	// vacRejectCode is the gRPC code the driver returns when VPC rejects a
	// parameter (e.g. shares_profile_bandwidth_not_allowed). It is a final
	// error, unlike Internal/Unavailable which csi-resizer retries.
	vacRejectCode = "InvalidArgument"
)

// vacCase describes one VolumeAttributesClass scenario. IDs are unique across
// the rfs (rfs_test.go) and dp2 (dp2_test.go) tables.
type vacCase struct {
	id            string            // e.g. "SC1"
	profile       string            // "rfs" or "dp2"
	summary       string            // e.g. "iops+throughput"
	params        map[string]string // VAC parameters
	reject        bool              // true: modification must be rejected with vacRejectCode
	wantIops      int64             // positive cases: expected share iops (0 = not checked)
	wantBandwidth int64             // positive cases: expected share bandwidth in Mbps (0 = not checked)
}

func (c vacCase) title() string {
	outcome := "should apply VAC successfully"
	if c.reject {
		outcome = "should be rejected with " + vacRejectCode + " (VolumeModifyFailed)"
	}
	return fmt.Sprintf("%s: %s PVC + VAC(%s): %s", c.id, strings.ToUpper(c.profile), c.summary, outcome)
}

// resultLine returns the line written to the e2e result file. It must not
// contain the word "failed": e2e.sh greps the result file for it and would
// mark the whole run as failed even though the negative spec passed.
func (c vacCase) resultLine(passed bool) string {
	prefix := fmt.Sprintf("VAC %s: %s PVC + VAC(%s)", c.id, strings.ToUpper(c.profile), c.summary)
	switch {
	case c.reject && passed:
		return fmt.Sprintf("✅ %s → REJECTED as expected (%s)\n", prefix, vacRejectCode)
	case c.reject:
		return fmt.Sprintf("❌ %s → NOT REJECTED as expected\n", prefix)
	case passed:
		return fmt.Sprintf("✅ %s → APPLIED\n", prefix)
	default:
		return fmt.Sprintf("❌ %s → NOT APPLIED\n", prefix)
	}
}

// skipIfVACUnsupported skips the spec when the cluster does not serve
// storage.k8s.io/v1 VolumeAttributesClass (GA from Kubernetes 1.34).
// csi-resizer must also have the VAC feature enabled; that cannot be detected
// from here.
func skipIfVACUnsupported(cs clientset.Interface) {
	const msg = "cluster does not serve storage.k8s.io/v1 VolumeAttributesClass (requires Kubernetes >= 1.34)"
	resources, err := cs.Discovery().ServerResourcesForGroupVersion("storage.k8s.io/v1")
	if err != nil {
		if apierrors.IsNotFound(err) {
			Skip(msg)
		}
		Expect(err).NotTo(HaveOccurred(), "failed to discover storage.k8s.io/v1 resources")
	}
	for _, r := range resources.APIResources {
		if r.Name == "volumeattributesclasses" {
			return
		}
	}
	Skip(msg)
}

// prepareVACNamespace labels the spec namespace as privileged.
func prepareVACNamespace(cs clientset.Interface, namespace string) {
	payload := `{"metadata": {"labels": {"security.openshift.io/scc.podSecurityLabelSync": "false","pod-security.kubernetes.io/enforce": "privileged"}}}`
	_, err := cs.CoreV1().Namespaces().Patch(context.TODO(), namespace, types.StrategicMergePatchType, []byte(payload), metav1.PatchOptions{})
	Expect(err).NotTo(HaveOccurred(), "failed to label namespace %q", namespace)
}

// registerVACResult appends the spec's outcome to the e2e result file once the
// spec finishes. Skipped specs write nothing.
func registerVACResult(c vacCase) {
	f, err := os.OpenFile(testResultFile, os.O_APPEND|os.O_WRONLY, 0644)
	Expect(err).NotTo(HaveOccurred(), "cannot open result file %q (E2E_TEST_RESULT)", testResultFile)
	DeferCleanup(func() {
		defer f.Close()
		report := CurrentSpecReport()
		if report.State.Is(ginkgotypes.SpecStateSkipped) {
			return
		}
		_, _ = f.WriteString(c.resultLine(!report.Failed()))
	})
}

// createVAC creates a VolumeAttributesClass whose name is generated from
// namePrefix (VACs are cluster-scoped, so fixed names would collide across
// runs) and returns the generated name. It is deleted via DeferCleanup.
func createVAC(cs clientset.Interface, namePrefix string, params map[string]string) string {
	vac := &storagev1.VolumeAttributesClass{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: namePrefix + "-",
		},
		DriverName: vacDriverName,
		Parameters: params,
	}
	created, err := cs.StorageV1().VolumeAttributesClasses().Create(context.TODO(), vac, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred(), "failed to create VolumeAttributesClass with prefix %q", namePrefix)
	name := created.Name
	DeferCleanup(func() {
		deleteVAC(cs, name)
	})
	return name
}

// deleteVAC deletes a VolumeAttributesClass, ignoring not-found errors.
func deleteVAC(cs clientset.Interface, name string) {
	err := cs.StorageV1().VolumeAttributesClasses().Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		fmt.Fprintf(GinkgoWriter, "Warning: failed to delete VolumeAttributesClass %q: %v\n", name, err)
	}
}

// createRWXPVC creates a ReadWriteMany PVC using the given StorageClass. On
// cleanup it deletes the PVC and waits until the PVC (and, for a Delete
// reclaim policy, its PV) is gone so the backing VPC share is not leaked.
func createRWXPVC(cs clientset.Interface, namespace, pvcName, scName, size string) *corev1.PersistentVolumeClaim {
	// An empty StorageClassName disables dynamic provisioning and the PVC would
	// stay Pending until pvcBoundTimeout.
	Expect(scName).NotTo(BeEmpty(), "StorageClass name for PVC %q is empty", pvcName)
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &scName,
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(size),
				},
			},
		},
	}
	created, err := cs.CoreV1().PersistentVolumeClaims(namespace).Create(context.TODO(), pvc, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred(), "failed to create PVC %q", pvcName)
	DeferCleanup(func() {
		deletePVCAndWait(cs, namespace, pvcName)
	})
	return created
}

// deletePVCAndWait deletes a PVC and waits for it, and its PV when the reclaim
// policy is Delete, to disappear.
func deletePVCAndWait(cs clientset.Interface, namespace, pvcName string) {
	ctx := context.TODO()
	pvc, err := cs.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return
	}
	Expect(err).NotTo(HaveOccurred(), "failed to get PVC %q for cleanup", pvcName)

	pvName := pvc.Spec.VolumeName
	waitForPVDeletion := false
	if pvName != "" {
		pv, pvErr := cs.CoreV1().PersistentVolumes().Get(ctx, pvName, metav1.GetOptions{})
		if pvErr == nil {
			waitForPVDeletion = pv.Spec.PersistentVolumeReclaimPolicy == corev1.PersistentVolumeReclaimDelete
		}
	}

	err = cs.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, pvcName, metav1.DeleteOptions{})
	Expect(err == nil || apierrors.IsNotFound(err)).To(BeTrue(), "failed to delete PVC %q: %v", pvcName, err)
	err = wait.PollUntilContextTimeout(ctx, vacPollInterval, pvcDeleteTimeout, true, func(ctx context.Context) (bool, error) {
		_, getErr := cs.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcName, metav1.GetOptions{})
		return apierrors.IsNotFound(getErr), nil
	})
	Expect(err).NotTo(HaveOccurred(), "PVC %q was not deleted within %s", pvcName, pvcDeleteTimeout)

	if !waitForPVDeletion {
		return
	}
	err = wait.PollUntilContextTimeout(ctx, vacPollInterval, pvcDeleteTimeout, true, func(ctx context.Context) (bool, error) {
		_, getErr := cs.CoreV1().PersistentVolumes().Get(ctx, pvName, metav1.GetOptions{})
		return apierrors.IsNotFound(getErr), nil
	})
	Expect(err).NotTo(HaveOccurred(), "PV %q (share) was not deleted within %s", pvName, pvcDeleteTimeout)
}

// waitForPVCBound polls until the PVC is Bound. On timeout the failure message
// includes the PVC's events so provisioning errors are visible in the test output.
func waitForPVCBound(cs clientset.Interface, namespace, pvcName string) {
	err := wait.PollUntilContextTimeout(context.TODO(), vacPollInterval, pvcBoundTimeout, true, func(ctx context.Context) (bool, error) {
		pvc, err := cs.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return pvc.Status.Phase == corev1.ClaimBound, nil
	})
	Expect(err).NotTo(HaveOccurred(), "PVC %q did not become Bound. PVC events:\n%s", pvcName, pvcEvents(cs, namespace, pvcName))
}

// pvcEvents returns the events recorded for a PVC as printable text.
func pvcEvents(cs clientset.Interface, namespace, pvcName string) string {
	events, err := cs.CoreV1().Events(namespace).List(context.TODO(), metav1.ListOptions{
		FieldSelector: fields.AndSelectors(
			fields.OneTermEqualSelector("involvedObject.name", pvcName),
			fields.OneTermEqualSelector("involvedObject.kind", "PersistentVolumeClaim"),
		).String(),
	})
	if err != nil {
		return fmt.Sprintf("(could not list events: %v)", err)
	}
	if len(events.Items) == 0 {
		return "(no events)"
	}
	out := ""
	for _, e := range events.Items {
		out += fmt.Sprintf("  %s %s: %s\n", e.Type, e.Reason, e.Message)
	}
	return out
}

// patchPVCWithVAC patches the PVC spec to request the given VAC.
func patchPVCWithVAC(cs clientset.Interface, namespace, pvcName, vacName string) {
	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"volumeAttributesClassName": vacName,
		},
	}
	patchBytes, err := json.Marshal(patch)
	Expect(err).NotTo(HaveOccurred())
	_, err = cs.CoreV1().PersistentVolumeClaims(namespace).Patch(
		context.TODO(), pvcName, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	Expect(err).NotTo(HaveOccurred(), "failed to patch PVC %q with VAC %q", pvcName, vacName)
}

// waitForVACApplied polls until the PVC's currentVolumeAttributesClassName
// matches the expected VAC name (VolumeModifySuccessful path).
func waitForVACApplied(cs clientset.Interface, namespace, pvcName, vacName string) {
	err := wait.PollUntilContextTimeout(context.TODO(), vacPollInterval, vacPollTimeout, true, func(ctx context.Context) (bool, error) {
		pvc, err := cs.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return pvc.Status.CurrentVolumeAttributesClassName != nil &&
			*pvc.Status.CurrentVolumeAttributesClassName == vacName, nil
	})
	Expect(err).NotTo(HaveOccurred(),
		"PVC %q currentVolumeAttributesClassName did not become %q within timeout", pvcName, vacName)
}

// waitForVACRejected polls until a Warning/VolumeModifyFailed event whose
// message carries vacRejectCode appears on the PVC, then asserts the VAC was
// NOT applied. Events with other codes (Internal, Unavailable, auth errors) are
// ignored: csi-resizer retries those, so they are not proof that VPC rejected
// the parameter.
func waitForVACRejected(cs clientset.Interface, namespace, pvcName, vacName string) {
	var seen []string
	err := wait.PollUntilContextTimeout(context.TODO(), vacPollInterval, vacPollTimeout, true, func(ctx context.Context) (bool, error) {
		events, err := cs.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
			FieldSelector: fields.AndSelectors(
				fields.OneTermEqualSelector("involvedObject.name", pvcName),
				fields.OneTermEqualSelector("involvedObject.kind", "PersistentVolumeClaim"),
				fields.OneTermEqualSelector("type", "Warning"),
				fields.OneTermEqualSelector("reason", "VolumeModifyFailed"),
			).String(),
		})
		if err != nil {
			fmt.Fprintf(GinkgoWriter, "listing events for PVC %q failed, will retry: %v\n", pvcName, err)
			return false, nil
		}
		seen = seen[:0]
		for _, e := range events.Items {
			seen = append(seen, e.Message)
			if strings.Contains(e.Message, vacRejectCode) {
				return true, nil
			}
		}
		return false, nil
	})
	Expect(err).NotTo(HaveOccurred(),
		"expected a VolumeModifyFailed Warning event containing %q on PVC %q; VolumeModifyFailed messages seen: %q",
		vacRejectCode, pvcName, seen)

	// Confirm VAC was NOT applied
	pvc, err := cs.CoreV1().PersistentVolumeClaims(namespace).Get(context.TODO(), pvcName, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())
	if pvc.Status.CurrentVolumeAttributesClassName != nil {
		Expect(*pvc.Status.CurrentVolumeAttributesClassName).NotTo(Equal(vacName),
			"PVC %q should NOT have currentVolumeAttributesClassName=%q after VolumeModifyFailed", pvcName, vacName)
	}
}

// shareInfo is the subset of a VPC share the VAC specs compare.
type shareInfo struct {
	State     string
	Iops      int64
	Bandwidth int64 // Mbps
}

// shareIDForPVC returns the VPC share ID backing a bound PVC.
func shareIDForPVC(cs clientset.Interface, namespace, pvcName string) string {
	pvc, err := cs.CoreV1().PersistentVolumeClaims(namespace).Get(context.TODO(), pvcName, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())
	pv, err := cs.CoreV1().PersistentVolumes().Get(context.TODO(), pvc.Spec.VolumeName, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())
	Expect(pv.Spec.CSI).NotTo(BeNil(), "PV %q is not a CSI volume", pv.Name)
	// volumeHandle is <shareID>#<accessPointID>
	return strings.SplitN(pv.Spec.CSI.VolumeHandle, "#", 2)[0]
}

// getShareInfo reads a share with a raw GET /shares/{id}. The beta SDK's Share
// model has no bandwidth field, so the response is decoded generically. It
// returns the HTTP status (0 if no response) alongside any error.
func getShareInfo(shareID string) (shareInfo, int, error) {
	svc := testsuites.VPCService
	builder := core.NewRequestBuilder(core.GET)
	if _, err := builder.ResolveRequestURL(svc.Service.Options.URL, `/shares/{id}`, map[string]string{"id": shareID}); err != nil {
		return shareInfo{}, 0, err
	}
	builder.AddHeader("Accept", "application/json")
	builder.AddQuery("version", *svc.Version)
	builder.AddQuery("generation", "2")
	builder.AddQuery("maturity", "beta")
	request, err := builder.Build()
	if err != nil {
		return shareInfo{}, 0, err
	}

	var body map[string]interface{}
	response, err := svc.Service.Request(request, &body)
	status := 0
	if response != nil {
		status = response.StatusCode
	}
	if err != nil {
		return shareInfo{}, status, err
	}

	var info shareInfo
	info.State, _ = body["lifecycle_state"].(string)
	if v, ok := body["iops"].(float64); ok {
		info.Iops = int64(v)
	}
	if v, ok := body["bandwidth"].(float64); ok {
		info.Bandwidth = int64(v)
	}
	return info, status, nil
}

// waitForShare polls the share until it is stable and ok(info) holds. 5xx and
// network errors are logged and retried; a 4xx (other than 429) aborts.
func waitForShare(shareID, want string, ok func(shareInfo) bool) shareInfo {
	var last shareInfo
	err := wait.PollUntilContextTimeout(context.TODO(), vacPollInterval, vacPollTimeout, true, func(ctx context.Context) (bool, error) {
		info, status, err := getShareInfo(shareID)
		if err != nil {
			if status >= 400 && status < 500 && status != 429 {
				return false, fmt.Errorf("GetShare %q returned HTTP %d: %w", shareID, status, err)
			}
			fmt.Fprintf(GinkgoWriter, "GetShare %q failed (HTTP %d), will retry: %v\n", shareID, status, err)
			return false, nil
		}
		last = info
		return info.State == "stable" && ok(info), nil
	})
	Expect(err).NotTo(HaveOccurred(),
		"share %q: expected %s and lifecycle_state=stable, last seen iops=%d bandwidth=%d state=%q",
		shareID, want, last.Iops, last.Bandwidth, last.State)
	return last
}

// currentShareInfo returns the share once it is stable.
func currentShareInfo(shareID string) shareInfo {
	return waitForShare(shareID, "a stable share", func(shareInfo) bool { return true })
}

// verifyShareIops waits until the share is stable with the expected IOPS.
func verifyShareIops(shareID string, expected int64) {
	waitForShare(shareID, fmt.Sprintf("iops=%d", expected), func(i shareInfo) bool { return i.Iops == expected })
}

// verifyShareBandwidth waits until the share is stable with the expected
// bandwidth in Mbps.
func verifyShareBandwidth(shareID string, expected int64) {
	waitForShare(shareID, fmt.Sprintf("bandwidth=%d", expected), func(i shareInfo) bool { return i.Bandwidth == expected })
}

// runVACSpec runs one vacCase end to end: create VAC and PVC, patch the PVC
// with the VAC, then check either that the change was rejected and the share is
// untouched, or that it was applied on the PVC and on the VPC share.
func runVACSpec(cs clientset.Interface, namespace, scName string, c vacCase) {
	registerVACResult(c)

	idLower := strings.ToLower(c.id)
	vacName := createVAC(cs, fmt.Sprintf("e2e-%s-vac-%s", c.profile, idLower), c.params)
	pvcName := fmt.Sprintf("e2e-%s-pvc-%s", c.profile, idLower)
	createRWXPVC(cs, namespace, pvcName, scName, "10Gi")
	waitForPVCBound(cs, namespace, pvcName)

	shareID := shareIDForPVC(cs, namespace, pvcName)
	before := currentShareInfo(shareID)

	patchPVCWithVAC(cs, namespace, pvcName, vacName)

	if c.reject {
		waitForVACRejected(cs, namespace, pvcName, vacName)
		after := currentShareInfo(shareID)
		Expect(after.Iops).To(Equal(before.Iops), "share %q iops changed although the VAC was rejected", shareID)
		Expect(after.Bandwidth).To(Equal(before.Bandwidth), "share %q bandwidth changed although the VAC was rejected", shareID)
		return
	}

	waitForVACApplied(cs, namespace, pvcName, vacName)
	if c.wantIops > 0 {
		verifyShareIops(shareID, c.wantIops)
	}
	if c.wantBandwidth > 0 {
		verifyShareBandwidth(shareID, c.wantBandwidth)
	}
}

var _ = BeforeSuite(func() {
	log.Print("Successfully construct the service client instance")
	testsuites.InitializeVPCClient()
})
