package main

import (
	"context"
	drivers "drivers"
	"encoding/json"
	"flag"
	"fmt"
	pb "github.com/envoyproxy/go-control-plane/envoy/service/ratelimit/v3"
	units "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc"
	"net"
	rl "ratelimit"
	"strings"
	"sync"
	"time"
)

var (
	port           uint
	hcPort         uint
	mongoEndpoints string
	redisEndpoints string
	redisAuth      string
	redisTls       bool
	redisType      string
	redisPoolSize  int
	driverTimeout  int64
	cache          *rl.CacheConfig
	configLock     sync.RWMutex
)

const grpcMaxConcurrentStreams = 1000000

func init() {
	flag.UintVar(&port, "port", 18000, "Ratelimit server port")
	flag.StringVar(&mongoEndpoints, "mongodb", "mongodb://10.99.6.19:27017", "Mongodb endpoints")
	flag.StringVar(&redisEndpoints, "redis", "127.0.0.1:6379", "Redis")
	flag.Int64Var(&driverTimeout, "timeout", 5, "Mongodb connection timeout")
	flag.IntVar(&redisPoolSize, "redisPool", 10, "Redis pool size")
	flag.BoolVar(&redisTls, "redisTls", false, "Use redis with TLS")
	flag.StringVar(&redisAuth, "redisAuth", "", "Redis auth")
	flag.StringVar(&redisType, "redisType", "single", "Redis single/cluster")
	flag.UintVar(&hcPort, "healthCheckPort", 8081, "Health check port")
}

func loadConfig(ctx context.Context) (*rl.CacheConfig, error) {
	var builder strings.Builder
	ctxTimeout, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)
	newConfig := &rl.RLConfig{}
	newConfig.Entries = make([]rl.RLEntry, 0)
	cacheConfig := &rl.CacheConfig{}
	cacheConfig.Domains = map[string]*rl.CacheDescriptors{}
	if cursor, err := drivers.Cli.Database("envoy").Collection("ratelimits").Find(ctxTimeout, bson.M{}); err == nil {
		for cursor.TryNext(ctxTimeout) {
			var domain rl.RLEntry
			if err := cursor.Decode(&domain); err == nil {
				newConfig.Entries = append(newConfig.Entries, domain)
			}
		}
		cursor.Close(ctxTimeout)
		for _, entry := range newConfig.Entries {
			builder.Reset()
			builder.WriteString(entry.Key)
			builder.WriteString(entry.Value)
			if unit, ok := units.RateLimitUnit_value[strings.ToUpper(entry.Unit)]; ok {
				if units.RateLimitUnit(unit) != units.RateLimitUnit_UNKNOWN {
					if _, ok := cacheConfig.Domains[entry.Domain]; !ok {
						cacheConfig.Domains[entry.Domain] = new(rl.CacheDescriptors)
						cacheConfig.Domains[entry.Domain].Entries = map[string]*rl.CacheEntry{}
						cacheConfig.Domains[entry.Domain].StripDomain = entry.StripDomain
					}
					cacheConfig.Domains[entry.Domain].Entries[builder.String()] = &rl.CacheEntry{Limit: entry.Limit, Unit: units.RateLimitUnit(unit)}
				}
			}
		}
		return cacheConfig, nil
	} else {
		return nil, err
	}
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

func RunServer(ctx context.Context, server pb.RateLimitServiceServer, port uint) {
	var grpcOptions []grpc.ServerOption
	grpcOptions = append(grpcOptions, grpc.MaxConcurrentStreams(grpcMaxConcurrentStreams))
	grpcServer := grpc.NewServer(grpcOptions...)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.WithError(err).Fatal("failed to listen")
	}

	pb.RegisterRateLimitServiceServer(grpcServer, server)
	log.WithFields(log.Fields{"port": port}).Info("ratelimit server listening")
	go func() {
		if err = grpcServer.Serve(lis); err != nil {
			log.Error(err)
		}
	}()
	<-ctx.Done()

	grpcServer.GracefulStop()
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
	redisClient, err := drivers.NewClient(redisEndpoints, redisTls, redisAuth, redisPoolSize, redisType)
	if err != nil {
		log.Fatal("Cannot connect to redis")
	}
	cache, err = loadConfig(ctx)
	if err != nil {
		log.Fatal("Failed to load config")
	}

	var rlServer rl.Service
	rlServer.Init(cache, redisClient)

	log.Printf("Starting ratelimit svc")
	go RunServer(ctx, &rlServer, port)

	rlCollection := drivers.Cli.Database("envoy").Collection("ratelimits")
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
		if data["operationType"].(string) == "insert" || data["operationType"].(string) == "update" || data["operationType"].(string) == "delete" {
			configLock.Lock()
			cache, err = loadConfig(ctx)
			configLock.Unlock()
			if err == nil {
				log.Print("Config updated")
			}
			rlServer.SetConfig(cache)
		}
	}
}
