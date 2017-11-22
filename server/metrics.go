package server

import (
	"github.com/opentracing/opentracing-go"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/gin-gonic/gin"
)

var (
	httpRequestsServed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "flow_service_http_requests",
		Help: "Total number of frontend service http requests served",
	})
	httpErrorsCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "flow_service_http_errors",
		Help: "Total number of frontend service http engine errors",
	})
)

func init() {
	prometheus.MustRegister(httpRequestsServed)
	prometheus.MustRegister(httpErrorsCounter)
}

// engineMetrics is a Gin middleware that records things like number of errors directly from the http engine.
func engineMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		httpRequestsServed.Inc()
		if len(c.Errors.Errors()) > 0 {
			httpErrorsCounter.Inc()
		}
	}
}

// unaryMetricsInterceptor returns a new unary server interceptor for our own OpenTracing-based metrics.
func unaryMetricsInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		span := opentracing.StartSpan("api")
		span.SetTag("fn_operation", info.FullMethod)
		newContext := opentracing.ContextWithSpan(ctx, span)
		defer span.Finish()
		resp, err := handler(newContext, req)
		return resp, err
	}
}

// streamMetricsInterceptor returns a new streaming server interceptor for our own OpenTracing-based metrics.
func streamMetricsInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		span := opentracing.StartSpan("api")
		span.SetTag("fn_operation", info.FullMethod)
		newContext := opentracing.ContextWithSpan(stream.Context(), span)
		defer span.Finish()
		wrappedStream := grpc_middleware.WrapServerStream(stream)
		wrappedStream.WrappedContext = newContext
		err := handler(srv, wrappedStream)
		return err
	}
}
