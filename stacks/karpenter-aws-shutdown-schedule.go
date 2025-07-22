package main

import (
	"fmt"
	"os"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsscheduler"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"

	"github.com/edify42/karpenter-aws-shutdown-schedule/pkg/utils"
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

	LambdaRoleArn := os.Getenv("LAMBDA_ROLE_ARN")

	var lambdaRole awsiam.IRole
	var functionProps *awslambda.FunctionProps

	if LambdaRoleArn != "" {
		fmt.Printf("Found LAMBDA_ROLE_ARN: %s. Using this on lambda", LambdaRoleArn)
		lambdaRole = awsiam.Role_FromRoleArn(stack, jsii.String("LambdaRoleArn"), jsii.String(LambdaRoleArn), &awsiam.FromRoleArnOptions{
			// Set mutable to false unless you want to add policies to this role in CDK
			Mutable: jsii.Bool(false),
		})
	} else {
		lambdaRole = awsiam.NewRole(stack, jsii.String("MyLambdaRole"), &awsiam.RoleProps{
			RoleName:  jsii.String(name),
			AssumedBy: awsiam.NewServicePrincipal(jsii.String("lambda.amazonaws.com"), nil),
			ManagedPolicies: &[]awsiam.IManagedPolicy{
				awsiam.ManagedPolicy_FromAwsManagedPolicyName(jsii.String("service-role/AWSLambdaBasicExecutionRole")),
			},
		})
	}

	nodepool := utils.GetenvDefault("KARPENTER_NODEPOOL_NAME", "default")
	k8sHost := utils.GetenvDefault("KUBERNETES_SERVICE_HOST", "https://k8s.api")
	clusterName := utils.GetenvDefault("KUBERNETES_CLUSTER_NAME", "dummy")
	envMap := map[string]*string{
		"KARPENTER_NODEPOOL_NAME": jsii.String(nodepool), // Replace with your NodePool name
		"KUBERNETES_SERVICE_HOST": &k8sHost,
		"KUBERNETES_CLUSTER_NAME": &clusterName,
	}
	if os.Getenv("KARPENTER_EXTRA_SHUTDOWN_TAG") != "" {
		envMap["SHUTDOWN_TAG"] = jsii.String(os.Getenv("KARPENTER_EXTRA_SHUTDOWN_TAG"))
	}

	functionProps = &awslambda.FunctionProps{
		Runtime:      awslambda.Runtime_PROVIDED_AL2023(),
		FunctionName: jsii.String(name),
		Architecture: arch,
		Handler:      jsii.String("main"),
		Code:         awslambda.Code_FromAsset(jsii.String("../build"), nil),
		Environment:  &envMap,
		Role:         lambdaRole, // adds custom role to lambda
	}

	function := awslambda.NewFunction(stack, jsii.String(name), functionProps)

	if LambdaRoleArn == "" {
		// IAM Permissions as no role was set
		function.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
			Actions:   jsii.Strings("ec2:DescribeInstances", "ec2:TerminateInstances", "eks:DescribeCluster"),
			Resources: jsii.Strings("*"),
		}))
	}

	// EventBridge Scheduler with timezone support
	shutdownSchedule := utils.GetenvDefault("KARPENTER_NODEPOOL_SHUTDOWN_SCHEDULE", "cron(0 22 * * ? *)") // 10pm night time
	startupSchedule := utils.GetenvDefault("KARPENTER_NODEPOOL_STARTUP_SCHEDULE", "cron(0 7 * * ? *)")    // 7am morning time
	timezone := utils.GetenvDefault("KARPENTER_SCHEDULE_TIMEZONE", "Australia/Sydney")                    // Yes I'm in Sydney - Adjust to your needs.

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
	return nil
}
