package main

import (
	az "authz"
	"context"
	drivers "drivers"
	"encoding/json"
	"flag"
	"fmt"
	pb "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc"
	"net"
	"sync"
	"time"
)

var (
	port           uint
	hcPort         uint
	mongoEndpoints string
	driverTimeout  int64
	configLock     sync.RWMutex
	cache          *az.AuthzConfigType
)

const grpcMaxConcurrentStreams = 1000000

func init() {
	flag.UintVar(&port, "port", 18006, "Auth server port")
	flag.StringVar(&mongoEndpoints, "mongodb", "mongodb://10.99.6.19:27017", "Mongodb endpoints")
	flag.UintVar(&hcPort, "healthCheckPort", 8083, "Health check port")
}

func loadConfig(ctx context.Context) (*az.AuthzConfigType, error) {
	ctxTimeout, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)
	newConfig := &az.AuthzConfigType{}
	if cursor, err := drivers.Cli.Database("envoy").Collection("authz").Find(ctxTimeout, bson.M{}); err == nil {
		for cursor.TryNext(ctxTimeout) {
			if err := cursor.Decode(newConfig); err != nil {
				return nil, err
			}
		}
	}
	return newConfig, nil
}

func RunServer(ctx context.Context, server pb.AuthorizationServer, port uint) {
	var grpcOptions []grpc.ServerOption
	grpcOptions = append(grpcOptions, grpc.MaxConcurrentStreams(grpcMaxConcurrentStreams))
	grpcServer := grpc.NewServer(grpcOptions...)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.WithError(err).Fatal("failed to listen")
	}

	pb.RegisterAuthorizationServer(grpcServer, server)
	log.WithFields(log.Fields{"port": port}).Info("authorization server listening")
	go func() {
		if err = grpcServer.Serve(lis); err != nil {
			log.Error(err)
		}
	}()
	<-ctx.Done()

	grpcServer.GracefulStop()
}

func GetLiveStatus(ctx *gin.Context) {
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.JSON(200, gin.H{
		"status": "alive"})
	return
}

func GetLiveConfig(ctx *gin.Context) {
	configLock.RLock()
	body, _ := json.Marshal(cache)
	configLock.RUnlock()
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Data(200, "application/json", body)
	return
}

func main() {
	var err error
	flag.Parse()
	drivers.ConnectTimeout = time.Duration(driverTimeout) * time.Second
	ctx := context.Background()
	drivers.Cli, err = mongo.NewClient(options.Client().ApplyURI(mongoEndpoints))
	err = drivers.Cli.Connect(ctx)
	if err != nil {
		log.Fatal("Cannot connect to mongodb")
	}
	defer drivers.Cli.Disconnect(ctx)
	err = drivers.Cli.Ping(context.TODO(), nil)
	if err != nil {
		log.Fatal(err)
	}
	cache, err := loadConfig(ctx)
	if err != nil {
		log.Fatal("Failed to load config")
	}

	var azServer az.Service
	azServer.SetConfig(cache)

	log.Printf("Starting auth svc")
	go RunServer(ctx, &azServer, port)

	rlCollection := drivers.Cli.Database("envoy").Collection("authz")
	versionStream, err := rlCollection.Watch(ctx, mongo.Pipeline{})
	if err != nil {
		log.Fatal(err)
	}

	r := gin.Default()
	v1 := r.Group("/api/v1/live")
	{
		v1.GET("/status", GetLiveStatus)
		v1.GET("/config", GetLiveConfig)
	}
	go r.Run(fmt.Sprintf(":%d", hcPort))

	for versionStream.Next(ctx) {
		var data bson.M
		if err = versionStream.Decode(&data); err != nil {
			log.Fatal(err)
		}
		//		if data["operationType"].(string) == "insert" || data["operationType"].(string) == "update" || data["operationType"].(string) == "delete" {
		configLock.Lock()
		cache, err = loadConfig(ctx)
		configLock.Unlock()
		if err == nil {
			log.Print("Config updated")
		}
		azServer.SetConfig(cache)
	}
	//	}
}
