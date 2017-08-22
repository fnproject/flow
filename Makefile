GOFILES = $(shell find . -name '*.go' -not -path './vendor/*')
GOPACKAGES = $(shell go list ./...  | grep -v /vendor/)

# Just builds
all: test build

dep: glide.yaml
	glide install --strip-vendor

dep-up:
	glide up --strip-vendor

protos:  model/model.pb.go persistence/testprotos.pb.go

test: protos $(shell find . -name *.go)
	@go test -v $(GOPACKAGES)

%.pb.go: %.proto
	protoc  --proto_path=$(@D) --go_out=$(@D) $<

build:  $(GOFILES)
	go build -o completer

run: build
	GIN_MODE=debug ./completer


COMPLETER_DIR := $(realpath $(dir $(firstword $(MAKEFILE_LIST))))

IMAGE_REPO_USER ?= fnproject
IMAGE_NAME ?= completer
IMAGE_VERSION ?= latest
IMAGE_FULL = $(IMAGE_REPO_USER)/$(IMAGE_NAME):$(IMAGE_VERSION)
IMAGE_LATEST = $(IMAGE_REPO_USER)/$(IMAGE_NAME):latest

docker-test: protos $(shell find . -name *.go)
	docker pull funcy/go:dev
	docker run --rm -it -v $(COMPLETER_DIR):$(COMPLETER_DIR) -w $(COMPLETER_DIR) -e GOPATH=$(GOPATH) -e GOOS=linux -e GOARCH=amd64 -e CGO_ENABLED=1 funcy/go:dev go test -v $(GOPACKAGES)

docker-build: $(GOFILES) docker-test
	docker pull funcy/go:dev
	docker run --rm -it -v $(COMPLETER_DIR):$(COMPLETER_DIR) -w $(COMPLETER_DIR) -e GOPATH=$(GOPATH) -e GOOS=linux -e GOARCH=amd64 -e CGO_ENABLED=1 funcy/go:dev go build -o completer-docker
	docker build -t $(IMAGE_FULL) -f $(COMPLETER_DIR)/Dockerfile $(COMPLETER_DIR)
