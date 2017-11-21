package server

import (
	"github.com/sirupsen/logrus"
	"github.com/fnproject/flow/cluster"
	"context"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"google.golang.org/grpc"
	"github.com/fnproject/flow/model"
	"net/http"
	"time"
	"github.com/gin-gonic/gin"
	"github.com/openzipkin/zipkin-go-opentracing"
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"strings"
	"net"
    "github.com/grpc-ecosystem/go-grpc-middleware/validator"

)

const (
	maxDelayStageDelay = 3600 * 1000 * 24
	maxRequestTimeout  = 1 * time.Hour
	minRequestTimeout  = 1 * time.Second
)

var log = logrus.WithField("logger", "server")

func (s *Server) handlePrometheusMetrics(c *gin.Context) {
	s.promHandler.ServeHTTP(c.Writer, c.Request)
}

func setTracer(ownURL string, zipkinURL string) {
	var (
		debugMode          = false
		serviceName        = "flow-service"
		serviceHostPort    = ownURL
		zipkinHTTPEndpoint = zipkinURL
		// ex: "http://zipkin:9411/api/v1/spans"
	)

	var collector zipkintracer.Collector

	// custom Zipkin collector to send tracing spans to Prometheus
	promCollector, promErr := NewPrometheusCollector()
	if promErr != nil {
		logrus.WithError(promErr).Fatalln("couldn't start Prometheus trace collector")
	}

	logger := zipkintracer.LoggerFunc(func(i ...interface{}) error { logrus.Error(i...); return nil })

	if zipkinHTTPEndpoint != "" {
		// Custom PrometheusCollector and Zipkin HTTPCollector
		httpCollector, zipErr := zipkintracer.NewHTTPCollector(zipkinHTTPEndpoint, zipkintracer.HTTPLogger(logger))
		if zipErr != nil {
			logrus.WithError(zipErr).Fatalln("couldn't start Zipkin trace collector")
		}
		collector = zipkintracer.MultiCollector{httpCollector, promCollector}
	} else {
		// Custom PrometheusCollector only
		collector = promCollector
	}

	ziptracer, err := zipkintracer.NewTracer(zipkintracer.NewRecorder(collector, debugMode, serviceHostPort, serviceName),
		zipkintracer.ClientServerSameSpan(true),
		zipkintracer.TraceID128Bit(true),
	)
	if err != nil {
		logrus.WithError(err).Fatalln("couldn't start tracer")
	}

	// wrap the Zipkin tracer in a FnTracer which will also send spans to Prometheus
	fntracer := NewFnTracer(ziptracer)

	opentracing.SetGlobalTracer(fntracer)
	logrus.WithFields(logrus.Fields{"url": zipkinHTTPEndpoint}).Info("started tracer")
}

// Server is the flow service root
type Server struct {
	Engine         *gin.Engine
	listen         string
	grpcServer     *grpc.Server
	clusterManager *cluster.Manager
	promHandler    http.Handler
}

func serverGrpc(server *grpc.Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		r := c.Request
		w := c.Writer
		log.Infof(" proto: %s %d %s %s", r.Method, r.ProtoMajor, r.Header.Get("Content-type"), r.RequestURI)

		if r.ProtoMajor == 2 && (r.Method == "PRI" || strings.Contains(r.Header.Get("Content-Type"), "application/grpc")) {
			log.Info("Serving GRPC ")
			server.ServeHTTP(w, r)
		} else {
			c.Next()
		}
	}
}

// New creates a new server - params are injected dependencies
func NewAPIServer(clusterManager *cluster.Manager, listenAddress string, zipkinURL string) (*Server, error) {

	setTracer(listenAddress, zipkinURL)

	gRPCServer := grpc.NewServer(validator.U)
	proxySvc := cluster.NewClusterProxy(clusterManager)
	model.RegisterFlowServiceServer(gRPCServer, proxySvc)

	// TODO make GRPC port configurable
	l, err := net.Listen("tcp", "localhost:9999")
	if err != nil {
		return nil, err
	}
	go func() {
		gRPCServer.Serve(l)
	}()

	engine := gin.New()
	engine.Use(gin.Logger(), gin.Recovery(), serverGrpc(gRPCServer))

	s := &Server{
		Engine:         engine,
		listen:         listenAddress,
		grpcServer:     gRPCServer,
		clusterManager: clusterManager,
		promHandler:    promhttp.Handler(),
	}

	s.Engine.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	gwmux := runtime.NewServeMux()
	model.RegisterFlowServiceHandlerFromEndpoint(context.Background(),   gwmux, "localhost:9999", []grpc.DialOption{grpc.WithInsecure()})

	s.Engine.Any("/v1/*path", func(c *gin.Context) {
		log.Info("Serving HTTP ")

		gwmux.ServeHTTP(c.Writer, c.Request)
	})

	// TODO: fix wss/change to gRPC!
	//s.Engine.GET("/wss", func(c *gin.Context) {
	//	query.WSSHandler(manager, c.Writer, c.Request)
	//})

	s.Engine.GET("/metrics", s.handlePrometheusMetrics)

	return s, nil

}

// Run starts the server
func (s *Server) Run() error {

	return s.Engine.Run(s.listen)
}
