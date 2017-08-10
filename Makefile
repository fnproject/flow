# Just builds
.PHONY: all test dep build test-log-datastore

dep:
	glide install --strip-vendor

dep-up:
	glide up --strip-vendor

protos:  model/model.pb.go

%.pb.go: %.proto
	protoc  --proto_path=$(@D) --go_out=$(@D) $<

build: $(find . -name *.go)
	go build -o completer

run: build
	GIN_MODE=debug ./completer

all: dep protos build
