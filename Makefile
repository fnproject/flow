GOFILES = $(shell find . -name '*.go' -not -path './vendor/*')
GOPACKAGES = $(shell go list ./...  | grep -v /vendor/)

COMPLETER_DIR := $(realpath $(dir $(firstword $(MAKEFILE_LIST))))

IMAGE_TAG ?= registry.oracledx.com/fnproject-completer:latest

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


docker-test: protos $(shell find . -name *.go)
	docker run --rm -it -v $(COMPLETER_DIR):$(COMPLETER_DIR) -w $(COMPLETER_DIR) -e GOPATH=$(GOPATH) -e GOOS=linux -e GOARCH=amd64 -e CGO_ENABLED=1 golang go test -v $(GOPACKAGES)

docker-build:  $(GOFILES)
	docker run --rm -it -v $(COMPLETER_DIR):$(COMPLETER_DIR) -w $(COMPLETER_DIR) -e GOPATH=$(GOPATH) -e GOOS=linux -e GOARCH=amd64 -e CGO_ENABLED=1 golang go build -o completer

docker: docker-test docker-build
	docker build -t $(IMAGE_TAG) -f $(COMPLETER_DIR)/Dockerfile $(COMPLETER_DIR)
