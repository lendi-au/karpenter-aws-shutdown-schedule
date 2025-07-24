# Karpenter AWS Shutdown Schedule

A Go-based AWS CDK project that automatically manages Karpenter nodepools on a schedule to optimize AWS costs by shutting down EC2 instances during off-hours and restarting them during business hours.

## Overview

This project deploys an AWS Lambda function that:

- **Shuts down** Karpenter nodepools by setting CPU limits to 0, deleting associated nodeclaims, and terminating EC2 instances
- **Starts up** Karpenter nodepools by restoring CPU limits to allow scaling
- Operates on a configurable schedule using AWS EventBridge Scheduler with timezone support

## Architecture

The solution consists of:
- **Lambda Function** (`lambda/`): A Go-based handler that interacts with the Kubernetes API and AWS EC2.
- **CDK Stack** (`stacks/`): Infrastructure as Code (IaC) to deploy the Lambda function, IAM roles, and EventBridge schedules.
- **Kubernetes Integration**: Uses a dynamic Kubernetes client to manage Karpenter nodepools and nodeclaims.
- **AWS Integration**: Manages EC2 instances tagged with Karpenter nodepool information.

## Prerequisites

- AWS CDK CLI installed globally: `npm install --global aws-cdk`
- Go 1.24+
- AWS credentials configured in your environment.
- An EKS cluster with Karpenter installed.
- The IAM role used by the Lambda function must have the necessary permissions to access the EKS cluster and manage EC2 instances.

## How to Use

### 1. Clone the Repository

```bash
git clone https://github.com/your-username/karpenter-aws-shutdown-schedule.git
cd karpenter-aws-shutdown-schedule
```

### 2. Configure Environment Variables

Create a `.env` file in the root of the project and add the following environment variables:

#### Build-time Configuration

```bash
# The architecture of the Lambda function (arm64 or amd64).
BUILD_ARCH=amd64

# The API server endpoint of your EKS cluster.
KUBERNETES_SERVICE_HOST="https://<your-eks-api-server>"

# The name of your EKS cluster.
KUBERNETES_CLUSTER_NAME="<your-cluster-name>"

# The name of the Karpenter nodepool to manage.
KARPENTER_NODEPOOL_NAME="<your-nodepool-name>"

# The CPU limit to set when scaling up the nodepool.
KARPENTER_NODEPOOL_LIMITS_CPU="1000"
```

#### Deployment Configuration

```bash
# (Optional) For private EKS clusters, provide VPC details.
KARPENTER_VPC_ID="<your-vpc-id>"
KARPENTER_SUBNET="<your-subnet-ids>" # Comma-separated
KARPENTER_SECURITY_GROUP="<your-security-group-id>" # Optional

# (Optional) Provide a pre-existing IAM role for the Lambda function.
LAMBDA_ROLE_ARN="arn:aws:iam::<your-account-id>:role/<your-lambda-role>"

```

#### Runtime Configuration

```bash
# The cron expression for the shutdown schedule.
KARPENTER_NODEPOOL_SHUTDOWN_SCHEDULE="cron(0 22 * * ? *)" # 10 PM daily

# The cron expression for the startup schedule.
KARPENTER_NODEPOOL_STARTUP_SCHEDULE="cron(0 7 ? * MON-FRI *)" # 7 AM on weekdays

# The timezone for the schedules.
KARPENTER_SCHEDULE_TIMEZONE="Australia/Sydney"

# Allows us to toggle on/off the AWS Cloudwatch Event schedule
KARPENTER_SCHEDULE_FUNCTION_STATE="ENABLED"
```

### 3. Build and Deploy

```bash
make deploy
```

This command will build the Lambda function, package it, and deploy the CDK stack to your AWS account.

### 4. Verify the Deployment

- Check the AWS CloudFormation console to ensure the stack was deployed successfully.
- Verify that the Lambda function and EventBridge schedules were created.
- Check the CloudWatch logs for the Lambda function to monitor its execution.

## How It Works

### Shutdown Process

1.  **Scale Down Nodepool**: The Lambda function sets the `spec.limits.cpu` of the target Karpenter nodepool to "0". This prevents Karpenter from provisioning new nodes.
2.  **Delete Nodeclaims**: It then deletes all `nodeclaims` associated with the nodepool. This triggers Karpenter to terminate the corresponding nodes.
3.  **Terminate EC2 Instances**: Finally, it terminates any remaining EC2 instances that are tagged with the nodepool's name.

### Startup Process

1.  **Scale Up Nodepool**: The Lambda function restores the `spec.limits.cpu` of the nodepool to the value defined in the `KARPENTER_NODEPOOL_LIMITS_CPU` environment variable.
2.  **Automatic Scaling**: Karpenter will then automatically provision new nodes as needed to meet the demands of pending pods.

## IAM Permissions

The Lambda function requires the following AWS permissions:

- `ec2:DescribeInstances`
- `ec2:TerminateInstances`
- `eks:DescribeCluster`

Additionally, the IAM role used by the Lambda function must be mapped to a Kubernetes user or group in the `aws-auth` ConfigMap of your EKS cluster. This allows the Lambda function to authenticate with the Kubernetes API server.

Example `aws-auth` ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: aws-auth
  namespace: kube-system
data:
  mapRoles: |
    - rolearn: <your-lambda-role-arn>
      username: karpenter-shutdown-lambda
      groups:
        - system:masters
```

## Troubleshooting

- **Lambda function times out**: Increase the timeout value in `stacks/karpenter-aws-shutdown-schedule.go`.
- **Permission errors**: Ensure the Lambda function's IAM role has the required permissions and is correctly configured in the `aws-auth` ConfigMap.
- **Incorrect schedule**: Double-check the cron expressions and timezone in your `.env` file.

## Useful CDK Commands

- `cdk deploy`: Deploy the stack to your AWS account/region.
- `cdk diff`: Compare the deployed stack with the current state.
- `cdk synth`: Generate the CloudFormation template.
- `cdk destroy`: Remove the deployed stack.