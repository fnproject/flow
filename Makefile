GOFILES = $(shell find . -name '*.go' -not -path './vendor/*')
GOPACKAGES = $(shell go list ./...  | grep -v /vendor/)

# Just builds
all: test build

dep: glide.yaml
	glide install --strip-vendor

dep-up:
	glide up --strip-vendor

protos:  model/model.pb.go persistence/testprotos.pb.go

vet: $(GOFILES)
	go vet $(GOPACKAGES)

test: protos $(shell find . -name *.go)
	go test -v $(GOPACKAGES)

%.pb.go: %.proto
	protoc  --proto_path=$(@D) --go_out=$(@D) $<

build: $(GOFILES)
	go build -o flow-service

run: build
	GIN_MODE=debug ./flow-service


COMPLETER_DIR := $(realpath $(dir $(firstword $(MAKEFILE_LIST))))
CONTAINER_COMPLETER_DIR := /go/src/github.com/fnproject/flow

IMAGE_REPO_USER ?= fnproject
IMAGE_NAME ?= flow
IMAGE_VERSION ?= latest
IMAGE_FULL = $(IMAGE_REPO_USER)/$(IMAGE_NAME):$(IMAGE_VERSION)
IMAGE_LATEST = $(IMAGE_REPO_USER)/$(IMAGE_NAME):latest

docker-pull-image-funcy-go:
	docker pull funcy/go:dev

docker-test: protos docker-pull-image-funcy-go
	docker run --rm -it -v $(COMPLETER_DIR):$(CONTAINER_COMPLETER_DIR) -w $(CONTAINER_COMPLETER_DIR) -e CGO_ENABLED=1 funcy/go:dev sh -c 'go test -v $$(go list ./...  | grep -v /vendor/)'

docker-build: docker-test docker-pull-image-funcy-go
	docker run --rm -it -v $(COMPLETER_DIR):$(CONTAINER_COMPLETER_DIR) -w $(CONTAINER_COMPLETER_DIR) -e CGO_ENABLED=1 funcy/go:dev go build -o flow-service-docker
	docker build -t $(IMAGE_FULL) -f $(COMPLETER_DIR)/Dockerfile $(COMPLETER_DIR)
