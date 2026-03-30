package main

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// deleteStuckPods force-deletes pods in Terminating state on the given nodes.
//
// Excluded from deletion:
//   - Pods not yet in Terminating state (DeletionTimestamp == nil)
//   - DaemonSet-owned pods (CNI agents, kube-proxy) — node deletion GCs these automatically
//   - Static/mirror pods (kube-apiserver, etcd) — managed by kubelet, not the API server
//   - Pods with system-cluster-critical or system-node-critical priority (CoreDNS, etc.)
func deleteStuckPods(ctx context.Context, client kubernetes.Interface, nodeNames []string) error {
	if len(nodeNames) == 0 {
		fmt.Println("No node names provided, skipping stuck pod cleanup")
		return nil
	}

	nodeSet := make(map[string]bool, len(nodeNames))
	for _, n := range nodeNames {
		nodeSet[n] = true
	}

	pods, err := client.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing pods: %w", err)
	}

	var stuck []corev1.Pod
	for _, pod := range pods.Items {
		if !nodeSet[pod.Spec.NodeName] {
			continue
		}
		if shouldSkipPod(pod) {
			continue
		}
		stuck = append(stuck, pod)
	}

	if len(stuck) == 0 {
		fmt.Println("No stuck pods found on target nodes")
		return nil
	}

	fmt.Printf("Found %d stuck pod(s) to force-delete\n", len(stuck))

	for _, pod := range stuck {
		fmt.Printf("Force-deleting pod %s/%s on node %s\n", pod.Namespace, pod.Name, pod.Spec.NodeName)

		if len(pod.Finalizers) > 0 {
			patch, _ := json.Marshal(map[string]interface{}{
				"metadata": map[string]interface{}{
					"finalizers": []string{},
				},
			})
			if _, err := client.CoreV1().Pods(pod.Namespace).Patch(
				ctx, pod.Name, types.MergePatchType, patch, metav1.PatchOptions{},
			); err != nil {
				fmt.Printf("Failed to remove finalizers from pod %s/%s: %v\n", pod.Namespace, pod.Name, err)
			}
		}

		gracePeriod := int64(0)
		if err := client.CoreV1().Pods(pod.Namespace).Delete(
			ctx, pod.Name, metav1.DeleteOptions{GracePeriodSeconds: &gracePeriod},
		); err != nil {
			fmt.Printf("Failed to delete pod %s/%s: %v\n", pod.Namespace, pod.Name, err)
		}
	}

	return nil
}

// shouldSkipPod returns true if the pod should NOT be force-deleted.
func shouldSkipPod(pod corev1.Pod) bool {
	// Only target pods already in Terminating state.
	if pod.DeletionTimestamp == nil {
		return true
	}

	// Skip static/mirror pods (kube-apiserver, etcd on self-managed clusters).
	// These are managed by kubelet and cannot be force-deleted via the API.
	if _, isMirror := pod.Annotations["kubernetes.io/config.mirror"]; isMirror {
		return true
	}

	// Skip DaemonSet-owned pods (Cilium, aws-node, kube-proxy, etc.).
	// Kubernetes garbage-collects these automatically when the node object is deleted.
	for _, ref := range pod.OwnerReferences {
		if ref.Kind == "DaemonSet" {
			return true
		}
	}

	// Skip pods with Kubernetes critical priority classes.
	// system-cluster-critical: CoreDNS, metrics-server, etc.
	// system-node-critical:    node-local-dns, other per-node critical services.
	switch pod.Spec.PriorityClassName {
	case "system-cluster-critical", "system-node-critical":
		return true
	}

	return false
}
