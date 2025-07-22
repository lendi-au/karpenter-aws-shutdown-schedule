package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/edify42/karpenter-aws-shutdown-schedule/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/aws-iam-authenticator/pkg/token"
)

func newDynamicClient(ctx context.Context) (*dynamic.DynamicClient, error) {
	clusterName := os.Getenv("KUBERNETES_CLUSTER_NAME")
	if clusterName == "" {
		return nil, fmt.Errorf("KUBERNETES_CLUSTER_NAME environment variable not set")
	}

	region := utils.GetenvDefault("AWS_REGION", "ap-southeast-2")

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("error creating AWS session: %w", err)
	}

	gen, err := token.NewGenerator(true, false)
	if err != nil {
		return nil, fmt.Errorf("error creating token generator: %w", err)
	}

	stsClient := sts.NewFromConfig(cfg)
	result, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, err
	}
	accountId := *result.Account
	fmt.Print(accountId)

	opts := &token.GetTokenOptions{
		ClusterID: clusterName,
		Region:    region,
	}
	tok, err := gen.GetWithOptions(opts)

	if err != nil {
		return nil, fmt.Errorf("error generating token: %w", err)
	}

	eksClient := eks.NewFromConfig(cfg)
	out, err := eksClient.DescribeCluster(ctx, &eks.DescribeClusterInput{
		Name: &clusterName,
	})
	if err != nil {
		return nil, err
	}
	caBase64 := *out.Cluster.CertificateAuthority.Data
	ca, err := base64.StdEncoding.DecodeString(caBase64)
	if err != nil {
		return nil, err
	}

	config := &rest.Config{
		Host:        os.Getenv("KUBERNETES_SERVICE_HOST"),
		BearerToken: tok.Token,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: ca,
		},
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating dynamic Kubernetes client: %w", err)
	}

	return dynamicClient, nil
}

type ActionEvent struct {
	Action string `json:"Action"`
}

func handler(ctx context.Context, request ActionEvent) error {
	fmt.Printf("ctx: %v", ctx)
	fmt.Printf("Requested action: %s", request.Action)

	nodePoolName := os.Getenv("KARPENTER_NODEPOOL_NAME")
	if nodePoolName == "" {
		return fmt.Errorf("KARPENTER_NODEPOOL_NAME environment variable not set")
	}

	dynamicClient, err := newDynamicClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %v", err)
	}

	// common const setup
	nodePoolGVR := schema.GroupVersionResource{
		Group:    "karpenter.sh",
		Version:  "v1",
		Resource: "nodepools",
	}

	np, err := dynamicClient.Resource(nodePoolGVR).Get(ctx, nodePoolName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get nodepool %s: %v", nodePoolName, err)
	}

	switch request.Action {
	case "shutdown":
		fmt.Printf("Simulating scaling down nodepool %s\n", nodePoolName)
		err = unstructured.SetNestedField(np.Object, "0", "spec", "limits", "cpu")
		if err != nil {
			return fmt.Errorf("failed to set cpu limit for nodepool %s: %v", nodePoolName, err)
		}

		_, err = dynamicClient.Resource(nodePoolGVR).Update(ctx, np, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update nodepool %s: %v", nodePoolName, err)
		}

		fmt.Printf("Successfully updated nodepool %s to set cpu limit to 0\n", nodePoolName)
	case "startup":
		fmt.Printf("Simulating scale up of nodepool %s\n", nodePoolName)
		cpuLimit := os.Getenv("KARPENTER_NODEPOOL_LIMITS_CPU")
		if cpuLimit == "" {
			fmt.Printf("Environment variable KARPENTER_NODEPOOL_LIMITS_CPU not set - using default 1000")
			cpuLimit = "1000"
		}
		err = unstructured.SetNestedField(np.Object, cpuLimit, "spec", "limits", "cpu")
		if err != nil {
			return fmt.Errorf("failed to set cpu limit for nodepool %s: %v", nodePoolName, err)
		}

		_, err = dynamicClient.Resource(nodePoolGVR).Update(ctx, np, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update nodepool %s: %v", nodePoolName, err)
		}
	}

	// EC2 interaction
	if err := ShutdownEC2Instances(ctx); err != nil {
		return err
	}

	return nil
}

func main() {
	lambda.Start(handler)
}
