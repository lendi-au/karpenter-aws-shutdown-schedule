package main

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

func deleteSpotNodeclaims(ctx context.Context, dynamicClient dynamic.Interface, nodePoolName string) error {
	nodeClaimGVR := schema.GroupVersionResource{
		Group:    "karpenter.sh",
		Version:  "v1",
		Resource: "nodeclaims",
	}

	labelSelector := fmt.Sprintf("karpenter.sh/nodepool=%s", nodePoolName)
	listOptions := metav1.ListOptions{
		LabelSelector: labelSelector,
	}

	nodeClaimList, err := dynamicClient.Resource(nodeClaimGVR).List(ctx, listOptions)
	if err != nil {
		return fmt.Errorf("failed to list nodeclaims with label selector %s: %v", labelSelector, err)
	}

	if len(nodeClaimList.Items) == 0 {
		fmt.Printf("No nodeclaims found with label selector: %s\n", labelSelector)
		return nil
	}

	fmt.Printf("Found %d nodeclaim(s) with label selector %s\n", len(nodeClaimList.Items), labelSelector)

	for _, nodeclaim := range nodeClaimList.Items {
		name := nodeclaim.GetName()
		fmt.Printf("Deleting nodeclaim: %s\n", name)

		err := dynamicClient.Resource(nodeClaimGVR).Delete(ctx, name, metav1.DeleteOptions{})
		if err != nil {
			fmt.Printf("Failed to delete nodeclaim %s: %v\n", name, err)
			return fmt.Errorf("failed to delete nodeclaim %s: %v", name, err)
		}
		fmt.Printf("Successfully deleted nodeclaim: %s\n", name)
	}

	return nil
}
