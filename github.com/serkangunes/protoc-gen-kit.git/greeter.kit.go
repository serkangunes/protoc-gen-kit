// Code generated by protoc-gen-kit. DO NOT EDIT.
// source: examples/greeter/greeter.proto

package protoc_gen_kit_git

import (
	fmt "fmt"
	math "math"

	proto "github.com/golang/protobuf/proto"
)

import (
	context "context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	kitendpoint "github.com/go-kit/kit/endpoint"
	kitlog "github.com/go-kit/kit/log"
	kitgrpc "github.com/go-kit/kit/transport/grpc"
	"github.com/oklog/oklog/pkg/group"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context

//////////////////////////////////////////////////////////
// Go-kit middlewares for Greeter service
//////////////////////////////////////////////////////////

type Middleware func(GreeterServer) GreeterServer

type loggingMiddleware struct {
	logger kitlog.Logger
	next   GreeterServer
}

// LoggingMiddleware takes a logger as a dependency
// and returns a GreeterServer Middleware.
func LoggingMiddleware(logger kitlog.Logger) Middleware {
	return func(next GreeterServer) GreeterServer {
		return &loggingMiddleware{logger, next}
	}
}
func (l loggingMiddleware) Hello(ctx context.Context, request *HelloRequest) (response *HelloResponse, err error) {
	defer func(begin time.Time) {
		l.logger.Log(
			"method", "Hello",
			"request", request,
			"response", response,
			"error", err,
			"took", time.Since(begin))
	}(time.Now())
	return l.next.Hello(ctx, request)
}
func (l loggingMiddleware) Goodbye(ctx context.Context, request *GoodbyeRequest) (response *GoodbyeResponse, err error) {
	defer func(begin time.Time) {
		l.logger.Log(
			"method", "Goodbye",
			"request", request,
			"response", response,
			"error", err,
			"took", time.Since(begin))
	}(time.Now())
	return l.next.Goodbye(ctx, request)
}

////////////////////////////////////////////////////////
// Go-kit endpoints for Greeter service
////////////////////////////////////////////////////////

//Endpoints stores all the enpoints of the service
type Endpoints struct {
	HelloEndpoint   kitendpoint.Endpoint
	GoodbyeEndpoint kitendpoint.Endpoint
}

func makeHelloEndpoint(handler GreeterServer) kitendpoint.Endpoint {
	return func(ctx context.Context, r interface{}) (interface{}, error) {
		request := r.(*HelloRequest)
		response, err := handler.Hello(ctx, request)
		return response, err
	}
}

func makeGoodbyeEndpoint(handler GreeterServer) kitendpoint.Endpoint {
	return func(ctx context.Context, r interface{}) (interface{}, error) {
		request := r.(*GoodbyeRequest)
		response, err := handler.Goodbye(ctx, request)
		return response, err
	}
}

// New returns a Endpoints struct that wraps the provided service, and wires in all of the
// expected endpoint middlewares
func NewEndpoints(handler GreeterServer, middlewares map[string][]kitendpoint.Middleware) Endpoints {
	endpoints := Endpoints{
		HelloEndpoint:   makeHelloEndpoint(handler),
		GoodbyeEndpoint: makeGoodbyeEndpoint(handler),
	}

	for _, middleware := range middlewares["Hello"] {
		endpoints.HelloEndpoint = middleware(endpoints.HelloEndpoint)
	}

	for _, middleware := range middlewares["Goodbye"] {
		endpoints.GoodbyeEndpoint = middleware(endpoints.GoodbyeEndpoint)
	}

	return endpoints
}

/////////////////////////////////////////////////////////////
// Go-kit grpc transport for Greeter service
/////////////////////////////////////////////////////////////

//RequestDecoder empty request decoder just returns the same request
func RequestDecoder(ctx context.Context, r interface{}) (interface{}, error) {
	return r, nil
}

//ResponseEncoder empty response encoder just returns the same response
func ResponseEncoder(_ context.Context, r interface{}) (interface{}, error) {
	return r, nil
}

type grpcServer struct {
	hellotransport   kitgrpc.Handler
	goodbyetransport kitgrpc.Handler
}

// implement GreeterServer Interface
//Hello implementation
func (s *grpcServer) Hello(ctx context.Context, r *HelloRequest) (*HelloResponse, error) {
	_, response, err := s.hellotransport.ServeGRPC(ctx, r)
	if err != nil {
		return nil, err
	}
	return response.(*HelloResponse), nil
}

//Goodbye implementation
func (s *grpcServer) Goodbye(ctx context.Context, r *GoodbyeRequest) (*GoodbyeResponse, error) {
	_, response, err := s.goodbyetransport.ServeGRPC(ctx, r)
	if err != nil {
		return nil, err
	}
	return response.(*GoodbyeResponse), nil
}

//NewGRPCServer create new grpc server
func NewGRPCServer(endpoints Endpoints, options map[string][]kitgrpc.ServerOption) GreeterServer {
	return &grpcServer{
		hellotransport: kitgrpc.NewServer(
			endpoints.HelloEndpoint,
			RequestDecoder,
			ResponseEncoder,
			options["Hello"]...,
		),
		goodbyetransport: kitgrpc.NewServer(
			endpoints.GoodbyeEndpoint,
			RequestDecoder,
			ResponseEncoder,
			options["Goodbye"]...,
		),
	}
}

/////////////////////////////////////////////////////////////////////
// Go-kit grpc main helper functions Greeter service
/////////////////////////////////////////////////////////////////////

func RunServer(logger, errorLogger kitlog.Logger, grpcAddr, debugAddr string, handler GreeterServer) {
	endpoints := NewEndpoints(handler, nil)
	group := createService(endpoints, logger, errorLogger, grpcAddr)
	initMetricsEndpoint(debugAddr, logger, errorLogger, group)
	initCancelInterrupt(group)
	logger.Log("exit", group.Run())
}

func Client(address string, insecure bool, timeoutInSeconds time.Duration) (GreeterClient, *grpc.ClientConn, error) {
	var conn *grpc.ClientConn
	var err error
	if insecure {
		conn, err = grpc.Dial(address, grpc.WithInsecure(), grpc.WithTimeout(timeoutInSeconds*time.Second))
	} else {
		conn, err = grpc.Dial(address, grpc.WithTimeout(timeoutInSeconds*time.Second))
	}

	if err != nil {
		return nil, nil, err
	}
	return NewGreeterClient(conn), conn, nil
}

func GetServiceMiddlewares(logger kitlog.Logger) (middlewares []Middleware) {
	middlewares = []Middleware{}
	return append(middlewares, LoggingMiddleware(logger))
}

func createService(endpoints Endpoints, logger, errorLogger kitlog.Logger, grpcAddr string) (g *group.Group) {
	g = &group.Group{}
	initGRPCHandler(endpoints, logger, errorLogger, grpcAddr, g)
	return g
}

func defaultGRPCOptions(errorLogger kitlog.Logger) map[string][]kitgrpc.ServerOption {
	options := map[string][]kitgrpc.ServerOption{
		"Hello":   {kitgrpc.ServerErrorLogger(errorLogger)},
		"Goodbye": {kitgrpc.ServerErrorLogger(errorLogger)},
	}
	return options
}

func initGRPCHandler(endpoints Endpoints, logger, errorLogger kitlog.Logger, grpcAddr string, g *group.Group) {
	options := defaultGRPCOptions(errorLogger)

	grpcServer := NewGRPCServer(endpoints, options)
	grpcListener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		errorLogger.Log("transport", "gRPC", "during", "Listen", "err", err)
	}
	g.Add(func() error {
		logger.Log("transport", "gRPC", "addr", grpcAddr)
		baseServer := grpc.NewServer()
		RegisterGreeterServer(baseServer, grpcServer)
		return baseServer.Serve(grpcListener)
	}, func(error) {
		grpcListener.Close()
	})
}

func initMetricsEndpoint(debugAddr string, logger, errorLogger kitlog.Logger, g *group.Group) {
	http.DefaultServeMux.Handle("/metrics", promhttp.Handler())
	debugListener, err := net.Listen("tcp", debugAddr)
	if err != nil {
		errorLogger.Log("transport", "debug/HTTP", "during", "Listen", "err", err)
	}
	g.Add(func() error {
		logger.Log("transport", "debug/HTTP", "addr", debugAddr)
		return http.Serve(debugListener, http.DefaultServeMux)
	}, func(error) {
		debugListener.Close()
	})
}

func initCancelInterrupt(g *group.Group) {
	cancelInterrupt := make(chan struct{})
	g.Add(func() error {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		select {
		case sig := <-c:
			return fmt.Errorf("received signal %s", sig)
		case <-cancelInterrupt:
			return nil
		}
	}, func(error) {
		close(cancelInterrupt)
	})
}
