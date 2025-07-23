package main

import (
	"os"
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/jsii-runtime-go"
)

func TestKarpenterAwsShutdownScheduleStack(t *testing.T) {
	// GIVEN
	app := awscdk.NewApp(nil)

	// Store and clear all relevant environment variables
	envVars := []string{
		"KARPENTER_NODEPOOL_NAME", 
		"KUBERNETES_SERVICE_HOST", 
		"KUBERNETES_CLUSTER_NAME",
		"KARPENTER_VPC_ID",
		"KARPENTER_SUBNET", 
		"KARPENTER_SECURITY_GROUP",
	}
	originalValues := make(map[string]string)
	
	for _, envVar := range envVars {
		originalValues[envVar] = os.Getenv(envVar)
		os.Unsetenv(envVar)
	}
	
	defer func() {
		// Restore original values
		for _, envVar := range envVars {
			if originalValue := originalValues[envVar]; originalValue != "" {
				os.Setenv(envVar, originalValue)
			}
		}
	}()

	// Set test values
	os.Setenv("KARPENTER_NODEPOOL_NAME", "test-nodepool")

	// WHEN
	stack := NewKarpenterAwsShutdownScheduleStack(app, "MyStack", nil)

	// THEN
	template := assertions.Template_FromStack(stack, nil)

	// Assert Lambda Function is created
	template.HasResourceProperties(jsii.String("AWS::Lambda::Function"), map[string]interface{}{
		"Handler": jsii.String("main"),
		"Runtime": jsii.String("provided.al2023"),
		"Architectures": &[]interface{}{
			jsii.String("arm64"),
		},
		"Environment": map[string]interface{}{
			"Variables": map[string]interface{}{
				"KARPENTER_NODEPOOL_NAME": jsii.String("test-nodepool"),
				"KUBERNETES_SERVICE_HOST": jsii.String("https://k8s.api"),
				"KUBERNETES_CLUSTER_NAME": jsii.String("dummy"),
			},
		},
	})

	// Assert Lambda Role is created
	template.HasResourceProperties(jsii.String("AWS::IAM::Role"), map[string]interface{}{
		"AssumeRolePolicyDocument": map[string]interface{}{
			"Statement": []interface{}{
				map[string]interface{}{
					"Action":    jsii.String("sts:AssumeRole"),
					"Effect":    jsii.String("Allow"),
					"Principal": map[string]interface{}{"Service": jsii.String("lambda.amazonaws.com")},
				},
			},
		},
	})

	// Assert Shutdown Schedule is created
	template.HasResourceProperties(jsii.String("AWS::Scheduler::Schedule"), map[string]interface{}{
		"ScheduleExpression":         jsii.String("cron(0 22 * * ? *)"),
		"ScheduleExpressionTimezone": jsii.String("Australia/Sydney"),
		"Target": map[string]interface{}{
			"Input": jsii.String(`{"Action": "shutdown"}`),
		},
	})

	// Assert Startup Schedule is created
	template.HasResourceProperties(jsii.String("AWS::Scheduler::Schedule"), map[string]interface{}{
		"ScheduleExpression":         jsii.String("cron(0 7 * * ? *)"),
		"ScheduleExpressionTimezone": jsii.String("Australia/Sydney"),
		"Target": map[string]interface{}{
			"Input": jsii.String(`{"Action": "startup"}`),
		},
	})

	// Assert Scheduler Role is created
	template.HasResourceProperties(jsii.String("AWS::IAM::Role"), map[string]interface{}{
		"AssumeRolePolicyDocument": map[string]interface{}{
			"Statement": []interface{}{
				map[string]interface{}{
					"Action":    jsii.String("sts:AssumeRole"),
					"Effect":    jsii.String("Allow"),
					"Principal": map[string]interface{}{"Service": jsii.String("scheduler.amazonaws.com")},
				},
			},
		},
	})
}

func TestKarpenterAwsShutdownScheduleStackDefaultNodepoolName(t *testing.T) {
	// GIVEN
	app := awscdk.NewApp(nil)

	// Store and clear all relevant environment variables
	envVars := []string{
		"KARPENTER_NODEPOOL_NAME", 
		"KUBERNETES_SERVICE_HOST", 
		"KUBERNETES_CLUSTER_NAME",
		"KARPENTER_VPC_ID",
		"KARPENTER_SUBNET", 
		"KARPENTER_SECURITY_GROUP",
	}
	originalValues := make(map[string]string)
	
	for _, envVar := range envVars {
		originalValues[envVar] = os.Getenv(envVar)
		os.Unsetenv(envVar)
	}
	
	defer func() {
		// Restore original values
		for _, envVar := range envVars {
			if originalValue := originalValues[envVar]; originalValue != "" {
				os.Setenv(envVar, originalValue)
			}
		}
	}()

	// Don't set KARPENTER_NODEPOOL_NAME to test default behavior

	// WHEN
	stack := NewKarpenterAwsShutdownScheduleStack(app, "MyStack", nil)

	// THEN
	template := assertions.Template_FromStack(stack, nil)

	// Assert Lambda Function uses default nodepool name when env var is not set
	template.HasResourceProperties(jsii.String("AWS::Lambda::Function"), map[string]interface{}{
		"Environment": map[string]interface{}{
			"Variables": map[string]interface{}{
				"KARPENTER_NODEPOOL_NAME": jsii.String("default"),
			},
		},
	})
}

func TestKarpenterAwsShutdownScheduleStackWithVPCExistingSecurityGroup(t *testing.T) {
	// GIVEN
	app := awscdk.NewApp(&awscdk.AppProps{
		Context: &map[string]interface{}{
			"@aws-cdk/core:enableStackNameDuplicates": jsii.Bool(true),
		},
	})

	// Set up environment variables for VPC configuration with existing security group
	envVars := map[string]string{
		"KARPENTER_NODEPOOL_NAME":   "test-nodepool",
		"KARPENTER_VPC_ID":          "vpc-12345678",
		"KARPENTER_SUBNET":          "subnet-87654321",
		"KARPENTER_SECURITY_GROUP":  "sg-abcdef123",
	}

	// Store original values and set test values
	originalValues := make(map[string]string)
	for key, value := range envVars {
		originalValues[key] = os.Getenv(key)
		os.Setenv(key, value)
	}

	defer func() {
		// Restore original values
		for key, originalValue := range originalValues {
			if originalValue != "" {
				os.Setenv(key, originalValue)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	// WHEN - This will fail due to VPC lookup requiring AWS context, but that's expected
	// We're testing that the VPC configuration logic is triggered
	defer func() {
		if r := recover(); r != nil {
			// Expected panic due to VPC lookup requiring AWS context
			if panicMsg, ok := r.(string); ok {
				if !containsString(panicMsg, "Cannot retrieve value from context provider vpc-provider") {
					t.Errorf("Unexpected panic message: %s", panicMsg)
				}
			}
		}
	}()

	// This should panic with VPC context error, confirming VPC config was attempted
	NewKarpenterAwsShutdownScheduleStack(app, "MyVPCStack", &KarpenterAwsShutdownScheduleStackProps{
		awscdk.StackProps{
			Env: &awscdk.Environment{
				Account: jsii.String("123456789012"),
				Region:  jsii.String("us-east-1"),
			},
		},
	})

	// If we get here without panic, the test should fail
	t.Error("Expected VPC lookup to fail without proper AWS context, but it didn't")
}

func TestKarpenterAwsShutdownScheduleStackWithVPCAutoCreateSecurityGroup(t *testing.T) {
	// GIVEN
	app := awscdk.NewApp(&awscdk.AppProps{
		Context: &map[string]interface{}{
			"@aws-cdk/core:enableStackNameDuplicates": jsii.Bool(true),
		},
	})

	// Set up environment variables for VPC configuration without security group (auto-create)
	envVars := map[string]string{
		"KARPENTER_NODEPOOL_NAME": "test-nodepool",
		"KARPENTER_VPC_ID":        "vpc-12345678",
		"KARPENTER_SUBNET":        "subnet-87654321",
	}

	// Store original values and set test values
	originalValues := make(map[string]string)
	for key, value := range envVars {
		originalValues[key] = os.Getenv(key)
		os.Setenv(key, value)
	}
	// Ensure security group is not set
	originalSG := os.Getenv("KARPENTER_SECURITY_GROUP")
	os.Unsetenv("KARPENTER_SECURITY_GROUP")

	defer func() {
		// Restore original values
		for key, originalValue := range originalValues {
			if originalValue != "" {
				os.Setenv(key, originalValue)
			} else {
				os.Unsetenv(key)
			}
		}
		if originalSG != "" {
			os.Setenv("KARPENTER_SECURITY_GROUP", originalSG)
		}
	}()

	// WHEN - This will fail due to VPC lookup, but that confirms VPC config was attempted
	defer func() {
		if r := recover(); r != nil {
			// Expected panic due to VPC lookup requiring AWS context
			if panicMsg, ok := r.(string); ok {
				if !containsString(panicMsg, "Cannot retrieve value from context provider vpc-provider") {
					t.Errorf("Unexpected panic message: %s", panicMsg)
				}
			}
		}
	}()

	// This should panic with VPC context error, confirming VPC config was attempted
	NewKarpenterAwsShutdownScheduleStack(app, "MyAutoSGStack", &KarpenterAwsShutdownScheduleStackProps{
		awscdk.StackProps{
			Env: &awscdk.Environment{
				Account: jsii.String("123456789012"),
				Region:  jsii.String("us-east-1"),
			},
		},
	})

	// If we get here without panic, the test should fail
	t.Error("Expected VPC lookup to fail without proper AWS context, but it didn't")
}

func TestKarpenterAwsShutdownScheduleStackWithMultipleSubnets(t *testing.T) {
	// GIVEN
	app := awscdk.NewApp(&awscdk.AppProps{
		Context: &map[string]interface{}{
			"@aws-cdk/core:enableStackNameDuplicates": jsii.Bool(true),
		},
	})

	// Set up environment variables for VPC configuration with multiple subnets
	envVars := map[string]string{
		"KARPENTER_NODEPOOL_NAME":  "test-nodepool",
		"KARPENTER_VPC_ID":         "vpc-12345678",
		"KARPENTER_SUBNET":         "subnet-11111111,subnet-22222222,subnet-33333333",
		"KARPENTER_SECURITY_GROUP": "sg-abcdef123",
	}

	// Store original values and set test values
	originalValues := make(map[string]string)
	for key, value := range envVars {
		originalValues[key] = os.Getenv(key)
		os.Setenv(key, value)
	}

	defer func() {
		// Restore original values
		for key, originalValue := range originalValues {
			if originalValue != "" {
				os.Setenv(key, originalValue)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	// WHEN - This will fail due to VPC lookup, but that confirms VPC config was attempted
	defer func() {
		if r := recover(); r != nil {
			// Expected panic due to VPC lookup requiring AWS context
			if panicMsg, ok := r.(string); ok {
				if !containsString(panicMsg, "Cannot retrieve value from context provider vpc-provider") {
					t.Errorf("Unexpected panic message: %s", panicMsg)
				}
			}
		}
	}()

	// This should panic with VPC context error, confirming VPC config was attempted
	NewKarpenterAwsShutdownScheduleStack(app, "MyMultiSubnetStack", &KarpenterAwsShutdownScheduleStackProps{
		awscdk.StackProps{
			Env: &awscdk.Environment{
				Account: jsii.String("123456789012"),
				Region:  jsii.String("us-east-1"),
			},
		},
	})

	// If we get here without panic, the test should fail
	t.Error("Expected VPC lookup to fail without proper AWS context, but it didn't")
}

func TestKarpenterAwsShutdownScheduleStackWithoutVPCConfiguration(t *testing.T) {
	// GIVEN
	app := awscdk.NewApp(nil)

	// Ensure VPC environment variables are not set
	vpcEnvVars := []string{"KARPENTER_VPC_ID", "KARPENTER_SUBNET", "KARPENTER_SECURITY_GROUP"}
	originalValues := make(map[string]string)
	
	for _, envVar := range vpcEnvVars {
		originalValues[envVar] = os.Getenv(envVar)
		os.Unsetenv(envVar)
	}
	
	// Set required env var
	os.Setenv("KARPENTER_NODEPOOL_NAME", "test-nodepool")

	defer func() {
		// Restore original values
		for envVar, originalValue := range originalValues {
			if originalValue != "" {
				os.Setenv(envVar, originalValue)
			}
		}
		os.Unsetenv("KARPENTER_NODEPOOL_NAME")
	}()

	// WHEN
	stack := NewKarpenterAwsShutdownScheduleStack(app, "MyNonVPCStack", nil)

	// THEN
	template := assertions.Template_FromStack(stack, nil)

	// Assert Lambda Function is created without VPC configuration
	template.HasResourceProperties(jsii.String("AWS::Lambda::Function"), map[string]interface{}{
		"Handler": jsii.String("main"),
		"Runtime": jsii.String("provided.al2023"),
		"Environment": map[string]interface{}{
			"Variables": map[string]interface{}{
				"KARPENTER_NODEPOOL_NAME": jsii.String("test-nodepool"),
			},
		},
	})

	// Assert that VPC configuration is NOT present (no VpcConfig section)
	template.HasResourceProperties(jsii.String("AWS::Lambda::Function"), map[string]interface{}{
		"VpcConfig": assertions.Match_Absent(),
	})
}

func TestKarpenterAwsShutdownScheduleStackPartialVPCConfiguration(t *testing.T) {

	testCases := []struct {
		name        string
		stackSuffix string
		vpcId       string
		subnet      string
		securityGroup string
		shouldTriggerVPC bool
	}{
		{
			name:             "VPC ID only",
			stackSuffix:      "VPCOnly",
			vpcId:            "vpc-12345678",
			subnet:           "",
			securityGroup:    "",
			shouldTriggerVPC: false,
		},
		{
			name:             "Subnet only",
			stackSuffix:      "SubnetOnly",
			vpcId:            "",
			subnet:           "subnet-87654321",
			securityGroup:    "",
			shouldTriggerVPC: false,
		},
		{
			name:             "Security Group only",
			stackSuffix:      "SGOnly",
			vpcId:            "",
			subnet:           "",
			securityGroup:    "sg-abcdef123",
			shouldTriggerVPC: false,
		},
		{
			name:             "VPC ID and Subnet (should trigger)",
			stackSuffix:      "VPCAndSubnet",
			vpcId:            "vpc-12345678",
			subnet:           "subnet-87654321",
			securityGroup:    "",
			shouldTriggerVPC: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a new app for each test case
			app := awscdk.NewApp(nil)
			
			// Store original values
			originalVPC := os.Getenv("KARPENTER_VPC_ID")
			originalSubnet := os.Getenv("KARPENTER_SUBNET")
			originalSG := os.Getenv("KARPENTER_SECURITY_GROUP")
			originalNodepool := os.Getenv("KARPENTER_NODEPOOL_NAME")

			defer func() {
				// Restore original values
				restoreEnvVar("KARPENTER_VPC_ID", originalVPC)
				restoreEnvVar("KARPENTER_SUBNET", originalSubnet)
				restoreEnvVar("KARPENTER_SECURITY_GROUP", originalSG)
				restoreEnvVar("KARPENTER_NODEPOOL_NAME", originalNodepool)
			}()

			// Set test values
			setEnvVar("KARPENTER_VPC_ID", tc.vpcId)
			setEnvVar("KARPENTER_SUBNET", tc.subnet)
			setEnvVar("KARPENTER_SECURITY_GROUP", tc.securityGroup)
			os.Setenv("KARPENTER_NODEPOOL_NAME", "test-nodepool")

			if tc.shouldTriggerVPC {
				// Expect panic due to VPC lookup
				defer func() {
					if r := recover(); r != nil {
						// Expected panic due to VPC lookup requiring AWS context
						if panicMsg, ok := r.(string); ok {
							if !containsString(panicMsg, "Cannot retrieve value from context provider vpc-provider") {
								t.Errorf("Unexpected panic message: %s", panicMsg)
							}
						}
					}
				}()

				NewKarpenterAwsShutdownScheduleStack(app, "TestStack"+tc.stackSuffix, &KarpenterAwsShutdownScheduleStackProps{
					awscdk.StackProps{
						Env: &awscdk.Environment{
							Account: jsii.String("123456789012"),
							Region:  jsii.String("us-east-1"),
						},
					},
				})

				t.Error("Expected VPC lookup to fail, but it didn't")
			} else {
				// Should succeed without VPC configuration
				stack := NewKarpenterAwsShutdownScheduleStack(app, "TestStack"+tc.stackSuffix, nil)
				template := assertions.Template_FromStack(stack, nil)

				// Assert that VPC configuration is NOT present
				template.HasResourceProperties(jsii.String("AWS::Lambda::Function"), map[string]interface{}{
					"VpcConfig": assertions.Match_Absent(),
				})
			}
		})
	}
}

// Helper functions
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > len(substr) && 
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		 containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func setEnvVar(key, value string) {
	if value != "" {
		os.Setenv(key, value)
	} else {
		os.Unsetenv(key)
	}
}

func restoreEnvVar(key, originalValue string) {
	if originalValue != "" {
		os.Setenv(key, originalValue)
	} else {
		os.Unsetenv(key)
	}
}
