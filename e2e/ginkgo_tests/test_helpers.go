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
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/IBM/ibmcloud-volume-file-vpc/e2e/testsuites"
	. "github.com/onsi/ginkgo/v2"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	serializer "k8s.io/apimachinery/pkg/runtime/serializer"
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
)

var (
	testResultFile = os.Getenv("E2E_TEST_RESULT")
	err            error
	fpointer       *os.File
	sc             = os.Getenv("SC")
	sc_retain      = os.Getenv("SC_RETAIN")
)

func rebootWorkersForRHCOS() {
	if os.Getenv("WORKER_OS") != "RHCOS" {
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

var _ = BeforeSuite(func() {
	log.Print("Successfully construct the service client instance")
	testsuites.InitializeVPCClient()
})
