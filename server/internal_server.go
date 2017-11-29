package server

import (
	"google.golang.org/grpc"
	"github.com/fnproject/flow/model"
	"net"
)

//InternalServer handles the node-local service (servicing graphs on this node only)
type InternalServer struct {
	grpc   *grpc.Server
	flows  model.FlowServiceServer
	listen string
}

// NewInternalFlowService creates a new flow service for a given graph API
func NewInternalFlowService(flows model.FlowServiceServer, listen string) (*InternalServer, error) {

	s := grpc.NewServer()

	model.RegisterFlowServiceServer(s, flows)

	return &InternalServer{grpc: s, flows: flows, listen: listen}, nil
}

//Run starts the server and blocks  until it closes
func (s *InternalServer) Run() error {
	lis, err := net.Listen("tcp", s.listen)
	if err != nil {
		return err
	}
	defer lis.Close()

	log.WithField("listen",s.listen).Info("Serving shard server")
	return s.grpc.Serve(lis)

}
