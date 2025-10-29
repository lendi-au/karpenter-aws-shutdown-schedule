package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// ShutdownEC2Instances terminates EC2 instances with tags matching the given nodepools.
func ShutdownEC2Instances(ctx context.Context, nodePoolNames []string) error {
	if len(nodePoolNames) == 0 {
		return fmt.Errorf("no nodepool names provided")
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(os.Getenv("AWS_REGION")))
	if err != nil {
		return fmt.Errorf("failed to load AWS configuration: %v", err)
	}

	ec2Svc := ec2.NewFromConfig(cfg)

	// Build filters for all nodepools
	ec2NodeTagKey := "tag:karpenter.sh/nodepool"
	input := &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   &ec2NodeTagKey,
				Values: nodePoolNames,
			},
		},
	}

	result, err := ec2Svc.DescribeInstances(ctx, input)
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
			InstanceIds: instanceIds,
		}
		_, err := ec2Svc.TerminateInstances(ctx, terminateInput)
		if err != nil {
			return fmt.Errorf("failed to terminate instances: %v", err)
		}
		fmt.Printf("Successfully terminated %d instance(s)\n", len(instanceIds))
	} else {
		fmt.Printf("Found no matching EC2 instances for nodepools: %v\n", nodePoolNames)
	}

	return nil
}
