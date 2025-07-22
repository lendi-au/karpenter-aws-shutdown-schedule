# karpenter-aws-shutdown-schedule

Vibe-coded (mostly) project which deploys an AWS lambda.

As this is a golang package,
but cdk is usually javascript/typescript tools, it's better to have the cdk
deployed globally like so:

```bash

npm install --global aws-cdk

```

Important to note that you need to have running karpenter pods for the startup
behaviour to function (where I'm using this there's a separate Auto Scaling
group for this).

Also the intent of this function is to be a 1:1 mapping with a specific
nodegroup in k8s.

**This function needs access to the k8s API to update the nodepool settings**

## shutdown behaviour

1. Set the configured node pools to 0
2. Find all nodes from the node pool and force the drain
3. Terminate all EC2 instances via AWS API

## startup behaviour

The startup behaviour is much simpler, with karpenter setting the desired node number depending on the environment variable set.

## build


``
If defined, we use this role by the lambda instead of creating one.

Set on the node group when it turns on again. This is done at build time as we
setup the AWS Eventbridge schedule.

```bash
# cat .env
BUILD_ARCH=amd64 # build amd64 golang binary instead
KUBERNETES_SERVICE_HOST="https://api-server.k8s.io" # k8s master API endpoint
KUBERNETES_CLUSTER_NAME="my-cluster"
KARPENTER_NODEPOOL_NAME="KARPENTER_DYNAMIC" # karpenter node pool name
KARPENTER_EXTRA_SHUTDOWN_TAG= # optional extra tag to search for karpenter nodes. checks only for existence.
KARPENTER_NODEPOOL_LIMITS_CPU="1000" # number of CPU cores karpenter can scale up to when it turns on the nodes again
KARPENTER_NODEPOOL_LIMITS_MEMORY="1000Gi" # number of memory karpenter should

```

## deploy

From the root directory, run `cdk deploy`. You should setup these environment
variables so that the lambda has the right config at runtime (not the default
values) to target what it needs:

```bash

cat .env
KARPENTER_VPC_ID=vpc-03434234234 # specify a VPC ID, subnet + security group if your EKS cluster is internal
KARPENTER_SUBNET=subnet-34234234
KARPENTER_SECURITY_GROUP=sg-1232323
LAMBDA_ROLE_ARN=arn:aws:iam::2231231231231:role/my-custom-karpenter-role # custom lambda role predefined and not created here

KARPENTER_NODEPOOL_SHUTDOWN_SCHEDULE="cron(0 22 * * ? *)" # When to shutdown the lambda (10pm every day)
KARPENTER_NODEPOOL_STARTUP_SCHEDULE="cron(0 7 ? * MON-FRI *)" # When the lambda should run and start things UP (7:00am Monday to Friday)
KARPENTER_SCHEDULE_TIMEZONE="Australia/Sydney" # Timezone - make it useful!
```

## cdk init stuff

The `cdk.json` file tells the CDK toolkit how to execute your app.

## Useful commands

 * `cdk deploy`      deploy this stack to your default AWS account/region
 * `cdk diff`        compare deployed stack with current state
 * `cdk synth`       emits the synthesized CloudFormation template
 * `go test`         run unit tests
