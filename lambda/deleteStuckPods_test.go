package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// terminatingPod creates a pod with DeletionTimestamp set, simulating Terminating state.
func terminatingPod(name, namespace, nodeName string) *corev1.Pod {
	now := metav1.NewTime(time.Now())
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
			DeletionTimestamp: &now,
		},
		Spec: corev1.PodSpec{NodeName: nodeName},
	}
}

func TestDeleteStuckPodsNoNodeNames(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()

	err := deleteStuckPods(ctx, client, []string{})
	assert.NoError(t, err)
}

func TestDeleteStuckPodsNoStuckPods(t *testing.T) {
	ctx := context.Background()

	// Running pod — no DeletionTimestamp, should not be deleted
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "running-pod", Namespace: "default"},
		Spec:       corev1.PodSpec{NodeName: "node-1"},
	}
	client := fake.NewSimpleClientset(pod)

	err := deleteStuckPods(ctx, client, []string{"node-1"})
	require.NoError(t, err)

	_, err = client.CoreV1().Pods("default").Get(ctx, "running-pod", metav1.GetOptions{})
	assert.NoError(t, err, "running pod should not have been deleted")
}

func TestDeleteStuckPodsDeletesTerminatingPod(t *testing.T) {
	ctx := context.Background()

	pod := terminatingPod("stuck-pod", "default", "node-1")
	client := fake.NewSimpleClientset(pod)

	err := deleteStuckPods(ctx, client, []string{"node-1"})
	require.NoError(t, err)

	_, err = client.CoreV1().Pods("default").Get(ctx, "stuck-pod", metav1.GetOptions{})
	assert.Error(t, err, "stuck pod should have been deleted")
}

func TestDeleteStuckPodsIgnoresDifferentNode(t *testing.T) {
	ctx := context.Background()

	pod := terminatingPod("stuck-pod", "default", "node-2")
	client := fake.NewSimpleClientset(pod)

	err := deleteStuckPods(ctx, client, []string{"node-1"})
	require.NoError(t, err)

	_, err = client.CoreV1().Pods("default").Get(ctx, "stuck-pod", metav1.GetOptions{})
	assert.NoError(t, err, "pod on a different node should be untouched")
}

// shouldSkipPod unit tests — each exclusion rule tested independently

func TestShouldSkipPodNotTerminating(t *testing.T) {
	pod := corev1.Pod{}
	assert.True(t, shouldSkipPod(pod), "non-terminating pod should be skipped")
}

func TestShouldSkipPodMirror(t *testing.T) {
	now := metav1.Now()
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			DeletionTimestamp: &now,
			Annotations: map[string]string{
				"kubernetes.io/config.mirror": "abc123",
			},
		},
	}
	assert.True(t, shouldSkipPod(pod), "mirror/static pod should be skipped")
}

func TestShouldSkipPodDaemonSet(t *testing.T) {
	now := metav1.Now()
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			DeletionTimestamp: &now,
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "DaemonSet", Name: "cilium"},
			},
		},
	}
	assert.True(t, shouldSkipPod(pod), "DaemonSet-owned pod should be skipped")
}

func TestShouldSkipPodSystemClusterCritical(t *testing.T) {
	now := metav1.Now()
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &now},
		Spec:       corev1.PodSpec{PriorityClassName: "system-cluster-critical"},
	}
	assert.True(t, shouldSkipPod(pod), "system-cluster-critical pod (CoreDNS) should be skipped")
}

func TestShouldSkipPodSystemNodeCritical(t *testing.T) {
	now := metav1.Now()
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &now},
		Spec:       corev1.PodSpec{PriorityClassName: "system-node-critical"},
	}
	assert.True(t, shouldSkipPod(pod), "system-node-critical pod should be skipped")
}

func TestShouldSkipPodRegularTerminating(t *testing.T) {
	now := metav1.Now()
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &now},
	}
	assert.False(t, shouldSkipPod(pod), "regular terminating pod should not be skipped")
}

func TestShouldSkipPodNonDaemonSetOwner(t *testing.T) {
	now := metav1.Now()
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			DeletionTimestamp: &now,
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "ReplicaSet", Name: "my-app-abc"},
			},
		},
	}
	assert.False(t, shouldSkipPod(pod), "ReplicaSet-owned terminating pod should not be skipped")
}
