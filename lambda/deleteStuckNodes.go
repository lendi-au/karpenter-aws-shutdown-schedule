package main

import (
	"context"
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// deleteStuckNodes force-deletes Kubernetes node objects for nodes belonging to the given
// nodepools that were not cleaned up after NodeClaim deletion. Returns the names of all
// nodes found so the caller can target stuck pods on those nodes.
func deleteStuckNodes(ctx context.Context, client kubernetes.Interface, nodePoolNames []string) ([]string, error) {
	var nodeNames []string

	for _, nodePoolName := range nodePoolNames {
		if nodePoolName == "" {
			continue
		}

		labelSelector := fmt.Sprintf("karpenter.sh/nodepool=%s", nodePoolName)
		nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			return nodeNames, fmt.Errorf("listing nodes for nodepool %s: %w", nodePoolName, err)
		}

		if len(nodes.Items) == 0 {
			fmt.Printf("No stuck nodes found for nodepool %s\n", nodePoolName)
			continue
		}

		fmt.Printf("Found %d stuck node(s) for nodepool %s, force-deleting\n", len(nodes.Items), nodePoolName)

		for _, node := range nodes.Items {
			nodeNames = append(nodeNames, node.Name)
			fmt.Printf("Force-deleting node %s\n", node.Name)

			// Remove finalizers before deletion — same issue as NodeClaims, finalizers
			// block the API server from completing the deletion even with gracePeriod=0.
			if len(node.Finalizers) > 0 {
				fmt.Printf("Removing finalizers from node %s: %v\n", node.Name, node.Finalizers)
				patch, _ := json.Marshal(map[string]interface{}{
					"metadata": map[string]interface{}{
						"finalizers": []string{},
					},
				})
				if _, err := client.CoreV1().Nodes().Patch(
					ctx, node.Name, types.MergePatchType, patch, metav1.PatchOptions{},
				); err != nil {
					fmt.Printf("Failed to remove finalizers from node %s: %v\n", node.Name, err)
				}
			}

			gracePeriod := int64(0)
			if err := client.CoreV1().Nodes().Delete(ctx, node.Name, metav1.DeleteOptions{
				GracePeriodSeconds: &gracePeriod,
			}); err != nil {
				fmt.Printf("Failed to delete node %s: %v\n", node.Name, err)
			}
		}
	}

	return nodeNames, nil
}
