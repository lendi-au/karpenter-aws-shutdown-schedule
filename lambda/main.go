package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func handler(ctx context.Context, request events.CloudWatchEvent) error {
	// Kubernetes API interaction
	config, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("failed to get in-cluster config: %v", err)
	}
	_, err = kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %v", err)
	}

	nodePoolName := os.Getenv("NODEPOOL_NAME")
	if nodePoolName == "" {
		return fmt.Errorf("NODEPOOL_NAME environment variable not set")
	}

	// This is a placeholder for the actual Karpenter API interaction
	// You would need to use the Karpenter client to update the NodePool
	fmt.Printf("Simulating scaling down nodepool %s\n", nodePoolName)

	// EC2 interaction
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(os.Getenv("AWS_REGION")),
	})
	if err != nil {
		return fmt.Errorf("failed to create AWS session: %v", err)
	}

	ec2Svc := ec2.New(sess)

	shutdownTag := os.Getenv("SHUTDOWN_TAG")
	if shutdownTag == "" {
		return fmt.Errorf("SHUTDOWN_TAG environment variable not set")
	}

	input := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("tag:" + shutdownTag),
				Values: aws.StringSlice([]string{"true"}),
			},
		},
	}

	result, err := ec2Svc.DescribeInstances(input)
	if err != nil {
		return fmt.Errorf("failed to describe instances: %v", err)
	}

	var instanceIds []string
	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			instanceIds = append(instanceIds, *instance.InstanceId)
		}
	}

	if len(instanceIds) > 0 {
		fmt.Printf("Terminating instances: %v\n", instanceIds)
		terminateInput := &ec2.TerminateInstancesInput{
			InstanceIds: aws.StringSlice(instanceIds),
		}
		_, err := ec2Svc.TerminateInstances(terminateInput)
		if err != nil {
			return fmt.Errorf("failed to terminate instances: %v", err)
		}
	}

	return nil
}

func main() {
	lambda.Start(handler)
}
