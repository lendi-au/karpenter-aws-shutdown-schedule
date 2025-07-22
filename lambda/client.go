package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/lendi-au/karpenter-aws-shutdown-schedule/pkg/utils"
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
