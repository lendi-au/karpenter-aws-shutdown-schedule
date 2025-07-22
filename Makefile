build:
	make -C lambda build

deploy: build
	cdk deploy --require-approval never

.PHONY: build