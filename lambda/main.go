package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/lambda"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ActionEvent struct {
	Action string `json:"Action"`
}

func handler(ctx context.Context, request ActionEvent) error {
	fmt.Printf("ctx: %v", ctx)
	fmt.Printf("Requested action: %s", request.Action)

	nodePoolsStr := os.Getenv("KARPENTER_NODEPOOLS")
	if nodePoolsStr == "" {
		return fmt.Errorf("KARPENTER_NODEPOOLS environment variable not set")
	}

	// Parse comma-separated list of nodepools
	nodePoolNames := strings.Split(nodePoolsStr, ",")
	for i := range nodePoolNames {
		nodePoolNames[i] = strings.TrimSpace(nodePoolNames[i])
	}

	fmt.Printf("Processing nodepools: %v\n", nodePoolNames)

	dynamicClient, err := newDynamicClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %v", err)
	}

	// common const setup
	nodePoolGVR := schema.GroupVersionResource{
		Group:    "karpenter.sh",
		Version:  "v1",
		Resource: "nodepools",
	}

	// Process each nodepool
	for _, nodePoolName := range nodePoolNames {
		if nodePoolName == "" {
			continue
		}

		fmt.Printf("\n=== Processing nodepool: %s ===\n", nodePoolName)

		np, err := dynamicClient.Resource(nodePoolGVR).Get(ctx, nodePoolName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get nodepool %s: %v", nodePoolName, err)
		}

		switch request.Action {
		case "shutdown":
			fmt.Printf("Simulating scaling down nodepool %s\n", nodePoolName)
			err = unstructured.SetNestedField(np.Object, "0", "spec", "limits", "cpu")
			if err != nil {
				return fmt.Errorf("failed to set cpu limit for nodepool %s: %v", nodePoolName, err)
			}

			_, err = dynamicClient.Resource(nodePoolGVR).Update(ctx, np, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("failed to update nodepool %s: %v", nodePoolName, err)
			}

			fmt.Printf("Successfully updated nodepool %s to set cpu limit to 0\n", nodePoolName)

			// Delete all nodeclaims with label karpenter.sh/nodepool=<nodepool-name>
			fmt.Printf("Deleting nodeclaims for nodepool %s...\n", nodePoolName)
			if err := deleteSpotNodeclaims(ctx, dynamicClient, nodePoolName); err != nil {
				return fmt.Errorf("failed to delete nodeclaims for nodepool %s: %v", nodePoolName, err)
			}
		case "startup":
			fmt.Printf("Simulating scale up of nodepool %s\n", nodePoolName)
			cpuLimit := os.Getenv("KARPENTER_NODEPOOL_LIMITS_CPU")
			if cpuLimit == "" {
				fmt.Printf("Environment variable KARPENTER_NODEPOOL_LIMITS_CPU not set - using default 1000\n")
				cpuLimit = "1000"
			}
			err = unstructured.SetNestedField(np.Object, cpuLimit, "spec", "limits", "cpu")
			if err != nil {
				return fmt.Errorf("failed to set cpu limit for nodepool %s: %v", nodePoolName, err)
			}

			_, err = dynamicClient.Resource(nodePoolGVR).Update(ctx, np, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("failed to update nodepool %s: %v", nodePoolName, err)
			}
			fmt.Printf("Successfully updated nodepool %s to restore cpu limit to %s\n", nodePoolName, cpuLimit)
		}
	}

	// EC2 interaction - pass all nodepool names
	if err := ShutdownEC2Instances(ctx, nodePoolNames); err != nil {
		return err
	}

	return nil
}

func main() {
	lambda.Start(handler)
}
