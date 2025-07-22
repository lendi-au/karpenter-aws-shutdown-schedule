BUILD_ARCH ?= "arm64"

build:
	GOOS=linux GOARCH=$(BUILD_ARCH) go build -o bootstrap *.go
	mkdir -p ../build/
	mv bootstrap ../build/

clean:
	rm -rf bootstrap ../build

.PHONY: build clean