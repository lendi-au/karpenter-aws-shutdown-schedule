# Karpenter AWS Shutdown Schedule

A Go-based AWS CDK project that automatically manages Karpenter nodepools on a schedule to optimize AWS costs by shutting down EC2 instances during off-hours and restarting them during business hours.

## Overview

This project deploys an AWS Lambda function that:
- **Shuts down** Karpenter nodepools by setting CPU limits to 0, deleting associated nodeclaims, and terminating EC2 instances
- **Starts up** Karpenter nodepools by restoring CPU limits to allow scaling
- Operates on a configurable schedule using AWS EventBridge Scheduler with timezone support

## Architecture

The solution consists of:
- **Lambda Function** (`lambda/`): Go-based handler that interacts with Kubernetes API and AWS EC2
- **CDK Stack** (`stacks/`): Infrastructure as Code to deploy Lambda, IAM roles, and EventBridge schedules
- **Kubernetes Integration**: Uses dynamic client to manage Karpenter nodepools and nodeclaims
- **AWS Integration**: Manages EC2 instances tagged with Karpenter nodepool information

## Prerequisites

- AWS CDK CLI installed globally: `npm install --global aws-cdk`
- Go 1.24+ for building the Lambda function
- AWS credentials configured
- EKS cluster with Karpenter installed
- Appropriate IAM permissions for Lambda to access EKS and EC2

## Configuration

### Build-time Environment Variables

```bash
# Architecture and cluster configuration
BUILD_ARCH=amd64                                    # Lambda architecture (arm64 or amd64)
KUBERNETES_SERVICE_HOST="https://api-server.k8s.io" # EKS API server endpoint
KUBERNETES_CLUSTER_NAME="my-cluster"               # EKS cluster name
KARPENTER_NODEPOOL_NAME="my-nodepool"              # Target Karpenter nodepool name

# Resource limits for startup
KARPENTER_NODEPOOL_LIMITS_CPU="1000"               # CPU limit when scaling up
KARPENTER_NODEPOOL_LIMITS_MEMORY="1000Gi"          # Memory limit when scaling up

# Optional: Additional EC2 instance filtering
KARPENTER_EXTRA_SHUTDOWN_TAG="custom-tag"          # Extra tag for EC2 instance filtering
```

### Deployment Environment Variables

```bash
# VPC Configuration (optional - for private EKS clusters)
KARPENTER_VPC_ID=vpc-xxxxxxxxx                     # VPC ID for Lambda
KARPENTER_SUBNET=subnet-xxxxxxxxx                  # Subnet ID(s), comma-separated
KARPENTER_SECURITY_GROUP=sg-xxxxxxxxx              # Security group ID (optional)

# IAM Configuration (optional)
LAMBDA_ROLE_ARN=arn:aws:iam::xxxx:role/my-role    # Pre-existing Lambda execution role

# Schedule Configuration
KARPENTER_NODEPOOL_SHUTDOWN_SCHEDULE="cron(0 22 * * ? *)"    # Shutdown time (10 PM daily)
KARPENTER_NODEPOOL_STARTUP_SCHEDULE="cron(0 7 ? * MON-FRI *)" # Startup time (7 AM weekdays)
KARPENTER_SCHEDULE_TIMEZONE="Australia/Sydney"               # Schedule timezone
```

## How It Works

### Shutdown Process (`lambda/main.go:44-63`)
1. Sets the target nodepool's CPU limit to "0" via Kubernetes API
2. Deletes all nodeclaims associated with the nodepool (`lambda/deleteNodeClaims.go`)
3. Terminates all EC2 instances tagged with the nodepool name (`lambda/shutdown.go`)

### Startup Process (`lambda/main.go:64-80`)
1. Restores the nodepool's CPU limit to the configured value
2. Karpenter automatically scales up nodes based on pending workloads

### AWS Integration (`lambda/client.go`)
- Uses AWS IAM authenticator to generate EKS tokens
- Creates Kubernetes dynamic client for API operations
- Supports both public and VPC-based EKS clusters

## Build and Deploy

### Building the Lambda Function

```bash
make build
```

This builds the Go Lambda function and places the binary in the `build/` directory.

### Deploying the Infrastructure

```bash
make deploy
```

This builds the Lambda function and deploys the CDK stack to your configured AWS account/region.

### Running Tests

```bash
make test
```

Runs the Go unit tests with necessary environment variables set.

## IAM Permissions

The Lambda function requires the following AWS permissions:
- `ec2:DescribeInstances`
- `ec2:TerminateInstances` 
- `eks:DescribeCluster`

For Kubernetes access, the Lambda execution role must be added to the EKS cluster's aws-auth ConfigMap or have appropriate RBAC permissions.

## Monitoring

The Lambda function logs all operations to CloudWatch Logs. Key log messages include:
- Nodepool scaling operations
- Nodeclaim deletion results
- EC2 instance termination status
- Authentication and API call results

## Cost Optimization

This solution helps reduce AWS costs by:
- Automatically terminating idle EC2 instances during off-hours
- Preventing unnecessary compute charges during nights and weekends
- Allowing precise control over when workloads are available

## Useful CDK Commands

- `cdk deploy` - Deploy the stack to your AWS account/region
- `cdk diff` - Compare deployed stack with current state  
- `cdk synth` - Generate CloudFormation template
- `cdk destroy` - Remove the deployed stack
