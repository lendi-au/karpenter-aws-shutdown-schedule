package main

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShutdownEC2InstancesMissingNodepools(t *testing.T) {
	ctx := context.Background()

	// Test with empty nodepool names
	err := ShutdownEC2Instances(ctx, []string{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no nodepool names provided")
}

func TestShutdownEC2InstancesWithNodepools(t *testing.T) {
	ctx := context.Background()

	// Set up environment variables
	os.Setenv("AWS_REGION", "us-east-1")
	defer func() {
		os.Unsetenv("AWS_REGION")
	}()

	// Test with valid nodepool names
	nodepools := []string{"test-pool-1", "test-pool-2"}
	err := ShutdownEC2Instances(ctx, nodepools)

	// In a test environment, the function may succeed if AWS config loads but no instances found
	// or it may fail if AWS credentials are not available
	// Either outcome is acceptable for this test - we just want to ensure the nodepool check passed
	if err != nil {
		assert.NotContains(t, err.Error(), "no nodepool names provided")
	}
}
