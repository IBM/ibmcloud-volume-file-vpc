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
	"bytes"
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

const (
	// CSIDriverNamespace is the namespace where CSI driver pods run
	CSIDriverNamespace = "kube-system"

	// CSIDriverLabelSelector is the label selector for CSI driver pods
	CSIDriverLabelSelector = "app=ibm-vpc-file-csi-node"

	// StunnelContainerName is the name of the denali-stunnel sidecar container
	StunnelContainerName = "denali-stunnel"

	// StunnelServicesDir is the directory where stunnel service configs are stored
	StunnelServicesDir = "/etc/stunnel/services"

	// EITCleanupTimeout is the timeout for EIT tunnel cleanup
	EITCleanupTimeout = 2 * time.Minute

	// EITPollInterval is the interval for polling EIT operations
	EITPollInterval = 5 * time.Second
)

// GetCSIDriverNamespace returns the CSI driver namespace from env or default
func GetCSIDriverNamespace() string {
	if ns := os.Getenv("CSI_DRIVER_NAMESPACE"); ns != "" {
		return ns
	}
	return CSIDriverNamespace
}

// GetCSIDriverPod gets the CSI driver node pod running on the specified node
func GetCSIDriverPod(client clientset.Interface, nodeName string) (*v1.Pod, error) {
	namespace := GetCSIDriverNamespace()

	pods, err := client.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: CSIDriverLabelSelector,
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", nodeName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list CSI driver pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no CSI driver pod found on node %s", nodeName)
	}

	return &pods.Items[0], nil
}

// GetPodNode returns the node name where the pod is running
func GetPodNode(client clientset.Interface, namespace, podName string) (string, error) {
	pod, err := client.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get pod: %w", err)
	}

	if pod.Spec.NodeName == "" {
		return "", fmt.Errorf("pod %s is not scheduled to any node", podName)
	}

	return pod.Spec.NodeName, nil
}

// ExecCommandInPod executes a command in a pod and returns the output
func ExecCommandInPod(config *restclient.Config, client clientset.Interface, namespace, podName, containerName string, command []string) (string, string, error) {
	req := client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")

	req.VersionedParams(&v1.PodExecOptions{
		Container: containerName,
		Command:   command,
		Stdout:    true,
		Stderr:    true,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return "", "", fmt.Errorf("failed to create executor: %w", err)
	}

	var stdout, stderr bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	return stdout.String(), stderr.String(), err
}

// extractShareID extracts the share ID from volumeID (format: shareID#targetID)
func extractShareID(volumeID string) string {
	// Split by # separator
	parts := strings.Split(volumeID, "#")
	if len(parts) > 0 {
		return parts[0]
	}
	// Fallback: try deprecated : separator
	parts = strings.Split(volumeID, ":")
	if len(parts) > 0 {
		return parts[0]
	}
	return volumeID
}

// VerifyStunnelTunnel verifies that a stunnel tunnel exists for the given volume
// Returns the allocated port number
func VerifyStunnelTunnel(config *restclient.Config, client clientset.Interface, testPodNamespace, testPodName, volumeID string) (int, error) {
	// Get the node where test pod is running
	nodeName, err := GetPodNode(client, testPodNamespace, testPodName)
	if err != nil {
		return 0, fmt.Errorf("failed to get pod node: %w", err)
	}

	// Get CSI driver pod on that node
	csiPod, err := GetCSIDriverPod(client, nodeName)
	if err != nil {
		return 0, fmt.Errorf("failed to get CSI driver pod: %w", err)
	}

	// Extract share ID from volumeID (format: shareID#targetID)
	// Config file is named after shareID only
	shareID := extractShareID(volumeID)

	// Check if stunnel config file exists
	configPath := fmt.Sprintf("%s/%s.conf", StunnelServicesDir, shareID)
	command := []string{"test", "-f", configPath}

	_, _, err = ExecCommandInPod(config, client, csiPod.Namespace, csiPod.Name, StunnelContainerName, command)
	if err != nil {
		return 0, fmt.Errorf("stunnel config file not found for volume %s (shareID: %s): %w", volumeID, shareID, err)
	}

	// Read the config file to get the port
	command = []string{"cat", configPath}
	stdout, _, err := ExecCommandInPod(config, client, csiPod.Namespace, csiPod.Name, StunnelContainerName, command)
	if err != nil {
		return 0, fmt.Errorf("failed to read stunnel config: %w", err)
	}

	// Extract port from config (accept = 127.0.0.1:PORT)
	port, err := extractPortFromStunnelConfig(stdout)
	if err != nil {
		return 0, fmt.Errorf("failed to extract port from config: %w", err)
	}

	return port, nil
}

// extractPortFromStunnelConfig extracts the port number from stunnel config content
func extractPortFromStunnelConfig(config string) (int, error) {
	// Look for "accept = 127.0.0.1:PORT" or "accept = 127.0.0.1 : PORT"
	re := regexp.MustCompile(`accept\s*=\s*127\.0\.0\.1\s*:\s*(\d+)`)
	matches := re.FindStringSubmatch(config)
	if len(matches) < 2 {
		return 0, fmt.Errorf("port not found in config")
	}

	port, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, fmt.Errorf("invalid port number: %w", err)
	}

	return port, nil
}

// VerifyMountUsesStunnel verifies that the mount uses the stunnel tunnel
func VerifyMountUsesStunnel(config *restclient.Config, client clientset.Interface, namespace, podName string, expectedPort int) error {
	// Read /proc/mounts in the test pod
	command := []string{"cat", "/proc/mounts"}
	stdout, _, err := ExecCommandInPod(config, client, namespace, podName, "", command)
	if err != nil {
		return fmt.Errorf("failed to read /proc/mounts: %w", err)
	}

	// Look for NFS4 mount using localhost and expected port
	expectedPortOption := fmt.Sprintf("port=%d", expectedPort)

	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		if strings.Contains(line, "nfs4") && strings.Contains(line, "127.0.0.1") {
			// Found NFS4 mount using localhost
			if !strings.Contains(line, expectedPortOption) {
				return fmt.Errorf("mount uses localhost but wrong port (expected port=%d)", expectedPort)
			}
			return nil
		}
	}

	return fmt.Errorf("no NFS4 mount found using stunnel tunnel (127.0.0.1:%d)", expectedPort)
}

// WaitForTunnelCleanup waits for the tunnel config to be removed
func WaitForTunnelCleanup(config *restclient.Config, client clientset.Interface, nodeName, volumeID string) error {
	// Extract share ID from volumeID (format: shareID#targetID)
	shareID := extractShareID(volumeID)

	return wait.PollImmediate(EITPollInterval, EITCleanupTimeout, func() (bool, error) {
		// Get CSI driver pod
		csiPod, err := GetCSIDriverPod(client, nodeName)
		if err != nil {
			// Pod might be restarting, continue waiting
			return false, nil
		}

		// Check if config file exists (named after shareID only)
		configPath := fmt.Sprintf("%s/%s.conf", StunnelServicesDir, shareID)
		command := []string{"test", "-f", configPath}

		_, _, err = ExecCommandInPod(config, client, csiPod.Namespace, csiPod.Name, StunnelContainerName, command)
		if err != nil {
			// Config file removed
			return true, nil
		}

		// Config still exists, keep waiting
		return false, nil
	})
}

// GetVolumeIDFromPVC extracts the volume ID from PVC
func GetVolumeIDFromPVC(client clientset.Interface, namespace, pvcName string) (string, error) {
	pvc, err := client.CoreV1().PersistentVolumeClaims(namespace).Get(context.TODO(), pvcName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get PVC: %w", err)
	}

	if pvc.Spec.VolumeName == "" {
		return "", fmt.Errorf("PVC not bound to any volume")
	}

	pv, err := client.CoreV1().PersistentVolumes().Get(context.TODO(), pvc.Spec.VolumeName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get PV: %w", err)
	}

	if pv.Spec.CSI == nil {
		return "", fmt.Errorf("PV is not a CSI volume")
	}

	return pv.Spec.CSI.VolumeHandle, nil
}

// VerifyStunnelCleanState verifies that stunnel is in clean state after all volumes unmounted
// - At least one stunnel process (main daemon)
// - No config files in /etc/stunnel/services/
func VerifyStunnelCleanState(config *restclient.Config, client clientset.Interface, nodeName string) error {
	// Get CSI driver pod
	csiPod, err := GetCSIDriverPod(client, nodeName)
	if err != nil {
		return fmt.Errorf("failed to get CSI driver pod: %w", err)
	}

	// Check stunnel process count using /proc (ps/pgrep not available in container)
	// Count processes by checking /proc/*/comm for "stunnel"
	command := []string{"sh", "-c", "for pid in /proc/[0-9]*; do [ -f $pid/comm ] && grep -q '^stunnel$' $pid/comm 2>/dev/null && echo 1; done | wc -l"}
	stdout, _, err := ExecCommandInPod(config, client, csiPod.Namespace, csiPod.Name, StunnelContainerName, command)
	if err != nil {
		return fmt.Errorf("failed to count stunnel processes: %w", err)
	}
	processCount := strings.TrimSpace(stdout)

	// Convert to int for comparison
	count := 0
	fmt.Sscanf(processCount, "%d", &count)

	// Should have at least 1 process (main daemon)
	if count == 0 {
		return fmt.Errorf("expected at least 1 stunnel process (main daemon), found %d", count)
	}

	// Check no config files exist (most important check for clean state)
	command = []string{"sh", "-c", fmt.Sprintf("ls -1 %s/*.conf 2>/dev/null | wc -l", StunnelServicesDir)}
	stdout, _, err = ExecCommandInPod(config, client, csiPod.Namespace, csiPod.Name, StunnelContainerName, command)
	if err != nil {
		return fmt.Errorf("failed to count config files: %w", err)
	}
	configCount := strings.TrimSpace(stdout)
	if configCount != "0" {
		return fmt.Errorf("expected 0 config files, found %s", configCount)
	}

	return nil
}

// GetStunnelProcessCount returns the number of stunnel processes
func GetStunnelProcessCount(config *restclient.Config, client clientset.Interface, nodeName string) (int, error) {
	csiPod, err := GetCSIDriverPod(client, nodeName)
	if err != nil {
		return 0, fmt.Errorf("failed to get CSI driver pod: %w", err)
	}

	// Use /proc to count processes (ps/pgrep not available)
	command := []string{"sh", "-c", "for pid in /proc/[0-9]*; do [ -f $pid/comm ] && grep -q '^stunnel$' $pid/comm 2>/dev/null && echo 1; done | wc -l"}
	stdout, _, err := ExecCommandInPod(config, client, csiPod.Namespace, csiPod.Name, StunnelContainerName, command)
	if err != nil {
		return 0, fmt.Errorf("failed to count stunnel processes: %w", err)
	}

	count := 0
	fmt.Sscanf(strings.TrimSpace(stdout), "%d", &count)
	return count, nil
}

// GetStunnelListenerCount returns the number of stunnel listeners
func GetStunnelListenerCount(config *restclient.Config, client clientset.Interface, nodeName string) (int, error) {
	csiPod, err := GetCSIDriverPod(client, nodeName)
	if err != nil {
		return 0, fmt.Errorf("failed to get CSI driver pod: %w", err)
	}

	// Use /proc/net/tcp to count listeners (ss not available)
	command := []string{"sh", "-c", "grep -c ' 0A ' /proc/net/tcp /proc/net/tcp6 2>/dev/null || echo 0"}
	stdout, _, err := ExecCommandInPod(config, client, csiPod.Namespace, csiPod.Name, StunnelContainerName, command)
	if err != nil {
		return 0, fmt.Errorf("failed to count listeners: %w", err)
	}

	count := 0
	fmt.Sscanf(strings.TrimSpace(stdout), "%d", &count)
	return count, nil
}

// GetStunnelConfigFileCount returns the number of config files in services directory
func GetStunnelConfigFileCount(config *restclient.Config, client clientset.Interface, nodeName string) (int, error) {
	csiPod, err := GetCSIDriverPod(client, nodeName)
	if err != nil {
		return 0, fmt.Errorf("failed to get CSI driver pod: %w", err)
	}

	command := []string{"sh", "-c", fmt.Sprintf("ls -1 %s/*.conf 2>/dev/null | wc -l", StunnelServicesDir)}
	stdout, _, err := ExecCommandInPod(config, client, csiPod.Namespace, csiPod.Name, StunnelContainerName, command)
	if err != nil {
		return 0, fmt.Errorf("failed to count config files: %w", err)
	}

	count := 0
	fmt.Sscanf(strings.TrimSpace(stdout), "%d", &count)
	return count, nil
}

// Made with Bob
