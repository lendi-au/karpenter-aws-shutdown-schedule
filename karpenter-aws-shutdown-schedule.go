package main

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsevents"
	"github.com/aws/aws-cdk-go/awscdk/v2/awseventstargets"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
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

	// Lambda Function
	function := awslambda.NewFunction(stack, jsii.String("ShutdownFunction"), &awslambda.FunctionProps{
		Runtime: awslambda.Runtime_GO_1_X(),
		Handler: jsii.String("main"),
		Code:    awslambda.Code_FromAsset(jsii.String("lambda"), nil),
		Environment: &map[string]*string{
			"NODEPOOL_NAME": jsii.String("default"), // Replace with your NodePool name
			"SHUTDOWN_TAG":  jsii.String("karpenter.sh/shutdown-schedule"),
			"AWS_REGION":    stack.Region(),
		},
	})

	// EventBridge Rule
	rule := awsevents.NewRule(stack, jsii.String("ShutdownRule"), &awsevents.RuleProps{
		Schedule: awsevents.Schedule_Expression(jsii.String("cron(0 22 * * ? *)")), // Every day at 10 PM UTC
	})

	rule.AddTarget(awseventstargets.NewLambdaFunction(function, nil))

	// IAM Permissions
	function.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions:   jsii.Strings("ec2:DescribeInstances", "ec2:TerminateInstances", "eks:DescribeCluster"),
		Resources: jsii.Strings("*"),
	}))

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
