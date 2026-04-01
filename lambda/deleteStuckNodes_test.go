package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestDeleteStuckNodesNoNodes(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()

	nodeNames, err := deleteStuckNodes(ctx, client, []string{"test-pool"})

	require.NoError(t, err)
	assert.Empty(t, nodeNames)
}

func TestDeleteStuckNodesWithNodes(t *testing.T) {
	ctx := context.Background()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ip-10-0-0-1",
			Labels: map[string]string{
				"karpenter.sh/nodepool": "test-pool",
			},
		},
	}
	client := fake.NewSimpleClientset(node)

	nodeNames, err := deleteStuckNodes(ctx, client, []string{"test-pool"})

	require.NoError(t, err)
	assert.Equal(t, []string{"ip-10-0-0-1"}, nodeNames)

	// Verify node was deleted
	_, err = client.CoreV1().Nodes().Get(ctx, "ip-10-0-0-1", metav1.GetOptions{})
	assert.Error(t, err)
}

func TestDeleteStuckNodesMultiplePools(t *testing.T) {
	ctx := context.Background()

	node1 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "ip-10-0-0-1",
			Labels: map[string]string{"karpenter.sh/nodepool": "pool-a"},
		},
	}
	node2 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "ip-10-0-0-2",
			Labels: map[string]string{"karpenter.sh/nodepool": "pool-b"},
		},
	}
	client := fake.NewSimpleClientset(node1, node2)

	nodeNames, err := deleteStuckNodes(ctx, client, []string{"pool-a", "pool-b"})

	require.NoError(t, err)
	assert.Len(t, nodeNames, 2)
	assert.Contains(t, nodeNames, "ip-10-0-0-1")
	assert.Contains(t, nodeNames, "ip-10-0-0-2")
}

func TestDeleteStuckNodesSkipsEmptyPoolName(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()

	nodeNames, err := deleteStuckNodes(ctx, client, []string{""})

	require.NoError(t, err)
	assert.Empty(t, nodeNames)
}

func TestDeleteStuckNodesWithFinalizers(t *testing.T) {
	ctx := context.Background()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "ip-10-0-0-1",
			Labels:     map[string]string{"karpenter.sh/nodepool": "test-pool"},
			Finalizers: []string{"node.kubernetes.io/not-ready"},
		},
	}
	client := fake.NewSimpleClientset(node)

	nodeNames, err := deleteStuckNodes(ctx, client, []string{"test-pool"})

	require.NoError(t, err)
	assert.Equal(t, []string{"ip-10-0-0-1"}, nodeNames)

	_, err = client.CoreV1().Nodes().Get(ctx, "ip-10-0-0-1", metav1.GetOptions{})
	assert.Error(t, err, "node with finalizers should have been deleted")
}

func TestDeleteStuckNodesOnlyMatchingPool(t *testing.T) {
	ctx := context.Background()

	// Node from a different pool — should not be touched
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "ip-10-0-0-1",
			Labels: map[string]string{"karpenter.sh/nodepool": "other-pool"},
		},
	}
	client := fake.NewSimpleClientset(node)

	nodeNames, err := deleteStuckNodes(ctx, client, []string{"test-pool"})

	require.NoError(t, err)
	assert.Empty(t, nodeNames)

	// Node should still exist
	_, err = client.CoreV1().Nodes().Get(ctx, "ip-10-0-0-1", metav1.GetOptions{})
	assert.NoError(t, err)
}
