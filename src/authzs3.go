package main

import (
	az "authzs3"
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
	"io/ioutil"
	"net"
	"net/http"
	"net/textproto"
	"reflect"
	"sync"
	"time"
)

var (
	port           uint
	hcPort         uint
	mongoEndpoints string
	driverTimeout  int64
	s3Endpoint     string
	s3Auth         string
	azServer       az.Service
	debug          bool
	configLock     sync.RWMutex
	namespaceLock  sync.RWMutex
	cache          *az.Authzs3ConfigType
	users          *BlobMapType
)

const (
	loginURL                 = "/login.json"
	usersURL                 = "/object/users.json"
	logoutURL                = "/logout.json"
	grpcMaxConcurrentStreams = 1000000
)

type LoginType struct {
	User string `json:"user"`
}

type BlobUserType struct {
	User      string `json:"userid"`
	Namespace string `json:"namespace"`
}

type UsersType struct {
	Users  []BlobUserType `json:"blobuser"`
	Filter string         `json:"filter"`
}

type BlobMapType struct {
	Users map[string]string `json:"users"`
}

func createMap(blobs *UsersType, maps *BlobMapType) {
	for _, i := range blobs.Users {
		maps.Users[i.User] = i.Namespace
	}
}

func init() {
	flag.UintVar(&port, "port", 18007, "Auth server port")
	flag.StringVar(&mongoEndpoints, "mongodb", "mongodb://10.99.6.24:27017", "Mongodb endpoints")
	flag.StringVar(&s3Endpoint, "s3", "https://avntg.s3mts.ru:4443", "S3 endpoint")
	flag.StringVar(&s3Auth, "s3auth", "base64 auth", "S3 basic auth")
	flag.BoolVar(&debug, "debug", false, "Use debug logging")
	flag.UintVar(&hcPort, "healthCheckPort", 8082, "Health check port")
	flag.Int64Var(&driverTimeout, "timeout", 5, "Mongodb connection timeout")
}

func loadConfig(ctx context.Context) (*az.Authzs3ConfigType, error) {
	ctxTimeout, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)
	newConfig := &az.Authzs3ConfigType{}
	if err := drivers.Cli.Database("envoy").Collection("authzs3").FindOne(ctxTimeout, bson.M{}).Decode(newConfig); err != nil {
		log.Debug("Fail parsing config")
		log.Debug(err)
		return nil, err
	}
	log.Debug(*newConfig)
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
	log.WithFields(log.Fields{"port": port}).Info("authorization s3 server listening")
	go func() {
		if err = grpcServer.Serve(lis); err != nil {
			log.Error(err)
		}
	}()
	<-ctx.Done()

	grpcServer.GracefulStop()
}

func loadUsers() {
	//	var u1 *BlobMapType
	var currentUsers map[string]string
	var auth string
	var body []byte
	var blobusers *UsersType
	httpClient := &http.Client{Timeout: (3 * time.Second)}
	for _ = range time.Tick(6 * time.Second) {
		log.Debug("Loading users")
		req, err := http.NewRequest("GET", s3Endpoint+loginURL, nil)
		req.Header.Add("Authorization", "Basic "+s3Auth)
		if err != nil {
			log.Error(err)
			continue
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			log.Error(err)
			continue
		}
		if authh, ok := resp.Header[textproto.CanonicalMIMEHeaderKey("x-sds-auth-token")]; ok {
			log.Debug("Auth success")
			auth = authh[0]
			req, _ = http.NewRequest("GET", s3Endpoint+usersURL, nil)
			req.Header.Add(textproto.CanonicalMIMEHeaderKey("x-sds-auth-token"), auth)
			resp, err = httpClient.Do(req)
			if err == nil {
				body, _ = ioutil.ReadAll(resp.Body)
				blobusers = &UsersType{}
				err = json.Unmarshal(body, blobusers)
				namespaceLock.Lock()
				users = &BlobMapType{}
				users.Users = make(map[string]string)
				if err == nil {
					createMap(blobusers, users)
					currentUsers = azServer.GetNamespaces()
					if !reflect.DeepEqual(users.Users, currentUsers) {
						azServer.SetNamespaces(users.Users)
					}
				} else {
					log.Error(err)
				}
				namespaceLock.Unlock()
			} else {
				log.Error(err)
			}
			req, _ = http.NewRequest("GET", s3Endpoint+logoutURL, nil)
			req.Header.Add(textproto.CanonicalMIMEHeaderKey("x-sds-auth-token"), auth)
			_, _ = httpClient.Do(req)
		}
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

func GetLiveUsers(ctx *gin.Context) {
	namespaceLock.RLock()
	body, _ := json.Marshal(users)
	namespaceLock.RUnlock()
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Data(200, "application/json", body)
	return
}

func main() {
	var err error
	flag.Parse()
	if debug {
		log.SetLevel(log.DebugLevel)
		log.Debug("Debug enabled")
	}
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
	cache, err = loadConfig(ctx)
	if err != nil {
		log.Info("Failed to load config")
	}

	azServer.SetConfig(cache)
	go loadUsers()

	log.Printf("Starting auths3 svc")
	go RunServer(ctx, &azServer, port)

	rlCollection := drivers.Cli.Database("envoy").Collection("authzs3")
	versionStream, err := rlCollection.Watch(ctx, mongo.Pipeline{})
	if err != nil {
		log.Fatal(err)
	}
	r := gin.Default()
	v1 := r.Group("/api/v1/live")
	{
		v1.GET("/status", GetLiveStatus)
		v1.GET("/config", GetLiveConfig)
		v1.GET("/users", GetLiveUsers)
	}
	go r.Run(fmt.Sprintf(":%d", hcPort))

	for versionStream.Next(ctx) {
		var data bson.M
		if err = versionStream.Decode(&data); err != nil {
			log.Fatal(err)
		}
		log.Debug("New config")
		//		if data["operationType"].(string) == "insert" || data["operationType"].(string) == "update" || data["operationType"].(string) == "delete" {
		configLock.Lock()
		cache, err = loadConfig(ctx)
		configLock.Unlock()
		if err == nil {
			log.Print("Config updated")
		}
		azServer.SetConfig(cache)
		//		}
	}
}
