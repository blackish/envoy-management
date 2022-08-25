package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"time"

	_ "docs"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	"github.com/swaggo/gin-swagger"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	xds "github.com/envoyproxy/go-control-plane/pkg/server/v3"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"

	drivers "drivers"
	myload "loader"
	rapi "rapi"
	apitypes "types"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// @title envoy manager REST API
// @version 1.0
// @description envoy manager server
// @BasePath /api/v1

var (
	debug bool

	port        uint
	gatewayPort uint

	mode string

	mongoEndpoints string

	driverTimeout int64

	config  cache.SnapshotCache
	workers *mongo.Collection
)

const (
	XdsCluster = "xds_cluster"
	Ads        = "ads"
	Xds        = "xds"
	Rest       = "rest"
)

func init() {
	flag.BoolVar(&debug, "debug", true, "Use debug logging")
	flag.UintVar(&port, "port", 18000, "Management server port")
	flag.UintVar(&gatewayPort, "gateway", 18001, "Management server port for HTTP gateway")
	flag.StringVar(&mode, "ads", Ads, "Management server type (ads, xds, rest)")
	flag.StringVar(&mongoEndpoints, "mongodb", "mongodb://10.99.6.19:27017", "Mongodb endpoints")
	flag.Int64Var(&driverTimeout, "timeout", 5, "Mongodb connection timeout")
}

type logger struct{}

func (logger logger) Infof(format string, args ...interface{}) {
	log.Infof(format, args...)
}
func (logger logger) Errorf(format string, args ...interface{}) {
	log.Errorf(format, args...)
}
func (logger logger) Debugf(format string, args ...interface{}) {
	log.Errorf(format, args...)
}
func (cb *callbacks) OnStreamOpen(ctx context.Context, id int64, typ string) error {
	log.Debugf("OnStreamOpen %d open for %s", id, typ)
	return nil
}
func (cb *callbacks) OnStreamClosed(id int64) {
	log.Debugf("OnStreamClosed %d closed", id)
}
func (cb *callbacks) OnStreamRequest(i int64, req *discovery.DiscoveryRequest) error {
	log.Debugf("OnStreamRequest %s", req.Node.Id)
	ups := &options.UpdateOptions{}
	ctx := context.Background()
	ctxTimeout, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)
	_, _ = drivers.Cli.Database("envoy").Collection("workers").UpdateOne(ctxTimeout, bson.M{"name": req.Node.Id}, bson.M{"$set": bson.M{"version": req.VersionInfo, "status": req.ErrorDetail.String(), "updated": time.Now().Format("Mon Jan 2 15:04:05 MST 2006")}}, ups.SetUpsert(true))
	return nil
}
func (cb *callbacks) OnStreamResponse(i int64, req *discovery.DiscoveryRequest, res *discovery.DiscoveryResponse) {
	log.Debugf("OnStreamResponse...")
}
func (cb *callbacks) OnFetchRequest(ctx context.Context, req *discovery.DiscoveryRequest) error {
	log.Debugf("OnFetchRequest %s", req.Node.Id)
	ups := &options.UpdateOptions{}
	ctxTimeout, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)
	_, _ = drivers.Cli.Database("envoy").Collection("workers").UpdateOne(ctxTimeout, bson.M{"name": req.Node.Id}, bson.M{"$set": bson.M{"version": req.VersionInfo, "status": req.ErrorDetail.String(), "updated": time.Now().Format("Mon Jan 2 15:04:05 MST 2006")}}, ups.SetUpsert(true))
	return nil
}
func (cb *callbacks) OnFetchResponse(*discovery.DiscoveryRequest, *discovery.DiscoveryResponse) {}

type callbacks struct {
}

// Hasher returns node ID as an ID
type Hasher struct {
}

// ID function
func (h Hasher) ID(node *core.Node) string {
	if node == nil {
		return "unknown"
	}
	return node.Id
}

func loadNodeConfig(ctx context.Context, node string, v string) {
	if l, c, e, s, err := myload.LoadNode(ctx, node); err == nil {
		snap := cache.NewSnapshot(v, e, c, nil, l, nil, s)
		if err := snap.Consistent(); err == nil {
			log.Debug("Snapshot consistent")
			config.SetSnapshot(node, snap)
		} else {
			log.Debugf("Config %s inconsistent for node %s", v, node)
		}
	} else {
		log.Debugf("Failed to load config for node %s", node)
	}
}

const grpcMaxConcurrentStreams = 1000000

// RunManagementServer starts an xDS server at the given port.
func RunManagementServer(ctx context.Context, server xds.Server, port uint) {
	var grpcOptions []grpc.ServerOption
	grpcOptions = append(grpcOptions, grpc.MaxConcurrentStreams(grpcMaxConcurrentStreams))
	grpcServer := grpc.NewServer(grpcOptions...)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.WithError(err).Fatal("failed to listen")
	}

	// register services
	discovery.RegisterAggregatedDiscoveryServiceServer(grpcServer, server)
	log.WithFields(log.Fields{"port": port}).Info("management server listening")
	go func() {
		if err = grpcServer.Serve(lis); err != nil {
			log.Error(err)
		}
	}()
	<-ctx.Done()

	grpcServer.GracefulStop()
}

type LocalHTTPGateway struct {
	Gateway *xds.HTTPGateway
}

func (h *LocalHTTPGateway) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	//	_ = h.Gateway.ServeHTTP(resp, req)
	respBytes, code, _ := h.Gateway.ServeHTTP(req)
	_, _ = resp.Write(respBytes)
	resp.WriteHeader(code)
}

// RunManagementGateway starts an HTTP gateway to an xDS server.
func RunManagementGateway(ctx context.Context, srv xds.Server, port uint) {
	log.WithFields(log.Fields{"port": port}).Info("gateway listening HTTP/1.1")
	server := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: &LocalHTTPGateway{Gateway: &xds.HTTPGateway{Server: srv}}}
	go func() {
		if err := server.ListenAndServe(); err != nil {
			log.Error(err)
		}
	}()
}

func main() {
	var err error
	flag.Parse()
	if debug {
		log.SetLevel(log.DebugLevel)
	}
	drivers.ConnectTimeout = time.Duration(driverTimeout) * time.Second
	ctx := context.Background()
	ctxTimeout, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)

	log.Printf("Starting control plane")

	cb := &callbacks{}
	config = cache.NewSnapshotCache(mode == Ads, Hasher{}, nil)

	srv := xds.NewServer(ctx, config, cb)

	// start the xDS server
	go RunManagementServer(ctx, srv, port)
	go RunManagementGateway(ctx, srv, gatewayPort)

	drivers.Cli, err = mongo.NewClient(options.Client().ApplyURI(mongoEndpoints))
	err = drivers.Cli.Connect(ctx)
	if err != nil {
		log.Fatal("Cannot connect to mongodb")
	}
	defer drivers.Cli.Disconnect(ctxTimeout)
	err = drivers.Cli.Ping(context.TODO(), nil)
	if err != nil {
		log.Fatal(err)
	}

	//  gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	v1 := r.Group("/api/v1/live")
	{
		v1.GET("/nodes", rapi.GetLiveNodes)
		v1.GET("/nodes/:node/listeners", rapi.GetLiveListeners)
		v1.GET("/nodes/:node/clusters", rapi.GetLiveClusters)
		v1.GET("/nodes/:node/config_dump", rapi.GetLiveConfig)
		v1.GET("/nodes/:node/logging", rapi.GetLiveLogging)
		v1.GET("/nodes/:node/sync", rapi.GetLiveSyncStatus)
		v1.PUT("/nodes/:node/logging", rapi.PutLiveLogging)
		v1.OPTIONS("/nodes/:node/logging", rapi.OptionsLiveLogging)
	}
	v1config := r.Group("/api/v1/config")
	{
		v1config.GET("/:node/listeners/*name", rapi.GetConfigListeners)
		v1config.GET("/:node/clusters/*name", rapi.GetConfigClusters)
		v1config.GET("/:node/endpoints/*name", rapi.GetConfigEndpoints)
		v1config.DELETE("/:node/listeners/:name", rapi.DeleteConfigListeners)
		v1config.DELETE("/:node/clusters/:name", rapi.DeleteConfigClusters)
		v1config.DELETE("/:node/endpoints/:name", rapi.DeleteConfigEndpoints)
		v1config.DELETE("/:node", rapi.DeleteConfigNodes)
		v1config.PUT("/:node/listeners/", rapi.PutConfigListeners)
		v1config.OPTIONS("/:node/listeners/*name", rapi.OptionsConfigListeners)
		v1config.PUT("/:node/clusters/", rapi.PutConfigClusters)
		v1config.OPTIONS("/:node/clusters/*name", rapi.OptionsConfigClusters)
		v1config.PUT("/:node/endpoints/", rapi.PutConfigEndpoints)
		v1config.PUT("/:node/endpoints/:name", rapi.PutConfigEndpointsNamed)
		v1config.OPTIONS("/:node/endpoints/:name", rapi.OptionsConfigEndpoints)
		v1config.OPTIONS("/:node/endpoints/", rapi.OptionsConfigEndpoints)
		v1config.PUT("/:node", rapi.PutConfigNode)
		v1config.OPTIONS("/:node", rapi.OptionsConfigNode)
		v1config.GET("/:node/interfaces", rapi.GetConfigInterfaces)
		v1config.PUT("/:node/interfaces", rapi.PutConfigInterfaces)
		v1config.OPTIONS("/:node/interfaces", rapi.OptionsConfigInterfaces)
		v1config.DELETE("/:node/interfaces", rapi.DeleteConfigInterfaces)
		v1config.GET("/:node/routes", rapi.GetConfigRoutes)
		v1config.PUT("/:node/routes", rapi.PutConfigRoutes)
		v1config.OPTIONS("/:node/routes", rapi.OptionsConfigRoutes)
		v1config.DELETE("/:node/routes", rapi.DeleteConfigRoutes)
		v1config.GET("/", rapi.GetConfigNode)
		v1config.GET("/:node/secrets/*name", rapi.GetConfigSecrets)
		v1config.PUT("/:node/secrets/", rapi.PutConfigSecrets)
		v1config.DELETE("/:node/secrets/:name", rapi.DeleteConfigSecrets)
		v1config.OPTIONS("/:node/secrets/*name", rapi.OptionsConfigSecrets)
		v1config.GET("/:node/configstatus", rapi.GetConfigStatus)
		v1config.OPTIONS("/:node/configstatus", rapi.OptionsConfigStatus)
	}
	v1ratelimit := r.Group("/api/v1/ratelimit")
	{
		v1ratelimit.GET("/*domain", rapi.GetRateLimit)
		v1ratelimit.PUT("/:domain", rapi.PutRateLimit)
		v1ratelimit.DELETE("/:domain", rapi.DeleteRateLimit)
		v1ratelimit.OPTIONS("/", rapi.OptionsRateLimit)
		v1ratelimit.OPTIONS("/:domain", rapi.OptionsRateLimit)
	}
	v1authz := r.Group("/api/v1/authz")
	{
		v1authz.GET("/", rapi.GetConfigAuthz)
		v1authz.PUT("/", rapi.PutConfigAuthz)
		v1authz.OPTIONS("/", rapi.OptionsAuthz)
	}
	v1authzs3 := r.Group("/api/v1/authzs3")
	{
		v1authzs3.GET("/", rapi.GetConfigAuthzS3)
		v1authzs3.PUT("/", rapi.PutConfigAuthzS3)
		v1authzs3.OPTIONS("/", rapi.OptionsAuthzS3)
	}
	go r.Run(":8080")

	endNodes := drivers.Cli.Database("envoy").Collection("nodes")
	workers = drivers.Cli.Database("envoy").Collection("workers")
	if cursor, err := endNodes.Find(ctxTimeout, bson.M{}); err == nil {
		for cursor.TryNext(ctxTimeout) {
			var node apitypes.NodeStaticConfigType
			if err := cursor.Decode(&node); err == nil {
				loadNodeConfig(ctx, node.Name, node.Version)
			}
		}
		cursor.Close(ctxTimeout)
	}

	versionStream, err := endNodes.Watch(ctx, mongo.Pipeline{})
	if err != nil {
		log.Fatal(err)
	}
	for versionStream.Next(ctx) {
		var data bson.M
		if err = versionStream.Decode(&data); err != nil {
			log.Fatal(err)
		}
		if data["operationType"].(string) == "insert" || data["operationType"].(string) == "update" {
			var newNode apitypes.NodeStaticConfigType
			doc := data["documentKey"].(primitive.M)
			ctxTimeout, _ = context.WithTimeout(ctx, drivers.ConnectTimeout)
			err = endNodes.FindOne(ctxTimeout, bson.M{"_id": doc["_id"].(primitive.ObjectID)}).Decode(&newNode)
			if err != nil {
				fmt.Println("Error getting new node")
				log.Debug(err)
			} else {
				loadNodeConfig(ctx, newNode.Name, newNode.Version)
			}
		}
	}
}
