package main

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShutdownEC2InstancesMissingEnvironment(t *testing.T) {
	ctx := context.Background()

	// Temporarily unset the environment variable
	originalValue := os.Getenv("KARPENTER_NODEPOOL_NAME")
	os.Unsetenv("KARPENTER_NODEPOOL_NAME")
	defer func() {
		if originalValue != "" {
			os.Setenv("KARPENTER_NODEPOOL_NAME", originalValue)
		}
	}()

	err := ShutdownEC2Instances(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "KARPENTER_NODEPOOL_NAME environment variable not set")
}

func TestShutdownEC2InstancesWithEnvironment(t *testing.T) {
	ctx := context.Background()

	// Set up environment variables
	os.Setenv("KARPENTER_NODEPOOL_NAME", "test-pool")
	os.Setenv("AWS_REGION", "us-east-1")
	defer func() {
		os.Unsetenv("KARPENTER_NODEPOOL_NAME")
		os.Unsetenv("AWS_REGION")
	}()

	err := ShutdownEC2Instances(ctx)

	// In a test environment, the function may succeed if AWS config loads but no instances found
	// or it may fail if AWS credentials are not available
	// Either outcome is acceptable for this test - we just want to ensure the env var check passed
	if err != nil {
		assert.NotContains(t, err.Error(), "KARPENTER_NODEPOOL_NAME environment variable not set")
	}
}
