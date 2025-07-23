package main

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestActionEvent(t *testing.T) {
	event := ActionEvent{Action: "shutdown"}
	assert.Equal(t, "shutdown", event.Action)

	event2 := ActionEvent{Action: "startup"}
	assert.Equal(t, "startup", event2.Action)
}

func TestHandlerMissingEnvironmentVariable(t *testing.T) {
	ctx := context.Background()

	// Temporarily unset the environment variable
	originalValue := os.Getenv("KARPENTER_NODEPOOL_NAME")
	os.Unsetenv("KARPENTER_NODEPOOL_NAME")
	defer func() {
		if originalValue != "" {
			os.Setenv("KARPENTER_NODEPOOL_NAME", originalValue)
		}
	}()

	request := ActionEvent{Action: "shutdown"}
	err := handler(ctx, request)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "KARPENTER_NODEPOOL_NAME environment variable not set")
}

func TestHandlerWithMockEnvironment(t *testing.T) {
	// Set up environment variables for testing
	os.Setenv("KARPENTER_NODEPOOL_NAME", "test-pool")
	os.Setenv("KUBERNETES_CLUSTER_NAME", "test-cluster")
	os.Setenv("KUBERNETES_SERVICE_HOST", "https://test.api")
	os.Setenv("AWS_REGION", "us-east-1")
	defer func() {
		os.Unsetenv("KARPENTER_NODEPOOL_NAME")
		os.Unsetenv("KUBERNETES_CLUSTER_NAME")
		os.Unsetenv("KUBERNETES_SERVICE_HOST")
		os.Unsetenv("AWS_REGION")
	}()

	ctx := context.Background()

	// Test with invalid action that should not cause immediate failure
	request := ActionEvent{Action: "invalid"}
	err := handler(ctx, request)

	// The handler should fail when trying to create the dynamic client
	// since we don't have real AWS credentials in the test environment
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create dynamic client")
}
