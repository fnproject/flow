// Add an extension method, ValidateX, that can be implemented on protobuf elements.

// Trivially derived from grpc-validator https://github.com/grpc-ecosystem/go-grpc-middleware/validator
// Original copyright 2016 Michal Witkowski (all rights reserved), under Apache 2.0 License.

package grpcx

import (
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

type validatorExtension interface {
	ValidateX() error
}

// UnaryServerInterceptor returns a new unary server interceptors that does extension validation on incoming messages.
//
// Invalid messages will be rejected with `InvalidArgument` before reaching any userspace handlers.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if v, ok := req.(validatorExtension); ok {
			if err := v.ValidateX(); err != nil {
				return nil, grpc.Errorf(codes.InvalidArgument, err.Error())
			}
		}
		return handler(ctx, req)
	}
}

// StreamServerInterceptor returns a new streaming server interceptors that does extension validation incoming messages.
//
// The stage at which invalid messages will be rejected with `InvalidArgument` varies based on the
// type of the RPC. For `ServerStream` (1:m) requests, it will happen before reaching any userspace
// handlers. For `ClientStream` (n:1) or `BidiStream` (n:m) RPCs, the messages will be rejected on
// calls to `stream.Recv()`.
func StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		wrapper := &recvWrapper{stream}
		return handler(srv, wrapper)
	}
}

type recvWrapper struct {
	grpc.ServerStream
}

func (s *recvWrapper) RecvMsg(m interface{}) error {
	if err := s.ServerStream.RecvMsg(m); err != nil {
		return err
	}
	if v, ok := m.(validatorExtension); ok {
		if err := v.ValidateX(); err != nil {
			return grpc.Errorf(codes.InvalidArgument, err.Error())
		}
	}
	return nil
}
