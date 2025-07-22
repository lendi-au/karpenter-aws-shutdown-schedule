build:
	make -C lambda build

deploy: build
	cdk deploy --require-approval never

test:
	KUBERNETES_CLUSTER_NAME=dummy KUBERNETES_SERVICE_HOST=https://k8s.api KARPENTER_NODEPOOL_SHUTDOWN_SCHEDULE="cron(0 22 * * ? *)" KARPENTER_NODEPOOL_STARTUP_SCHEDULE="cron(0 7 * * ? *)" go test -v ./...

.PHONY: build