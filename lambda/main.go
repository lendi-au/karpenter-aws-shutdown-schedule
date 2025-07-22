package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/edify42/karpenter-aws-shutdown-schedule/pkg/utils"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/aws-iam-authenticator/pkg/token"
)

func newClientset(ctx context.Context) (*kubernetes.Clientset, error) {
	clusterName := os.Getenv("KUBERNETES_CLUSTER_NAME")
	if clusterName == "" {
		return nil, fmt.Errorf("KUBERNETES_CLUSTER_NAME environment variable not set")
	}

	region := utils.GetenvDefault("AWS_REGION", "ap-southeast-2") // lambda knows what region it's in right?

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
	// roleArn := fmt.Sprintf("arn:aws:iam::%s:role/karpenter-ec2-instance-stop-start", accountId)

	opts := &token.GetTokenOptions{
		// AssumeRoleARN: "arn:aws:iam::<account_id>:role/<role-name>", // Consider supporting this via config...
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

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating Kubernetes clientset: %w", err)
	}

	return clientset, nil
}

func handler(ctx context.Context, request events.CloudWatchEvent) error {
	fmt.Printf("ctx: %v", ctx)
	fmt.Printf("request: %v", request)
	// Kubernetes API interaction
	client, err := newClientset(ctx)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %v", err)
	}

	fmt.Print(client)

	nodePoolName := os.Getenv("KARPENTER_NODEPOOL_NAME")
	if nodePoolName == "" {
		return fmt.Errorf("KARPENTER_NODEPOOL_NAME environment variable not set")
	}

	// This is a placeholder for the actual Karpenter API interaction
	// You would need to use the Karpenter client to update the NodePool
	fmt.Printf("Simulating scaling down nodepool %s\n", nodePoolName)

	// EC2 interaction
	if err := ShutdownEC2Instances(ctx); err != nil {
		return err
	}

	return nil
}

func main() {
	lambda.Start(handler)
}
