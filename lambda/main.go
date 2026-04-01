package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/lendi-au/karpenter-aws-shutdown-schedule/pkg/utils"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ActionEvent struct {
	Action string `json:"Action"`
}

func waitSeconds(envKey string, defaultVal int) {
	secs, err := strconv.Atoi(utils.GetenvDefault(envKey, strconv.Itoa(defaultVal)))
	if err != nil {
		secs = defaultVal
	}
	fmt.Printf("Waiting %ds (%s)...\n", secs, envKey)
	time.Sleep(time.Duration(secs) * time.Second)
}

func handler(ctx context.Context, request ActionEvent) error {
	fmt.Printf("ctx: %v", ctx)
	fmt.Printf("Requested action: %s", request.Action)

	nodePoolsStr := os.Getenv("KARPENTER_NODEPOOLS")
	if nodePoolsStr == "" {
		return fmt.Errorf("KARPENTER_NODEPOOLS environment variable not set")
	}

	forcefulTermination := strings.EqualFold(os.Getenv("FORCEFUL_NODEPOOLS_TERMINATION"), "true")

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
			if k8serrors.IsNotFound(err) {
				fmt.Printf("Nodepool %s not found, skipping\n", nodePoolName)
				continue
			}
			return fmt.Errorf("failed to get nodepool %s: %v", nodePoolName, err)
		}

		switch request.Action {
		case "shutdown":
			fmt.Printf("Scaling down nodepool %s\n", nodePoolName)
			err = unstructured.SetNestedField(np.Object, "0", "spec", "limits", "cpu")
			if err != nil {
				return fmt.Errorf("failed to set cpu limit for nodepool %s: %v", nodePoolName, err)
			}

			_, err = dynamicClient.Resource(nodePoolGVR).Update(ctx, np, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("failed to update nodepool %s: %v", nodePoolName, err)
			}

			fmt.Printf("Successfully updated nodepool %s to set cpu limit to 0\n", nodePoolName)

			// Pass 1: graceful deletion — let Karpenter attempt to drain pods normally
			fmt.Printf("Gracefully deleting nodeclaims for nodepool %s...\n", nodePoolName)
			if err := deleteSpotNodeclaims(ctx, dynamicClient, nodePoolName, false); err != nil {
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

	if request.Action == "shutdown" {
		// Wait for Karpenter to gracefully drain and delete NodeClaims
		waitSeconds("GRACEFUL_TERMINATION_WAIT_SECONDS", 60)

		if forcefulTermination {
			// Pass 2: force-delete any NodeClaims still stuck (PDB-blocked or unreachable nodes)
			for _, nodePoolName := range nodePoolNames {
				if nodePoolName == "" {
					continue
				}
				fmt.Printf("Force-deleting remaining nodeclaims for nodepool %s...\n", nodePoolName)
				if err := deleteSpotNodeclaims(ctx, dynamicClient, nodePoolName, true); err != nil {
					return fmt.Errorf("failed to force-delete nodeclaims for nodepool %s: %v", nodePoolName, err)
				}
			}
		}

		typedClient, err := newTypedClient(ctx)
		if err != nil {
			return fmt.Errorf("failed to create typed client: %v", err)
		}

		// Wait for NodeClaim deletion to propagate before checking nodes
		waitSeconds("NODE_DELETE_WAIT_SECONDS", 60)

		nodeNames, err := deleteStuckNodes(ctx, typedClient, nodePoolNames)
		if err != nil {
			return fmt.Errorf("failed to delete stuck nodes: %v", err)
		}

		// Wait for node deletion to propagate before checking pods
		waitSeconds("POD_DELETE_WAIT_SECONDS", 60)

		if err := deleteStuckPods(ctx, typedClient, nodeNames); err != nil {
			return fmt.Errorf("failed to delete stuck pods: %v", err)
		}

		// Terminate EC2 instances
		if err := ShutdownEC2Instances(ctx, nodePoolNames); err != nil {
			return err
		}
	}

	return nil
}

func main() {
	lambda.Start(handler)
}
