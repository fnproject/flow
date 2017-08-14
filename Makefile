# Just builds
all: test build

dep: glide.yaml
	glide install --strip-vendor

dep-up:
	glide up --strip-vendor

protos:  model/model.pb.go

test: protos $(shell find . -name *.go)
	go test -v $(shell glide nv)

%.pb.go: %.proto
	protoc  --proto_path=$(@D) --go_out=$(@D) $<

build: protos $(shell find . -name *.go)
	go build -o completer

run: build
	GIN_MODE=debug ./completer

