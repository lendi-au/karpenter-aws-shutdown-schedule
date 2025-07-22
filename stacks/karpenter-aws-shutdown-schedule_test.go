package main

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/jsii-runtime-go"
)

func TestKarpenterAwsShutdownScheduleStack(t *testing.T) {
	// GIVEN
	app := awscdk.NewApp(nil)

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
				"KARPENTER_NODEPOOL_NAME": jsii.String("default"),
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