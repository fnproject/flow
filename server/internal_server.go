package server

import (
	"google.golang.org/grpc"
	"github.com/fnproject/flow/model"
	"net"
)

type InternalServer struct {
	grpc   *grpc.Server
	flows  model.FlowServiceServer
	listen string
}

func NewInternalFlowService(flows model.FlowServiceServer, listen string) (*InternalServer, error) {

	s := grpc.NewServer()

	model.RegisterFlowServiceServer(s, flows)

	return &InternalServer{grpc: s, flows: flows, listen: listen}, nil
}

func (s *InternalServer) Run() error {
	lis, err := net.Listen("tcp", s.listen)
	if err != nil {
		return err
	}
	defer lis.Close()


	return s.grpc.Serve(lis)

}
