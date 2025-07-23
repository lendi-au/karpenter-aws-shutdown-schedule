package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
)

func TestDeleteSpotNodeclaimsNoItems(t *testing.T) {
	ctx := context.Background()
	
	// Create a fake dynamic client with custom list kinds
	scheme := runtime.NewScheme()
	fakeDynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, 
		map[schema.GroupVersionResource]string{
			{Group: "karpenter.sh", Version: "v1", Resource: "nodeclaims"}: "NodeClaimList",
		})
	
	// Convert to interface type
	var dynamicClient dynamic.Interface = fakeDynamicClient
	
	nodePoolName := "test-pool"
	err := deleteSpotNodeclaims(ctx, dynamicClient, nodePoolName)
	
	// Should succeed with no items to delete
	assert.NoError(t, err)
}

func TestDeleteSpotNodeclaimsWithItems(t *testing.T) {
	ctx := context.Background()
	
	// Create a fake nodeclaim object
	nodeClaimGVR := schema.GroupVersionResource{
		Group:    "karpenter.sh",
		Version:  "v1",
		Resource: "nodeclaims",
	}
	
	nodeclaim := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "karpenter.sh/v1",
			"kind":       "NodeClaim",
			"metadata": map[string]interface{}{
				"name": "test-nodeclaim-1",
				"labels": map[string]interface{}{
					"karpenter.sh/nodepool": "test-pool",
				},
			},
		},
	}
	
	// Create a fake dynamic client with custom list kinds
	scheme := runtime.NewScheme()
	fakeDynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			{Group: "karpenter.sh", Version: "v1", Resource: "nodeclaims"}: "NodeClaimList",
		},
		nodeclaim)
	
	// Convert to interface type
	var dynamicClient dynamic.Interface = fakeDynamicClient
	
	nodePoolName := "test-pool"
	err := deleteSpotNodeclaims(ctx, dynamicClient, nodePoolName)
	
	// Should succeed
	assert.NoError(t, err)
	
	// Verify the nodeclaim was deleted by trying to list it
	listResult, err := dynamicClient.Resource(nodeClaimGVR).List(ctx, metav1.ListOptions{
		LabelSelector: "karpenter.sh/nodepool=test-pool",
	})
	assert.NoError(t, err)
	assert.Equal(t, 0, len(listResult.Items))
}