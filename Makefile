# VERSION is defined here and placed in the program.
# STAGING should be set to 1 or 0 and defines which docker repo the image is pushed to.
VERSION="0.1.0"
STAGING="1"
REPO_INFO="https://github.com/litespeed-prometheus-exporter/-/tree/master"
TAG=latest

REPO_INFO := git-$(shell git rev-parse --short HEAD)

ifeq (${STAGING}, "0")
.PHONY: all
all: controller package 
else
.PHONY: all
all: controller 
endif

.PHONY: controller
controller:
	echo "Building controller"
	CGO_ENABLED=0 GOOS=linux go mod tidy
	CGO_ENABLED=0 GOOS=linux go build -ldflags \
		"-w -X main.version=${VERSION} -X main.gitRepo=${REPO_INFO}" \
		-o lsws-prometheus-exporter .
	cp lsws-prometheus-exporter dist

.PHONY: package
package:
	echo "Building package"
	VERSION=${VERSION} ./mkdist.sh ${VERSION}

.PHONY: clean
clean:
	rm lsws-prometheus-exporter
	rm lsws-prometheus-exporter.*.tgz

	                              

