package main

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDynamicClientMissingClusterName(t *testing.T) {
	ctx := context.Background()

	// Temporarily unset the environment variable
	originalValue := os.Getenv("KUBERNETES_CLUSTER_NAME")
	os.Unsetenv("KUBERNETES_CLUSTER_NAME")
	defer func() {
		if originalValue != "" {
			os.Setenv("KUBERNETES_CLUSTER_NAME", originalValue)
		}
	}()

	client, err := newDynamicClient(ctx)

	assert.Nil(t, client)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "KUBERNETES_CLUSTER_NAME environment variable not set")
}

func TestNewDynamicClientWithEnvironment(t *testing.T) {
	ctx := context.Background()

	// Set up environment variables
	os.Setenv("KUBERNETES_CLUSTER_NAME", "test-cluster")
	os.Setenv("KUBERNETES_SERVICE_HOST", "https://test.api")
	os.Setenv("AWS_REGION", "us-east-1")
	defer func() {
		os.Unsetenv("KUBERNETES_CLUSTER_NAME")
		os.Unsetenv("KUBERNETES_SERVICE_HOST")
		os.Unsetenv("AWS_REGION")
	}()

	client, err := newDynamicClient(ctx)

	// In a test environment without proper AWS credentials, this should fail
	// but not due to missing environment variables
	assert.Nil(t, client)
	assert.Error(t, err)
	// The error should be related to AWS configuration, not missing env vars
	assert.NotContains(t, err.Error(), "KUBERNETES_CLUSTER_NAME environment variable not set")
}

func TestNewDynamicClientWithDefaultRegion(t *testing.T) {
	ctx := context.Background()

	// Set up environment variables without AWS_REGION to test default
	os.Setenv("KUBERNETES_CLUSTER_NAME", "test-cluster")
	os.Setenv("KUBERNETES_SERVICE_HOST", "https://test.api")
	os.Unsetenv("AWS_REGION") // Ensure AWS_REGION is not set
	defer func() {
		os.Unsetenv("KUBERNETES_CLUSTER_NAME")
		os.Unsetenv("KUBERNETES_SERVICE_HOST")
	}()

	client, err := newDynamicClient(ctx)

	// Should attempt to use default region (ap-southeast-2)
	assert.Nil(t, client)
	assert.Error(t, err)
	// Should not fail due to missing cluster name
	assert.NotContains(t, err.Error(), "KUBERNETES_CLUSTER_NAME environment variable not set")
	// Should progress beyond region validation
	assert.NotContains(t, err.Error(), "AWS_REGION environment variable not set")
}

func TestNewDynamicClientWithCustomRegion(t *testing.T) {
	ctx := context.Background()

	// Set up environment variables with custom AWS_REGION
	os.Setenv("KUBERNETES_CLUSTER_NAME", "production-cluster")
	os.Setenv("KUBERNETES_SERVICE_HOST", "https://prod-k8s.api")
	os.Setenv("AWS_REGION", "eu-west-1")
	defer func() {
		os.Unsetenv("KUBERNETES_CLUSTER_NAME")
		os.Unsetenv("KUBERNETES_SERVICE_HOST")
		os.Unsetenv("AWS_REGION")
	}()

	client, err := newDynamicClient(ctx)

	// Should attempt to use custom region
	assert.Nil(t, client)
	assert.Error(t, err)
	// Should not fail due to missing environment variables
	assert.NotContains(t, err.Error(), "KUBERNETES_CLUSTER_NAME environment variable not set")
}

func TestNewDynamicClientEnvironmentValidation(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		clusterName string
		serviceHost string
		awsRegion   string
		expectError string
	}{
		{
			name:        "valid environment setup",
			clusterName: "my-cluster",
			serviceHost: "https://k8s.example.com",
			awsRegion:   "us-west-2",
			expectError: "", // Should not fail on env validation
		},
		{
			name:        "valid with minimal setup",
			clusterName: "minimal-cluster",
			serviceHost: "https://minimal.api",
			awsRegion:   "",
			expectError: "", // Should use default region
		},
		{
			name:        "missing service host should not fail validation",
			clusterName: "cluster-without-host",
			serviceHost: "",
			awsRegion:   "ap-southeast-2",
			expectError: "", // Function doesn't validate service host initially
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean environment first
			os.Unsetenv("KUBERNETES_CLUSTER_NAME")
			os.Unsetenv("KUBERNETES_SERVICE_HOST")
			os.Unsetenv("AWS_REGION")

			// Set up test environment
			if tt.clusterName != "" {
				os.Setenv("KUBERNETES_CLUSTER_NAME", tt.clusterName)
			}
			if tt.serviceHost != "" {
				os.Setenv("KUBERNETES_SERVICE_HOST", tt.serviceHost)
			}
			if tt.awsRegion != "" {
				os.Setenv("AWS_REGION", tt.awsRegion)
			}

			defer func() {
				os.Unsetenv("KUBERNETES_CLUSTER_NAME")
				os.Unsetenv("KUBERNETES_SERVICE_HOST")
				os.Unsetenv("AWS_REGION")
			}()

			client, err := newDynamicClient(ctx)

			// Should fail due to AWS/EKS issues, not environment validation
			assert.Nil(t, client)
			require.Error(t, err)

			if tt.expectError != "" {
				assert.Contains(t, err.Error(), tt.expectError)
			} else {
				// Should not fail due to environment variable validation
				assert.NotContains(t, err.Error(), "KUBERNETES_CLUSTER_NAME environment variable not set")
			}
		})
	}
}

func TestNewDynamicClientClusterNameValidation(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name        string
		clusterName string
		shouldPass  bool
	}{
		{"empty cluster name", "", false},
		{"simple cluster name", "test", true},
		{"cluster with hyphens", "my-test-cluster", true},
		{"cluster with numbers", "cluster123", true},
		{"cluster with mixed", "prod-cluster-2024", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clean environment
			os.Unsetenv("KUBERNETES_CLUSTER_NAME")
			os.Unsetenv("KUBERNETES_SERVICE_HOST")
			os.Unsetenv("AWS_REGION")

			if tc.clusterName != "" {
				os.Setenv("KUBERNETES_CLUSTER_NAME", tc.clusterName)
			}
			os.Setenv("KUBERNETES_SERVICE_HOST", "https://test.api")
			os.Setenv("AWS_REGION", "us-east-1")

			defer func() {
				os.Unsetenv("KUBERNETES_CLUSTER_NAME")
				os.Unsetenv("KUBERNETES_SERVICE_HOST")
				os.Unsetenv("AWS_REGION")
			}()

			client, err := newDynamicClient(ctx)

			assert.Nil(t, client) // Always nil in test environment
			require.Error(t, err) // Always error in test environment

			if tc.shouldPass {
				// Should not fail due to cluster name validation
				assert.NotContains(t, err.Error(), "KUBERNETES_CLUSTER_NAME environment variable not set")
			} else {
				// Should fail due to cluster name validation
				assert.Contains(t, err.Error(), "KUBERNETES_CLUSTER_NAME environment variable not set")
			}
		})
	}
}
