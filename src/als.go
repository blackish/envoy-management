package main

import (
	"context"
	"flag"
	"fmt"
	"net"

	accesslog "github.com/envoyproxy/go-control-plane/envoy/service/accesslog/v3"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	myals "accesslogs"
)

// @title envoy manager REST API
// @version 1.0
// @description envoy manager server
// @BasePath /api/v1

var (
	debug bool

	alsPort uint

	elasticEndpoint string

	elasticUsername string

	elasticPassword string

	grpcMaxConcurrentStreams uint

	workers int
)

func init() {
	flag.BoolVar(&debug, "debug", true, "Use debug logging")
	flag.UintVar(&alsPort, "als", 18090, "Accesslog server port")
	flag.StringVar(&elasticEndpoint, "elastic", "http://10.99.6.19:9200", "Elastic endpoints, comma separated")
	flag.StringVar(&elasticUsername, "username", "", "Elastic usename")
	flag.StringVar(&elasticPassword, "password", "", "Elastic password")
	flag.UintVar(&grpcMaxConcurrentStreams, "grpcmaxstreams", 100000, "grpc max concurrent streams")
	flag.IntVar(&workers, "workers", 10, "Number of elasticsearch workers")
}

//const grpcMaxConcurrentStreams = 100000

// RunAccessLogServer starts an accesslog service.
func RunAccessLogServer(ctx context.Context, als *myals.AccessLogServiceServer, port uint) {
	var grpcOptions []grpc.ServerOption
	grpcOptions = append(grpcOptions, grpc.MaxConcurrentStreams(uint32(grpcMaxConcurrentStreams)))
	grpcServer := grpc.NewServer(grpcOptions...)
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.WithError(err).Fatal("failed to listen")
	}

	accesslog.RegisterAccessLogServiceServer(grpcServer, als)
	log.WithFields(log.Fields{"port": port}).Info("access log server listening")

	go func() {
		if err = grpcServer.Serve(lis); err != nil {
			log.Error(err)
		}
	}()
	<-ctx.Done()

	grpcServer.GracefulStop()
}

func main() {
	flag.Parse()
	if debug {
		log.SetLevel(log.DebugLevel)
	}
	ctx := context.Background()
	log.Printf("Starting log service")

	als := &myals.AccessLogServiceServer{}
	als.Init(elasticEndpoint, elasticUsername, elasticPassword, workers)
	go RunAccessLogServer(ctx, als, alsPort)
	<-ctx.Done()
}
