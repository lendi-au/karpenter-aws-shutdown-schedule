package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsscheduler"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"

	"github.com/lendi-au/karpenter-aws-shutdown-schedule/pkg/utils"
)

type KarpenterAwsShutdownScheduleStackProps struct {
	awscdk.StackProps
}

func NewKarpenterAwsShutdownScheduleStack(scope constructs.Construct, id string, props *KarpenterAwsShutdownScheduleStackProps) awscdk.Stack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	name := "karpenter-ec2-instance-stop-start"
	arch := awslambda.Architecture_ARM_64()

	if os.Getenv("BUILD_ARCH") == "amd64" {
		arch = awslambda.Architecture_X86_64()
	}

	// ========================================================================
	// IAM ROLE SETUP
	// ========================================================================
	// This section consolidates all IAM role and policy configuration for the Lambda function

	LambdaRoleArn := os.Getenv("LAMBDA_ROLE_ARN")
	vpcId := os.Getenv("KARPENTER_VPC_ID")
	subnetId := os.Getenv("KARPENTER_SUBNET")
	hasVpcConfig := vpcId != "" && subnetId != ""

	var lambdaRole awsiam.IRole

	if LambdaRoleArn != "" {
		// Using a pre-existing IAM role
		fmt.Printf("Found LAMBDA_ROLE_ARN: %s. Using this role for the Lambda function.\n", LambdaRoleArn)
		fmt.Println("NOTE: When using a custom IAM role, ensure it has the following permissions:")
		fmt.Println("  - ec2:DescribeInstances")
		fmt.Println("  - ec2:TerminateInstances")
		fmt.Println("  - eks:DescribeCluster")
		fmt.Println("  - CloudWatch Logs permissions (AWSLambdaBasicExecutionRole)")
		if hasVpcConfig {
			fmt.Println("  - VPC permissions (AWSLambdaVPCAccessExecutionRole)")
		}
		fmt.Println("  - The role must be mapped in your EKS cluster's aws-auth ConfigMap")

		lambdaRole = awsiam.Role_FromRoleArn(stack, jsii.String("LambdaRole"), jsii.String(LambdaRoleArn), &awsiam.FromRoleArnOptions{
			Mutable: jsii.Bool(false),
		})
	} else {
		// Create a new IAM role with all necessary permissions
		fmt.Println("Creating new IAM role for Lambda function with required permissions")

		// Add VPC execution policy if VPC configuration is present
		var managedPolicies []awsiam.IManagedPolicy
		if hasVpcConfig {
			fmt.Println("VPC configuration detected - adding VPC execution permissions")
			managedPolicies = append(managedPolicies,
				awsiam.ManagedPolicy_FromAwsManagedPolicyName(jsii.String("service-role/AWSLambdaVPCAccessExecutionRole")))
		}

		lambdaRole = awsiam.NewRole(stack, jsii.String("LambdaRole"), &awsiam.RoleProps{
			RoleName:        jsii.String(name),
			AssumedBy:       awsiam.NewServicePrincipal(jsii.String("lambda.amazonaws.com"), nil),
			ManagedPolicies: &managedPolicies,
			InlinePolicies: &map[string]awsiam.PolicyDocument{
				"LambdaPermissions": awsiam.NewPolicyDocument(&awsiam.PolicyDocumentProps{
					Statements: &[]awsiam.PolicyStatement{
						awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
							Effect: awsiam.Effect_ALLOW,
							Actions: jsii.Strings(
								"ec2:DescribeInstances",
								"ec2:TerminateInstances",
								"eks:DescribeCluster",
								"logs:CreateLogGroup",
								"logs:CreateLogStream",
								"logs:PutLogEvents",
							),
							Resources: jsii.Strings("*"),
						}),
					},
				}),
			},
		})

		fmt.Printf("Created IAM role: %s\n", name)
		fmt.Println("IMPORTANT: Map this role in your EKS cluster's aws-auth ConfigMap with the appropriate permissions")
	}

	// ========================================================================
	// LAMBDA ENVIRONMENT VARIABLES
	// ========================================================================
	nodepool := utils.GetenvDefault("KARPENTER_NODEPOOL_NAME", "default")
	k8sHost := utils.GetenvDefault("KUBERNETES_SERVICE_HOST", "https://k8s.api")
	clusterName := utils.GetenvDefault("KUBERNETES_CLUSTER_NAME", "dummy")
	envMap := map[string]*string{
		"KARPENTER_NODEPOOL_NAME": jsii.String(nodepool),
		"KUBERNETES_SERVICE_HOST": &k8sHost,
		"KUBERNETES_CLUSTER_NAME": &clusterName,
	}
	if os.Getenv("KARPENTER_EXTRA_SHUTDOWN_TAG") != "" {
		envMap["SHUTDOWN_TAG"] = jsii.String(os.Getenv("KARPENTER_EXTRA_SHUTDOWN_TAG"))
	}

	// ========================================================================
	// BUILD PATH CONFIGURATION
	// ========================================================================
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Current working directory: ", cwd)

	// Check if we're in the stacks directory (during testing) or root directory (during deployment)
	var buildPath string
	if filepath.Base(cwd) == "stacks" {
		buildPath = "../build"
	} else {
		buildPath = "./build"
	}
	fmt.Println("Build path: ", buildPath)

	// ========================================================================
	// VPC CONFIGURATION
	// ========================================================================
	// Environment variables:
	// KARPENTER_VPC_ID=vpc-xxxxx (mandatory for VPC config)
	// KARPENTER_SUBNET=subnet-xxxxx (mandatory for VPC config, supports comma-separated list)
	// KARPENTER_SECURITY_GROUP=sg-xxxxx (optional, creates new SG if not provided)
	var vpcConfig *awsec2.SubnetSelection
	var securityGroups *[]awsec2.ISecurityGroup
	var vpc awsec2.IVpc
	securityGroupId := os.Getenv("KARPENTER_SECURITY_GROUP")

	if vpcId != "" && subnetId != "" {
		fmt.Printf("Configuring Lambda with VPC: %s, Subnet: %s\n", vpcId, subnetId)

		// Import existing VPC
		vpc = awsec2.Vpc_FromLookup(stack, jsii.String("ExistingVPC"), &awsec2.VpcLookupOptions{
			VpcId: jsii.String(vpcId),
		})

		// Parse subnet IDs (support comma-separated list)
		subnetIds := strings.Split(subnetId, ",")
		var subnets []awsec2.ISubnet
		for i, subId := range subnetIds {
			subId = strings.TrimSpace(subId)
			subnet := awsec2.Subnet_FromSubnetId(stack, jsii.String(fmt.Sprintf("ExistingSubnet%d", i)), jsii.String(subId))
			subnets = append(subnets, subnet)
		}

		// Handle Security Group
		var sgs []awsec2.ISecurityGroup
		if securityGroupId != "" {
			fmt.Printf("Using existing security group: %s\n", securityGroupId)
			// Import existing security group
			existingSG := awsec2.SecurityGroup_FromSecurityGroupId(stack, jsii.String("ExistingSecurityGroup"), jsii.String(securityGroupId), &awsec2.SecurityGroupImportOptions{})
			sgs = append(sgs, existingSG)
		} else {
			fmt.Println("Creating new security group with no inbound rules and open egress")
			// Create new security group with no inbound rules and open egress
			newSG := awsec2.NewSecurityGroup(stack, jsii.String("LambdaSecurityGroup"), &awsec2.SecurityGroupProps{
				Vpc:               vpc,
				Description:       jsii.String("Security group for Karpenter Lambda function"),
				SecurityGroupName: jsii.String(fmt.Sprintf("%s-sg", name)),
				AllowAllOutbound:  jsii.Bool(true), // Open egress rules
			})
			// Note: By default, CDK security groups have no inbound rules, so we don't need to explicitly remove them
			sgs = append(sgs, newSG)
		}

		vpcConfig = &awsec2.SubnetSelection{Subnets: &subnets}
		securityGroups = &sgs
	}

	// ========================================================================
	// LAMBDA FUNCTION CONFIGURATION
	// ========================================================================
	var functionProps *awslambda.FunctionProps

	if vpcConfig != nil && securityGroups != nil {
		functionProps = &awslambda.FunctionProps{
			Runtime:           awslambda.Runtime_PROVIDED_AL2023(),
			FunctionName:      jsii.String(name),
			Architecture:      arch,
			Handler:           jsii.String("main"),
			Code:              awslambda.Code_FromAsset(jsii.String(buildPath), nil),
			Environment:       &envMap,
			Role:              lambdaRole,
			Vpc:               vpc,
			VpcSubnets:        vpcConfig,
			SecurityGroups:    securityGroups,
			AllowPublicSubnet: jsii.Bool(true),
			Timeout:           awscdk.Duration_Minutes(jsii.Number(5)),
		}
	} else {
		functionProps = &awslambda.FunctionProps{
			Runtime:      awslambda.Runtime_PROVIDED_AL2023(),
			FunctionName: jsii.String(name),
			Architecture: arch,
			Handler:      jsii.String("main"),
			Code:         awslambda.Code_FromAsset(jsii.String(buildPath), nil),
			Environment:  &envMap,
			Role:         lambdaRole,
			Timeout:      awscdk.Duration_Minutes(jsii.Number(5)),
		}
	}

	function := awslambda.NewFunction(stack, jsii.String(name), functionProps)

	// ========================================================================
	// EVENTBRIDGE SCHEDULER CONFIGURATION
	// ========================================================================
	shutdownSchedule := utils.GetenvDefault("KARPENTER_NODEPOOL_SHUTDOWN_SCHEDULE", "cron(0 22 * * ? *)") // 10pm night time
	startupSchedule := utils.GetenvDefault("KARPENTER_NODEPOOL_STARTUP_SCHEDULE", "cron(0 7 * * ? *)")    // 7am morning time
	timezone := utils.GetenvDefault("KARPENTER_SCHEDULE_TIMEZONE", "Australia/Sydney")                    // Yes I'm in Sydney - Adjust to your needs.
	state := utils.GetenvDefault("KARPENTER_SCHEDULE_FUNCTION_STATE", "ENABLED")

	// Create IAM role for EventBridge Scheduler
	schedulerRole := awsiam.NewRole(stack, jsii.String("SchedulerRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("scheduler.amazonaws.com"), nil),
		InlinePolicies: &map[string]awsiam.PolicyDocument{
			"LambdaInvokePolicy": awsiam.NewPolicyDocument(&awsiam.PolicyDocumentProps{
				Statements: &[]awsiam.PolicyStatement{
					awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
						Actions:   jsii.Strings("lambda:InvokeFunction"),
						Resources: jsii.Strings(*function.FunctionArn()),
					}),
				},
			}),
		},
	})

	awsscheduler.NewCfnSchedule(stack, jsii.String("ShutdownSchedule"), &awsscheduler.CfnScheduleProps{
		ScheduleExpression:         jsii.String(shutdownSchedule),
		ScheduleExpressionTimezone: jsii.String(timezone),
		Target: &awsscheduler.CfnSchedule_TargetProperty{
			Arn:     function.FunctionArn(),
			RoleArn: schedulerRole.RoleArn(),
			Input:   jsii.String(`{"Action": "shutdown"}`),
		},
		FlexibleTimeWindow: &awsscheduler.CfnSchedule_FlexibleTimeWindowProperty{
			Mode:                   jsii.String("FLEXIBLE"),
			MaximumWindowInMinutes: jsii.Number(10),
		},
		State: &state,
	})

	awsscheduler.NewCfnSchedule(stack, jsii.String("StartupSchedule"), &awsscheduler.CfnScheduleProps{
		ScheduleExpression:         jsii.String(startupSchedule),
		ScheduleExpressionTimezone: jsii.String(timezone),
		Target: &awsscheduler.CfnSchedule_TargetProperty{
			Arn:     function.FunctionArn(),
			RoleArn: schedulerRole.RoleArn(),
			Input:   jsii.String(`{"Action": "startup"}`),
		},
		FlexibleTimeWindow: &awsscheduler.CfnSchedule_FlexibleTimeWindowProperty{
			Mode:                   jsii.String("FLEXIBLE"),
			MaximumWindowInMinutes: jsii.Number(10),
		},
		State: &state,
	})

	return stack
}

func main() {
	defer jsii.Close()

	app := awscdk.NewApp(nil)

	NewKarpenterAwsShutdownScheduleStack(app, "KarpenterAwsShutdownScheduleStack", &KarpenterAwsShutdownScheduleStackProps{
		awscdk.StackProps{
			Env: env(),
		},
	})

	app.Synth(nil)
}

func env() *awscdk.Environment {
	return &awscdk.Environment{
		Account: jsii.String(os.Getenv("CDK_DEFAULT_ACCOUNT")),
		Region:  jsii.String(os.Getenv("CDK_DEFAULT_REGION")),
	}
}
