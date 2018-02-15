GOFILES = $(shell find . -name '*.go' -not -path './vendor/*')
GOPACKAGES = $(shell go list ./...  | grep -v /vendor/)
# GOPATH can take multiple values - only grab the first as that's where go get puts stuff
GOLINTPATH =$(shell echo $$GOPATH | sed -e 's/:.*//')/bin/golint
# Just builds
all: myday build

dep: glide.yaml
	glide install --strip-vendor

dep-up:
	glide up --strip-vendor

protos:  model/model.pb.go persistence/testprotos.pb.go

bindata:  model/model.swagger.json
	go-bindata -nometadata -o  model/swagger_file.go -pkg model model/model.swagger.json

vet: $(GOFILES)
	go vet $(GOPACKAGES)

lint: $(GOFILES)
	OK=0; for pkg in $(GOPACKAGES) ; do   echo Running golint $$pkg ;  $(GOLINTPATH) $$pkg  || OK=1 ;  done ; exit $$OK

test:  $(shell find . -name *.go)
	go test -v $(GOPACKAGES)

%.pb.go: %.proto
	# protoc  --proto_path=$(@D) --proto_path=./vendor --go_out=$(@D) 	--govalidators_out=$(@D) $<
	protoc  --proto_path=$(@D) --proto_path=./vendor -I./vendor/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis \
      --go_out=plugins=grpc:$(@D) --govalidators_out=$(@D)  --grpc-gateway_out=logtostderr=true:$(@D)    --swagger_out=logtostderr=true:$(@D) $<


build: $(GOFILES) bindata
	go build -o flow-service

run: build
	GIN_MODE=debug ./flow-service

fmt: $(GOFILES)
	gofmt -w -s $(GOFILES)

myday: test lint vet

COMPLETER_DIR := $(realpath $(dir $(firstword $(MAKEFILE_LIST))))
CONTAINER_COMPLETER_DIR := /go/src/github.com/fnproject/flow

IMAGE_REPO_USER ?= fnproject
IMAGE_NAME ?= flow
IMAGE_VERSION ?= latest
IMAGE_FULL = $(IMAGE_REPO_USER)/$(IMAGE_NAME):$(IMAGE_VERSION)
IMAGE_LATEST = $(IMAGE_REPO_USER)/$(IMAGE_NAME):latest

docker-pull-image-funcy-go:
	docker pull funcy/go:dev

docker-test: docker-pull-image-funcy-go
	docker run --rm -it -v $(COMPLETER_DIR):$(CONTAINER_COMPLETER_DIR) -w $(CONTAINER_COMPLETER_DIR) -e CGO_ENABLED=1 funcy/go:dev sh -c 'go test -v $$(go list ./...  | grep -v /vendor/)'

docker-build-0: docker-pull-image-funcy-go
	docker run --rm -it -v $(COMPLETER_DIR):$(CONTAINER_COMPLETER_DIR) -w $(CONTAINER_COMPLETER_DIR) -e CGO_ENABLED=1 funcy/go:dev go build -o flow-service-docker
	docker build -t $(IMAGE_FULL) -f $(COMPLETER_DIR)/Dockerfile $(COMPLETER_DIR)

docker-build: docker-test docker-pull-image-funcy-go docker-build-0
