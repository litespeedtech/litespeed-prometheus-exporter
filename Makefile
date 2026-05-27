# VERSION is defined here and placed in the program.
# STAGING should be set to 1 or 0 and defines which docker repo the image is pushed to.
VERSION="0.2.1"
STAGING="0"
REPO_INFO="https://github.com/litespeed-prometheus-exporter/-/tree/master"
TAG=latest

REPO_INFO := git-$(shell git rev-parse --short HEAD)

# Pin the Go toolchain so a builder with an older `go` (e.g. go1.22 on
# Ubuntu 22.04) auto-downloads a published Go 1.25 patch release instead
# of trying — and failing — to fetch the un-published bare `go1.25`.
# Override on the command line if you want a different patch:
#   GOTOOLCHAIN=go1.25.11 make all
GOTOOLCHAIN ?= go1.25.10
export GOTOOLCHAIN

ifeq (${STAGING}, "0")
.PHONY: all
all: controller package
else
.PHONY: all
all: controller
endif

.PHONY: controller
controller:
	echo "Building controller (GOTOOLCHAIN=$(GOTOOLCHAIN))"
	CGO_ENABLED=0 GOOS=linux go mod tidy
	CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags \
		"-w -s -X main.version=${VERSION} -X main.gitRepo=${REPO_INFO}" .
	cp litespeed-prometheus-exporter dist/lsws-prometheus-exporter

.PHONY: package
package:
	echo "Building package"
	VERSION=${VERSION} ./mkdist.sh ${VERSION}

.PHONY: clean
clean:
	rm lsws-prometheus-exporter
	rm lsws-prometheus-exporter.*.tgz

	                              

