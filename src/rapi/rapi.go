package rapi

import (
	authz "authz"
	authzs3 "authzs3"
	"bufio"
	"bytes"
	"context"
	rsa "crypto/rsa"
	x509 "crypto/x509"
	"drivers"
	"encoding/json"
	pem "encoding/pem"
	"fmt"
	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	auth "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/gin-gonic/gin"
	jsonpb "github.com/golang/protobuf/jsonpb"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"io"
	"io/ioutil"
	"ipmgmt"
	myload "loader"
	"net"
	"net/http"
	rl "ratelimit"
	"strconv"
	"strings"
	"time"
	apitypes "types"
)

// GetLiveLogging godoc
// @Summary Retrieves logging level
// @Produce json
// @Success 200
// @Router /live/nodes/{node}/logging [get]
func GetLiveLogging(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("GET GetLiveLogging")
	var loggingResult = make([]apitypes.LoggingLiveType, 0)
	node := ctx.Param("node")
	httpClient := &http.Client{Timeout: (3 * time.Second)}
	timeoutCtx, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)
	var newNode apitypes.NodeStaticConfigType
	err := drivers.Cli.Database("envoy").Collection("nodes").FindOne(timeoutCtx, bson.M{"name": node}).Decode(&newNode)
	if err == nil {
		if r, err := httpClient.Post("http://"+newNode.Address+":9901/logging", "application/text", nil); err == nil {
			defer r.Body.Close()
			reader := bufio.NewReader(r.Body)
			_, _ = reader.ReadString('\n')
			for {
				line, lerr := reader.ReadString('\n')
				if lerr == io.EOF {
					break
				}
				logger := strings.Split(line, ":")
				loggerModule := strings.TrimSpace(logger[0])
				if apitypes.LoggingModules[loggerModule] {
					loggingResult = append(loggingResult, apitypes.LoggingLiveType{Module: loggerModule, Loglevel: strings.TrimSpace(logger[1])})
				}
			}
		}
	}
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.JSON(200, gin.H{
		"logging": loggingResult})
	return
}

// PutLiveLogging godoc
// @Summary Change loglevel
// @Param node path string true "node id"
// @Produce json
// @Success 200
// @Router /live/nodes/{node}/logging [put]
func PutLiveLogging(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("PUT LiveLogging")
	var loggingResult = make([]apitypes.LoggingLiveType, 0)
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, DELETE")
	ctx.Header("Access-Control-Allow-Headers", "content-type")
	node := ctx.Param("node")
	if len(node) < 2 {
		ctx.JSON(400, gin.H{"error": "invalid name"})
	}
	var l apitypes.LoggingLiveType
	if err := ctx.ShouldBindJSON(&l); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !apitypes.LoggingModules[l.Module] || !apitypes.LoggingLevels[l.Loglevel] {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid data"})
		return
	}
	httpClient := &http.Client{Timeout: (3 * time.Second)}
	timeoutCtx, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)
	var newNode apitypes.NodeStaticConfigType
	err := drivers.Cli.Database("envoy").Collection("nodes").FindOne(timeoutCtx, bson.M{"name": node}).Decode(&newNode)
	if err == nil {
		if r, err := httpClient.Post("http://"+newNode.Address+":9901/logging?"+l.Module+"="+l.Loglevel, "application/text", nil); err == nil {
			defer r.Body.Close()
			reader := bufio.NewReader(r.Body)
			_, _ = reader.ReadString('\n')
			for {
				line, lerr := reader.ReadString('\n')
				if lerr == io.EOF {
					break
				}
				logger := strings.Split(line, ":")
				loggerModule := strings.TrimSpace(logger[0])
				if apitypes.LoggingModules[loggerModule] {
					loggingResult = append(loggingResult, apitypes.LoggingLiveType{Module: loggerModule, Loglevel: strings.TrimSpace(logger[1])})
				}
			}
		}
	}
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.JSON(200, gin.H{
		"logging": loggingResult})
	return

}

// GetLiveNodes godoc
// @Summary Retrieves list of worker nodes
// @Produce json
// @Success 200
// @Router /live/nodes [get]
func GetLiveNodes(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("GET GetLiveNodes")
	httpClient := &http.Client{Timeout: (3 * time.Second)}
	var n apitypes.NodeInfoType
	nodes := make(map[string]apitypes.NodeInfoType)
	timeoutCtx, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)
	if cursor, err := drivers.Cli.Database("envoy").Collection("nodes").Find(timeoutCtx, bson.M{}); err == nil {
		for cursor.TryNext(timeoutCtx) {
			var node apitypes.NodeStaticConfigType
			if err := cursor.Decode(&node); err == nil {
				if r, err := httpClient.Get("http://" + node.Address + ":9901/server_info"); err == nil {
					err = json.NewDecoder(r.Body).Decode(&n)
					if err == nil {
						log.Debug("Json parse error")
						r.Body.Close()
						nodes[node.Name] = n
					}
				}
			}
		}
		cursor.Close(timeoutCtx)
	}
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.JSON(200, gin.H{
		"nodes": nodes})
	return
}

// GetLiveListeners godoc
// @Summary Retrieves list of listeners on node
// @Produce json
// @Param node path string true "node id"
// @Success 200
// @Router /live/nodes/{node}/listeners [get]
func GetLiveListeners(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("GET GetLiveListeners")
	nodes := make(map[string][]apitypes.ListenerLiveType)
	node := ctx.Param("node")
	httpClient := &http.Client{Timeout: (3 * time.Second)}
	timeoutCtx, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)
	var newNode apitypes.NodeStaticConfigType
	err := drivers.Cli.Database("envoy").Collection("nodes").FindOne(timeoutCtx, bson.M{"name": node}).Decode(&newNode)
	if err == nil {
		if r, err := httpClient.Get("http://" + newNode.Address + ":9901/listeners"); err == nil {
			defer r.Body.Close()
			reader := bufio.NewReader(r.Body)
			for {
				line, lerr := reader.ReadString('\n')
				if lerr == io.EOF {
					break
				}
				listener := strings.Split(line, "::")
				currentNode := apitypes.ListenerLiveType{Name: listener[0], Address: strings.TrimSuffix(listener[1], "\n")}
				nodes[newNode.Name] = append(nodes[newNode.Name], currentNode)
			}
		}
	}
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.JSON(200, gin.H{
		"nodes": nodes})
	return
}

// GetLiveClusters godoc
// @Summary Retrieves list of clusters on node
// @Produce json
// @Param node path string true "node id"
// @Success 200
// @Router /live/nodes/{node}/clusters [get]
func GetLiveClusters(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("GET GetLiveListeners")
	nodes := make(map[string][]apitypes.ClustersLiveType)
	node := ctx.Param("node")
	httpClient := &http.Client{Timeout: (3 * time.Second)}
	timeoutCtx, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)
	var newNode apitypes.NodeStaticConfigType
	err := drivers.Cli.Database("envoy").Collection("nodes").FindOne(timeoutCtx, bson.M{"name": node}).Decode(&newNode)
	if err == nil {
		if r, err := httpClient.Get("http://" + newNode.Address + ":9901/clusters"); err == nil {
			defer r.Body.Close()
			reader := bufio.NewReader(r.Body)
			for {
				line, lerr := reader.ReadString('\n')
				if lerr == io.EOF {
					break
				}
				clusters := strings.Split(strings.TrimSuffix(line, "\n"), "::")
				if clusters[0] == "xds_cluster" || clusters[0] == "log_cluster" {
					continue
				}
				if clusters[1] == "default_priority" || clusters[1] == "high_priority" || clusters[1] == "added_via_api" {
					continue
				}
				if len(nodes[newNode.Name]) == 0 || nodes[newNode.Name][len(nodes[newNode.Name])-1].Name != clusters[0] {
					nodes[newNode.Name] = append(nodes[newNode.Name], apitypes.ClustersLiveType{Name: clusters[0]})
				}
				lastCluster := len(nodes[newNode.Name]) - 1
				if len(nodes[newNode.Name][lastCluster].Endpoints) == 0 || nodes[newNode.Name][lastCluster].Endpoints[len(nodes[newNode.Name][lastCluster].Endpoints)-1].Address != clusters[1] {
					nodes[newNode.Name][lastCluster].Endpoints = append(nodes[newNode.Name][lastCluster].Endpoints, apitypes.EndpointLiveType{Address: clusters[1]})
				}
				lastEndpoint := len(nodes[newNode.Name][lastCluster].Endpoints) - 1
				switch clusters[2] {
				case "priority":
					value, _ := strconv.Atoi(clusters[3])
					nodes[newNode.Name][lastCluster].Endpoints[lastEndpoint].Priority = value
				case "health_flags":
					nodes[newNode.Name][lastCluster].Endpoints[lastEndpoint].Health = clusters[3]
				case "rq_total":
					value, _ := strconv.ParseUint(clusters[3], 10, 32)
					nodes[newNode.Name][lastCluster].Endpoints[lastEndpoint].RequestsTotal = value
				case "rq_success":
					value, _ := strconv.ParseUint(clusters[3], 10, 32)
					nodes[newNode.Name][lastCluster].Endpoints[lastEndpoint].RequestsSuccess = value
				}
			}
		}
	}
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.JSON(200, gin.H{
		"nodes": nodes})
	return
}

// GetLiveConfig godoc
// @Summary Retrieves node config
// @Produce json
// @Param node path string true "node id"
// @Success 200
// @Router /live/nodes/{node}/config_dump [get]
func GetLiveConfig(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("GET GetLiveListeners")
	node := ctx.Param("node")
	httpClient := &http.Client{Timeout: (3 * time.Second)}
	timeoutCtx, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)
	var newNode apitypes.NodeStaticConfigType
	err := drivers.Cli.Database("envoy").Collection("nodes").FindOne(timeoutCtx, bson.M{"name": node}).Decode(&newNode)
	if err == nil {
		if r, err := httpClient.Get("http://" + newNode.Address + ":9901/config_dump"); err == nil {
			defer r.Body.Close()
			config, _ := ioutil.ReadAll(r.Body)
			ctx.Header("Access-Control-Allow-Origin", "*")
			ctx.Data(200, "application/json", config)
			return
		}
	}
	ctx.JSON(400, gin.H{"error": "failed to get config"})
	return
}

// GetConfigListeners godoc
// @Summary Retrieves listeners configuration
// @Produce json
// @Param node path string true "node id"
// @Param name path string false "listener id"
// @Success 200
// @Router /config/{node}/listeners/{name} [get]
func GetConfigListeners(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("GET GetConfigListeners")
	node := ctx.Param("node")
	name := ctx.Param("name")
	var res bytes.Buffer
	marshaler := jsonpb.Marshaler{}
	fmt.Fprintf(&res, `{ "listeners": [`)
	contextWithTimeout, _ := context.WithTimeout(ctx, 5*time.Second)
	l, _, _, _, _ := myload.LoadNode(contextWithTimeout, node)
	for _, elem := range l {
		e := elem.(*listener.Listener)
		if "/"+e.Name == name || name == "/" {
			_ = marshaler.Marshal(&res, e)
			fmt.Fprintf(&res, ",")
		}
	}
	if res.Len() > 16 {
		res.Truncate(len(res.Bytes()) - 1)
	}
	fmt.Fprintf(&res, `] }`)
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Data(200, "application/json", res.Bytes())
}

// GetConfigClusters godoc
// @Summary Retrieves clisters configuration
// @Produce json
// @Param node path string true "node id"
// @Param name path string false "cluster id"
// @Success 200
// @Router /config/{node}/clusters/{name} [get]
func GetConfigClusters(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("GET GetConfigClusters")
	node := ctx.Param("node")
	name := ctx.Param("name")
	var res bytes.Buffer
	marshaler := jsonpb.Marshaler{}
	fmt.Fprintf(&res, `{ "clusters": [`)
	contextWithTimeout, _ := context.WithTimeout(ctx, 5*time.Second)
	_, c, _, _, _ := myload.LoadNode(contextWithTimeout, node)
	for _, elem := range c {
		e := elem.(*cluster.Cluster)
		if "/"+e.Name == name || name == "/" {
			_ = marshaler.Marshal(&res, e)
			fmt.Fprintf(&res, ",")
		}
	}
	if res.Len() > 15 {
		res.Truncate(len(res.Bytes()) - 1)
	}
	fmt.Fprintf(&res, `] }`)
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Data(200, "application/json", res.Bytes())
}

// GetConfigEndpoints godoc
// @Summary Retrieves endpoints configuration
// @Produce json
// @Param node path string true "node id"
// @Param name path string false "cluster id"
// @Success 200
// @Router /config/{node}/endpoints/{name} [get]
func GetConfigEndpoints(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("GET GetConfigEndpoints")
	node := ctx.Param("node")
	name := ctx.Param("name")
	var res bytes.Buffer
	marshaler := jsonpb.Marshaler{}
	fmt.Fprintf(&res, `{ "endpoints": [`)
	contextWithTimeout, _ := context.WithTimeout(ctx, 5*time.Second)
	_, _, en, _, _ := myload.LoadNode(contextWithTimeout, node)
	for _, elem := range en {
		e := elem.(*endpoint.ClusterLoadAssignment)
		if "/"+e.ClusterName == name || name == "/" {
			_ = marshaler.Marshal(&res, e)
			fmt.Fprintf(&res, ",")
		}
	}
	if res.Len() > 16 {
		res.Truncate(len(res.Bytes()) - 1)
	}
	fmt.Fprintf(&res, `] }`)
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Data(200, "application/json", res.Bytes())
}

// DeleteConfigListeners godoc
// @Summary Delete configured listener
// @Param node path string true "node id"
// @Param name path string true "listener id"
// @Success 204
// @Router /config/{node}/listeners/{name} [delete]
func DeleteConfigListeners(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("DELETE DeleteConfigListeners")
	node := ctx.Param("node")
	name := ctx.Param("name")
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, DELETE")
	ctx.Header("Access-Control-Allow-Headers", "content-type")
	timeoutCtx, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)
	_, err := drivers.Cli.Database("envoy").Collection("listeners").DeleteOne(timeoutCtx, bson.M{"nodeId": node, "name": name})
	if err != nil {
		ctx.JSON(400, gin.H{"error": "failed to delete"})
		return
	}
	u := uuid.New()
	ups := &options.UpdateOptions{}
	_, err = drivers.Cli.Database("envoy").Collection("nodes").UpdateOne(timeoutCtx, bson.M{"name": node}, bson.M{"$set": bson.M{"version": u.String()}}, ups.SetUpsert(true))
	if err != nil {
		ctx.JSON(400, gin.H{"error": "failed to update node version"})
		return
	}
	ctx.Status(204)
}

// DeleteConfigClusters godoc
// @Summary Delete configured cluster
// @Param node path string true "node id"
// @Param name path string true "cluster id"
// @Success 204
// @Router /config/{node}/clusters/{name} [delete]
func DeleteConfigClusters(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("DELETE DeleteConfigClusters")
	node := ctx.Param("node")
	name := ctx.Param("name")
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, DELETE")
	ctx.Header("Access-Control-Allow-Headers", "content-type")
	timeoutCtx, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)
	_, err := drivers.Cli.Database("envoy").Collection("clusters").DeleteOne(timeoutCtx, bson.M{"nodeId": node, "name": name})
	if err != nil {
		ctx.JSON(400, gin.H{"error": "failed to delete cluster"})
		return
	}
	_, err = drivers.Cli.Database("envoy").Collection("endpoints").DeleteMany(timeoutCtx, bson.M{"nodeId": node, "clusterName": name})
	if err != nil {
		ctx.JSON(400, gin.H{"error": "failed to delete endpoints"})
		return
	}
	u := uuid.New()
	ups := &options.UpdateOptions{}
	_, err = drivers.Cli.Database("envoy").Collection("nodes").UpdateOne(timeoutCtx, bson.M{"name": node}, bson.M{"$set": bson.M{"version": u.String()}}, ups.SetUpsert(true))
	if err != nil {
		ctx.JSON(400, gin.H{"error": "failed to update node version"})
		return
	}
	ctx.Status(204)
}

// DeleteConfigEndpoints godoc
// @Summary Delete configured endpoint
// @Param node path string true "node id"
// @Param name path string true "cluster id"
// @Param priority path integer true "priority"
// @Param address path string true "endpoint address"
// @Success 204
// @Router /config/{node}/endpoints/{name} [delete]
func DeleteConfigEndpoints(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("DELETE DeleteConfigEndpoints")
	node := ctx.Param("node")
	name := ctx.Param("name")
	var e apitypes.EndpointsConfigType
	if err := ctx.ShouldBindJSON(&e); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, DELETE")
	ctx.Header("Access-Control-Allow-Headers", "content-type")
	e.NodeID = node
	newEndpoints, err := bson.Marshal(e)
	if err != nil {
		ctx.JSON(400, gin.H{"error": "failed to parse endpoints"})
		return
	}
	timeoutCtx, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)
	repOpts := &options.ReplaceOptions{}
	ups := &options.UpdateOptions{}
	_, err = drivers.Cli.Database("envoy").Collection("endpoints").ReplaceOne(timeoutCtx, bson.M{"nodeId": node, "clusterName": name}, newEndpoints, repOpts.SetUpsert(true))
	if err != nil {
		ctx.JSON(400, gin.H{"error": "failed to update endpoints"})
		return
	}
	u := uuid.New()
	_, err = drivers.Cli.Database("envoy").Collection("nodes").UpdateOne(timeoutCtx, bson.M{"name": node}, bson.M{"$set": bson.M{"version": u.String()}}, ups.SetUpsert(true))
	if err != nil {
		ctx.JSON(400, gin.H{"error": "failed to update node version"})
		return
	}
	ctx.Status(204)
}

// DeleteConfigNode godoc
// @Summary Delete configured node
// @Param node path string true "node id"
// @Success 204
// @Router /config/{node} [delete]
func DeleteConfigNodes(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("DELETE DeleteConfigNode")
	node := ctx.Param("node")
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, DELETE")
	ctx.Header("Access-Control-Allow-Headers", "content-type")
	if len(node) < 2 {
		ctx.JSON(400, gin.H{"error": "invalid name"})
	}
	timeoutCtx, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)
	_, err := drivers.Cli.Database("envoy").Collection("listeners").DeleteMany(timeoutCtx, bson.M{"nodeId": node})
	if err != nil {
		ctx.JSON(400, gin.H{"error": "failed to delete listeners"})
		return
	}
	_, err = drivers.Cli.Database("envoy").Collection("clusters").DeleteMany(timeoutCtx, bson.M{"nodeId": node})
	if err != nil {
		ctx.JSON(400, gin.H{"error": "failed to delete cluster"})
		return
	}
	_, err = drivers.Cli.Database("envoy").Collection("endpoints").DeleteMany(timeoutCtx, bson.M{"nodeId": node})
	if err != nil {
		ctx.JSON(400, gin.H{"error": "failed to delete endpoints"})
		return
	}
	_, err = drivers.Cli.Database("envoy").Collection("nodes").DeleteOne(timeoutCtx, bson.M{"name": node})
	if err != nil {
		ctx.JSON(400, gin.H{"error": "failed to delete node"})
		return
	}
	ctx.Status(204)
}

// PutConfigListeners godoc
// @Summary Create or update listener
// @Param node path string true "node id"
// @Success 204
// @Router /config/{node}/listeners [put]
func PutConfigListeners(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("PUT PutConfigListeners")
	node := ctx.Param("node")
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, DELETE")
	ctx.Header("Access-Control-Allow-Headers", "content-type")
	if len(node) < 2 {
		ctx.JSON(400, gin.H{"error": "invalid name"})
	}
	var l apitypes.ListenerConfigType
	if err := ctx.ShouldBindJSON(&l); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if net.ParseIP(l.Address.SocketAddress.Address) == nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Error parsing ip"})
		return
	}
	l.NodeID = node
	newListener, err := bson.Marshal(l)
	if err != nil {
		ctx.JSON(400, gin.H{"error": "failed to parse listener"})
		return
	}

	timeoutCtx, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)
	repOpts := &options.ReplaceOptions{}
	_, err = drivers.Cli.Database("envoy").Collection("listeners").ReplaceOne(timeoutCtx, bson.M{"nodeId": node, "name": l.Name}, newListener, repOpts.SetUpsert(true))
	if err != nil {
		ctx.JSON(400, gin.H{"error": "failed to update listener"})
		return
	}
	u := uuid.New()
	ups := &options.UpdateOptions{}
	_, err = drivers.Cli.Database("envoy").Collection("nodes").UpdateOne(timeoutCtx, bson.M{"name": node}, bson.M{"$set": bson.M{"version": u.String()}}, ups.SetUpsert(true))
	if err != nil {
		ctx.JSON(400, gin.H{"error": "failed to update node version"})
		return
	}
	ctx.Status(204)
}

// PutConfigClusters godoc
// @Summary Create or update cluster
// @Param node path string true "node id"
// @Success 204
// @Router /config/{node}/clusters [put]
func PutConfigClusters(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("PUT PutConfigClusters")
	node := ctx.Param("node")
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, DELETE")
	ctx.Header("Access-Control-Allow-Headers", "content-type")
	if len(node) < 2 {
		ctx.JSON(400, gin.H{"error": "invalid name"})
	}
	var c apitypes.ClusterConfigType
	if err := ctx.ShouldBindJSON(&c); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if c.ConnectTimeout == "0s" || c.ConnectTimeout == "0" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "ConnectTimeout should be greater than 0"})
		return
	}
	c.NodeID = node
	newCluster, err := bson.Marshal(c)
	if err != nil {
		ctx.JSON(400, gin.H{"error": "failed to parse cluster"})
		return
	}

	c.NodeID = node
	timeoutCtx, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)
	repOpts := &options.ReplaceOptions{}
	_, err = drivers.Cli.Database("envoy").Collection("clusters").ReplaceOne(timeoutCtx, bson.M{"nodeId": node, "name": c.Name}, newCluster, repOpts.SetUpsert(true))
	if err != nil {
		ctx.JSON(400, gin.H{"error": "failed to update listener"})
		return
	}
	u := uuid.New()
	ups := &options.UpdateOptions{}
	_, err = drivers.Cli.Database("envoy").Collection("nodes").UpdateOne(timeoutCtx, bson.M{"name": node}, bson.M{"$set": bson.M{"version": u.String()}}, ups.SetUpsert(true))
	if err != nil {
		ctx.JSON(400, gin.H{"error": "failed to update node version"})
		return
	}
	ctx.Status(204)
}

// PutConfigEndpoints godoc
// @Summary Create or update endpoint
// @Param node path string true "node id"
// @Success 204
// @Router /config/{node}/endpoints [put]
func PutConfigEndpoints(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("PUT PutConfigEndpoints")
	node := ctx.Param("node")
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, DELETE")
	ctx.Header("Access-Control-Allow-Headers", "content-type")
	if len(node) < 2 {
		ctx.JSON(400, gin.H{"error": "invalid name"})
	}
	var e apitypes.EndpointsConfigType
	if err := ctx.ShouldBindJSON(&e); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(e.Endpoints) == 0 || len(e.Endpoints[0].LbEndpoints) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "No endpoints found"})
		return
	}
	if net.ParseIP(e.Endpoints[0].LbEndpoints[0].Endpoint.Address.SocketAddress.Address) == nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Error parsing ip"})
		return
	}

	e.NodeID = node
	timeoutCtx, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)
	repOpts := &options.ReplaceOptions{}
	var endpoints apitypes.EndpointsConfigType
	updated := false
	err := drivers.Cli.Database("envoy").Collection("endpoints").FindOne(timeoutCtx, bson.M{"nodeId": node, "clusterName": e.ClusterName}).Decode(&endpoints)
	if err == nil {
		for p := range endpoints.Endpoints {
			if endpoints.Endpoints[p].Priority == e.Endpoints[0].Priority {
				endpoints.Endpoints[p].LbEndpoints = append(endpoints.Endpoints[p].LbEndpoints, e.Endpoints[0].LbEndpoints[0])
				updated = true
			}
		}
		if !updated {
			endpoints.Endpoints = append(endpoints.Endpoints, apitypes.LbEndpointConfigType{Priority: e.Endpoints[0].Priority, LbEndpoints: e.Endpoints[0].LbEndpoints})
		}
	} else {
		endpoints = e
	}
	newEndpoints, err := bson.Marshal(endpoints)
	if err != nil {
		ctx.JSON(400, gin.H{"error": "failed to parse endpoints"})
		return
	}
	_, err = drivers.Cli.Database("envoy").Collection("endpoints").ReplaceOne(timeoutCtx, bson.M{"nodeId": node, "clusterName": e.ClusterName}, newEndpoints, repOpts.SetUpsert(true))
	if err != nil {
		ctx.JSON(400, gin.H{"error": "failed to update endpoints"})
		return
	}
	u := uuid.New()
	ups := &options.UpdateOptions{}
	timeoutCtx, _ = context.WithTimeout(ctx, time.Duration(drivers.ConnectTimeout)*time.Second)
	_, err = drivers.Cli.Database("envoy").Collection("nodes").UpdateOne(timeoutCtx, bson.M{"name": node}, bson.M{"$set": bson.M{"version": u.String()}}, ups.SetUpsert(true))
	if err != nil {
		ctx.JSON(400, gin.H{"error": "failed to update node version"})
		return
	}
	ctx.Status(204)
}

// PutConfigEndpointsNamed godoc
// @Summary Update endpoint cluster
// @Param node path string true "node id"
// @Success 204
// @Router /config/{node}/endpoints/{name} [put]
func PutConfigEndpointsNamed(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("PUT PutConfigEndpoints")
	node := ctx.Param("node")
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, DELETE")
	ctx.Header("Access-Control-Allow-Headers", "content-type")
	if len(node) < 2 {
		ctx.JSON(400, gin.H{"error": "invalid name"})
	}
	var e apitypes.EndpointsConfigType
	if err := ctx.ShouldBindJSON(&e); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(e.Endpoints) == 0 || len(e.Endpoints[0].LbEndpoints) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "No endpoints found"})
		return
	}
	if net.ParseIP(e.Endpoints[0].LbEndpoints[0].Endpoint.Address.SocketAddress.Address) == nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Error parsing ip"})
		return
	}

	e.NodeID = node
	timeoutCtx, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)
	newEndpoints, err := bson.Marshal(e)
	if err != nil {
		ctx.JSON(400, gin.H{"error": "failed to parse endpoints"})
		return
	}
	repOpts := &options.ReplaceOptions{}
	_, err = drivers.Cli.Database("envoy").Collection("endpoints").ReplaceOne(timeoutCtx, bson.M{"nodeId": node, "clusterName": e.ClusterName}, newEndpoints, repOpts.SetUpsert(true))
	if err != nil {
		ctx.JSON(400, gin.H{"error": "failed to update endpoints"})
		return
	}
	u := uuid.New()
	ups := &options.UpdateOptions{}
	timeoutCtx, _ = context.WithTimeout(ctx, time.Duration(drivers.ConnectTimeout)*time.Second)
	_, err = drivers.Cli.Database("envoy").Collection("nodes").UpdateOne(timeoutCtx, bson.M{"name": node}, bson.M{"$set": bson.M{"version": u.String()}}, ups.SetUpsert(true))
	if err != nil {
		ctx.JSON(400, gin.H{"error": "failed to update node version"})
		return
	}
	ctx.Status(204)
}

// GetConfigNode godoc
// @Summary Get configured nodes
// @Produce json
// @Success 200
// @Router /config/ [get]
func GetConfigNode(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("GET GetConfigNodes")
	nodes := make([]apitypes.NodeStaticConfigType, 0)
	timeoutCtx, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)
	if cursor, err := drivers.Cli.Database("envoy").Collection("nodes").Find(timeoutCtx, bson.M{}); err == nil {
		for cursor.TryNext(timeoutCtx) {
			var node apitypes.NodeStaticConfigType
			if err := cursor.Decode(&node); err == nil {
				nodes = append(nodes, node)
			}
		}
		cursor.Close(timeoutCtx)
	}
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.JSON(200, gin.H{
		"nodes": nodes})
	return

}

// PutConfigNode godoc
// @Summary Create or update node
// @Param node path string true "node id"
// @Success 204
// @Router /config/{node} [put]
func PutConfigNode(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("GET PutConfigNode")
	node := ctx.Param("node")
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, DELETE")
	ctx.Header("Access-Control-Allow-Headers", "content-type")
	if len(node) < 2 {
		ctx.JSON(400, gin.H{"error": "invalid name"})
	}
	var n apitypes.NodeStaticConfigType
	if err := ctx.ShouldBindJSON(&n); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if net.ParseIP(n.Address) == nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Error parsing ip"})
		return
	}
	n.Name = node
	timeoutCtx, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)

	ups := &options.UpdateOptions{}
	_, err := drivers.Cli.Database("envoy").Collection("nodes").UpdateOne(timeoutCtx, bson.M{"name": node}, bson.M{"$set": bson.M{"address": n.Address}}, ups.SetUpsert(true))
	if err != nil {
		ctx.JSON(400, gin.H{"error": "failed to update node version"})
		return
	}
	ctx.Status(204)
}

// GetConfigInterfaces godoc
// @Summary Get configured node interfaces
// @Param node path string true "node id"
// @Produce json
// @Success 200
// @Router /config/{node}/interfaces [get]
func GetConfigInterfaces(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("GET GetConfigInterfaces")
	node := ctx.Param("node")
	httpClient := &http.Client{Timeout: (3 * time.Second)}
	timeoutCtx, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)
	var newNode apitypes.NodeStaticConfigType
	err := drivers.Cli.Database("envoy").Collection("nodes").FindOne(timeoutCtx, bson.M{"name": node}).Decode(&newNode)
	if err == nil {
		if r, err := httpClient.Get("http://" + newNode.Address + ":9902/api/v1/interface"); err == nil {
			defer r.Body.Close()
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				ctx.JSON(400, gin.H{"error": "failed to get interfaces"})
				return
			}
			var interfaces ipmgmt.NodeInterfaceListConfigType
			err = json.Unmarshal(body, &interfaces)
			if err != nil {
				ctx.JSON(400, gin.H{"error": "failed to get interfaces"})
				return
			}
			ctx.Header("Access-Control-Allow-Origin", "*")
			ctx.Data(200, "application/json", body)
			return
		}
	}
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.JSON(200, gin.H{"localinterfaces": ""})
	return
}

// PutConfigInterfaces godoc
// @Summary Create or update node interface address
// @Param node path string true "node id"
// @Success 204
// @Router /config/{node}/interfaces [put]
func PutConfigInterfaces(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("PUT PutConfigInterfaces")
	node := ctx.Param("node")
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, DELETE")
	ctx.Header("Access-Control-Allow-Headers", "content-type")
	nodeAddr := ""
	nodeInterface := ipmgmt.NodeInterfaceConfigType{}
	httpClient := &http.Client{Timeout: (3 * time.Second)}
	timeoutCtx, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)
	var newNode apitypes.NodeStaticConfigType
	err := drivers.Cli.Database("envoy").Collection("nodes").FindOne(timeoutCtx, bson.M{"name": node}).Decode(&newNode)
	if err == nil {
		nodeAddr = newNode.Address
	} else {
		ctx.JSON(400, gin.H{"error": "node not found"})
		return
	}
	if err := ctx.ShouldBindJSON(&nodeInterface); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	body, _ := json.Marshal(nodeInterface)
	req, _ := http.NewRequest("PUT", "http://"+nodeAddr+":9902/api/v1/interface", bytes.NewReader(body))
	req.Header["Content-Type"] = []string{"application/json"}
	if r, err := httpClient.Do(req); err == nil {
		defer r.Body.Close()
		if err != nil || r.StatusCode > 299 {
			ctx.JSON(400, gin.H{"error": "failed to set interfaces"})
			return
		}
	}
	ctx.Status(204)
	return
}

// DeleteConfigInterfaces godoc
// @Summary Delete node interface address
// @Param node path string true "node id"
// @Success 204
// @Router /config/{node}/interfaces [delete]
func DeleteConfigInterfaces(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("DELETE DeleteConfigInterfaces")
	node := ctx.Param("node")
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, DELETE")
	ctx.Header("Access-Control-Allow-Headers", "content-type")
	nodeAddr := ""
	nodeInterface := ipmgmt.NodeInterfaceConfigType{}
	httpClient := &http.Client{Timeout: (3 * time.Second)}
	timeoutCtx, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)
	var newNode apitypes.NodeStaticConfigType
	err := drivers.Cli.Database("envoy").Collection("nodes").FindOne(timeoutCtx, bson.M{"name": node}).Decode(&newNode)
	if err == nil {
		nodeAddr = newNode.Address
	} else {
		ctx.JSON(400, gin.H{"error": "node not found"})
		return
	}
	if err := ctx.ShouldBindJSON(&nodeInterface); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	body, _ := json.Marshal(nodeInterface)
	req, _ := http.NewRequest("DELETE", "http://"+nodeAddr+":9902/api/v1/interface", bytes.NewReader(body))
	req.Header["Content-Type"] = []string{"application/json"}
	if r, err := httpClient.Do(req); err == nil {
		defer r.Body.Close()
		if err != nil || r.StatusCode > 299 {
			ctx.JSON(400, gin.H{"error": "failed to delete interfaces"})
			return
		}
	}
	ctx.Status(204)
	return
}

// GetConfigRoutes godoc
// @Summary Get configured node routes
// @Param node path string true "node id"
// @Produce json
// @Success 200
// @Router /config/{node}/routes [get]
func GetConfigRoutes(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("GET GetConfigRoutes")
	node := ctx.Param("node")
	httpClient := &http.Client{Timeout: (3 * time.Second)}
	timeoutCtx, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)
	var newNode apitypes.NodeStaticConfigType
	err := drivers.Cli.Database("envoy").Collection("nodes").FindOne(timeoutCtx, bson.M{"name": node}).Decode(&newNode)
	if err == nil {
		if r, err := httpClient.Get("http://" + newNode.Address + ":9902/api/v1/route"); err == nil {
			defer r.Body.Close()
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				ctx.JSON(400, gin.H{"error": "failed to get routes"})
				return
			}
			var routes ipmgmt.NodeRouteListConfigType
			err = json.Unmarshal(body, &routes)
			if err != nil {
				ctx.JSON(400, gin.H{"error": "failed to get routes"})
				return
			}
			ctx.Header("Access-Control-Allow-Origin", "*")
			ctx.Data(200, "application/json", body)
			return
		}
	}
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.JSON(200, gin.H{"localinterfaces": ""})
	return
}

// PutConfigRoutes godoc
// @Summary Create or update node route
// @Param node path string true "node id"
// @Success 204
// @Router /config/{node}/routes [put]
func PutConfigRoutes(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("PUT PutConfigRoutes")
	node := ctx.Param("node")
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, DELETE")
	ctx.Header("Access-Control-Allow-Headers", "content-type")
	nodeAddr := ""
	nodeRoute := ipmgmt.NodeRouteConfigType{}
	httpClient := &http.Client{Timeout: (3 * time.Second)}
	timeoutCtx, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)
	var newNode apitypes.NodeStaticConfigType
	err := drivers.Cli.Database("envoy").Collection("nodes").FindOne(timeoutCtx, bson.M{"name": node}).Decode(&newNode)
	if err == nil {
		nodeAddr = newNode.Address
	} else {
		ctx.JSON(400, gin.H{"error": "node not found"})
		return
	}
	if err := ctx.ShouldBindJSON(&nodeRoute); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	body, _ := json.Marshal(nodeRoute)
	req, _ := http.NewRequest("PUT", "http://"+nodeAddr+":9902/api/v1/route", bytes.NewReader(body))
	req.Header["Content-Type"] = []string{"application/json"}
	if r, err := httpClient.Do(req); err == nil {
		defer r.Body.Close()
		if err != nil || r.StatusCode > 299 {
			ctx.JSON(400, gin.H{"error": "failed to set routes"})
			return
		}
	}
	ctx.Status(204)
	return
}

// DeleteConfigRoutes godoc
// @Summary Delete configured routes
// @Param node path string true "node id"
// @Success 204
// @Router /config/{node}/routes [delete]
func DeleteConfigRoutes(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("DELETE DeleteConfigInterface")
	node := ctx.Param("node")
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, DELETE")
	ctx.Header("Access-Control-Allow-Headers", "content-type")
	nodeAddr := ""
	nodeRoute := ipmgmt.NodeRouteConfigType{}
	httpClient := &http.Client{Timeout: (3 * time.Second)}
	timeoutCtx, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)
	var newNode apitypes.NodeStaticConfigType
	err := drivers.Cli.Database("envoy").Collection("nodes").FindOne(timeoutCtx, bson.M{"name": node}).Decode(&newNode)
	if err == nil {
		nodeAddr = newNode.Address
	} else {
		ctx.JSON(400, gin.H{"error": "node not found"})
		return
	}
	if err := ctx.ShouldBindJSON(&nodeRoute); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	body, _ := json.Marshal(nodeRoute)
	req, _ := http.NewRequest("DELETE", "http://"+nodeAddr+":9902/api/v1/route", bytes.NewReader(body))
	req.Header["Content-Type"] = []string{"application/json"}
	if r, err := httpClient.Do(req); err == nil {
		defer r.Body.Close()
		if err != nil || r.StatusCode > 299 {
			ctx.JSON(400, gin.H{"error": "failed to delete route"})
			return
		}
	}
	ctx.Status(204)
	return
}

// GetConfigSecrets godoc
// @Summary Retrieves listeners configuration
// @Produce json
// @Param node path string true "node id"
// @Param name path string false "secret id"
// @Success 200
// @Router /config/{node}/secrets/{name} [get]
func GetConfigSecrets(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("GET GetConfigSecrets")
	node := ctx.Param("node")
	name := ctx.Param("name")
	var res bytes.Buffer
	fmt.Fprintf(&res, `{ "secrets": [`)
	contextWithTimeout, _ := context.WithTimeout(ctx, 5*time.Second)
	_, _, _, s, _ := myload.LoadNode(contextWithTimeout, node)
	for _, elem := range s {
		e := elem.(*auth.Secret)
		if "/"+e.Name == name || name == "/" {
			var c apitypes.SecretViewType
			c.Name = e.Name
			var cblock *pem.Block
			switch e.Type.(type) {
			case *auth.Secret_TlsCertificate:
				lcrt := e.Type.(*auth.Secret_TlsCertificate)
				cblock, _ = pem.Decode([]byte(lcrt.TlsCertificate.CertificateChain.Specifier.(*core.DataSource_InlineString).InlineString))
			case *auth.Secret_ValidationContext:
				lcrt := e.Type.(*auth.Secret_ValidationContext)
				cblock, _ = pem.Decode([]byte(lcrt.ValidationContext.TrustedCa.Specifier.(*core.DataSource_InlineString).InlineString))
			}
			crt, err := x509.ParseCertificate(cblock.Bytes)
			if err == nil {
				switch e.Type.(type) {
				case *auth.Secret_TlsCertificate:
					c.TlsCertificate = &apitypes.TlsCertificateViewType{
						NotBefore: crt.NotBefore.String(),
						NotAfter:  crt.NotAfter.String(),
						Issuer:    crt.Issuer.String(),
						Subject:   crt.Subject.String(),
					}
				case *auth.Secret_ValidationContext:
					c.ValidationContextCa = &apitypes.TlsCertificateViewType{
						NotBefore: crt.NotBefore.String(),
						NotAfter:  crt.NotAfter.String(),
						Issuer:    crt.Issuer.String(),
						Subject:   crt.Subject.String(),
					}
				}
			}
			body, _ := json.Marshal(c)
			if err == nil {
				fmt.Fprintf(&res, string(body))
				fmt.Fprintf(&res, ",")
			}
		}
	}
	if res.Len() > 14 {
		res.Truncate(len(res.Bytes()) - 1)
	}
	fmt.Fprintf(&res, `] }`)
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Data(200, "application/json", res.Bytes())
}

// PutConfigSecrets godoc
// @Summary Create or update cluster
// @Param node path string true "node id"
// @Success 204
// @Router /config/{node}/secrets [put]
func PutConfigSecrets(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("PUT PutConfigSecrets")
	node := ctx.Param("node")
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, DELETE")
	ctx.Header("Access-Control-Allow-Headers", "content-type")
	if len(node) < 2 {
		log.Println("invalid nodename")
		ctx.JSON(400, gin.H{"error": "invalid name"})
	}
	var s apitypes.SecretConfigType
	if err := ctx.ShouldBindJSON(&s); err != nil {
		log.Println("Error parse")
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	s.NodeID = node
	if s.TlsCertificate != nil {
		pblock, rest := pem.Decode([]byte(s.TlsCertificate.PrivateKey.InlineString))
		if len(rest) > 0 {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Error parsing private key"})
			return
		}
		rootPool := x509.NewCertPool()
		cblock, rest := pem.Decode([]byte(s.TlsCertificate.CertificateChain.InlineString))
		if len(rest) > 0 {
			_ = rootPool.AppendCertsFromPEM(rest)
		}
		crt, err := x509.ParseCertificate(cblock.Bytes)
		if err != nil {
			log.Println("Failed to parse certificate")
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err})
			return
		}
		if len(rest) > 0 {
			opts := x509.VerifyOptions{
				Roots: rootPool,
			}
			_, err := crt.Verify(opts)
			if err != nil {
				log.Println("Failed to verify certificate chain")
				ctx.JSON(http.StatusBadRequest, gin.H{"error": err})
				return
			}
		}
		pkey, err := x509.ParsePKCS1PrivateKey(pblock.Bytes)
		if err != nil {
			log.Println("Failed to parse private key")
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err})
			return
		}
		pkey_public := pkey.Public().(*rsa.PublicKey)
		if ok := pkey_public.Equal(crt.PublicKey); ok != true {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Private key do not correspond certificate"})
			return
		}
	}
	if s.ValidationContext != nil {
		cblock, _ := pem.Decode([]byte(s.ValidationContext.TrustedCa.InlineString))
		_, err := x509.ParseCertificate(cblock.Bytes)
		if err != nil {
			log.Println("Failed to parse certificate")
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err})
			return
		}
	}
	newSecret, err := bson.Marshal(s)
	if err != nil {
		ctx.JSON(400, gin.H{"error": "failed to parse secret"})
		return
	}
	timeoutCtx, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)
	repOpts := &options.ReplaceOptions{}
	_, err = drivers.Cli.Database("envoy").Collection("secrets").ReplaceOne(timeoutCtx, bson.M{"nodeId": node, "name": s.Name}, newSecret, repOpts.SetUpsert(true))
	if err != nil {
		ctx.JSON(400, gin.H{"error": "failed to update secret"})
		return
	}
	u := uuid.New()
	ups := &options.UpdateOptions{}
	_, err = drivers.Cli.Database("envoy").Collection("nodes").UpdateOne(timeoutCtx, bson.M{"name": node}, bson.M{"$set": bson.M{"version": u.String()}}, ups.SetUpsert(true))
	if err != nil {
		ctx.JSON(400, gin.H{"error": "failed to update node version"})
		return
	}
	ctx.Status(204)
}

// DeleteConfigSecrets godoc
// @Summary Delete configured listener
// @Param node path string true "node id"
// @Param name path string true "secret id"
// @Success 204
// @Router /config/{node}/secrets/{name} [delete]
func DeleteConfigSecrets(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("DELETE DeleteConfigSecrets")
	node := ctx.Param("node")
	name := ctx.Param("name")
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, DELETE")
	ctx.Header("Access-Control-Allow-Headers", "content-type")
	timeoutCtx, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)
	_, err := drivers.Cli.Database("envoy").Collection("secrets").DeleteOne(timeoutCtx, bson.M{"nodeId": node, "name": name})
	if err != nil {
		ctx.JSON(400, gin.H{"error": "failed to delete"})
		return
	}
	u := uuid.New()
	ups := &options.UpdateOptions{}
	_, err = drivers.Cli.Database("envoy").Collection("nodes").UpdateOne(timeoutCtx, bson.M{"name": node}, bson.M{"$set": bson.M{"version": u.String()}}, ups.SetUpsert(true))
	if err != nil {
		ctx.JSON(400, gin.H{"error": "failed to update node version"})
		return
	}
	ctx.Status(204)
}

// GetConfigRateLimit godoc
// @Summary get configured ratelimits
// @Param domain path string false "domain"
// @Success 200
// @Router /ratelimit/{domain} [get]
func GetRateLimit(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("GET GetRateLimit")
	domain := ctx.Param("domain")
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, DELETE")
	ctx.Header("Access-Control-Allow-Headers", "content-type")
	var filter bson.M
	if domain == "/" {
		filter = bson.M{}
	} else {
		filter = bson.M{"domain": domain}
	}
	var ratelimits rl.RLConfig
	ratelimits.Entries = make([]rl.RLEntry, 0)
	timeoutCtx, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)
	if cursor, err := drivers.Cli.Database("envoy").Collection("ratelimits").Find(timeoutCtx, filter); err == nil {
		for cursor.TryNext(timeoutCtx) {
			var rlentry rl.RLEntry
			if err := cursor.Decode(&rlentry); err == nil {
				ratelimits.Entries = append(ratelimits.Entries, rlentry)
			}
		}
		cursor.Close(timeoutCtx)
	}
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.JSON(200, gin.H{
		"ratelimits": ratelimits.Entries})
	return
}

// PutRateLimit godoc
// @Summary Create or update rate limit
// @Param domain path string true "node id"
// @Success 204
// @Router /ratelimit/{domain} [put]
func PutRateLimit(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("PUT PutRateLimit")
	domain := ctx.Param("domain")
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, DELETE")
	ctx.Header("Access-Control-Allow-Headers", "content-type")
	if len(domain) < 2 {
		log.Println("invalid domain")
		ctx.JSON(400, gin.H{"error": "invalid domain"})
	}
	var ratelimit rl.RLEntry
	if err := ctx.ShouldBindJSON(&ratelimit); err != nil {
		log.Println("Error parse")
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ratelimit.Domain = domain
	newRL, err := bson.Marshal(ratelimit)
	timeoutCtx, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)
	repOpts := &options.ReplaceOptions{}
	_, err = drivers.Cli.Database("envoy").Collection("ratelimits").ReplaceOne(timeoutCtx, bson.M{"domain": domain, "key": ratelimit.Key, "value": ratelimit.Value}, newRL, repOpts.SetUpsert(true))
	if err != nil {
		ctx.JSON(400, gin.H{"error": "failed to update ratelimit"})
		return
	}
	ctx.Status(204)
}

// DeleteRateLimit godoc
// @Summary Delete rate limit
// @Param domain path string true "node id"
// @Success 204
// @Router /ratelimit/{domain} [put]
func DeleteRateLimit(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("DELETE DeleteRateLimit")
	domain := ctx.Param("domain")
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, DELETE")
	ctx.Header("Access-Control-Allow-Headers", "content-type")
	if len(domain) < 2 {
		log.Println("invalid domain")
		ctx.JSON(400, gin.H{"error": "invalid domain"})
	}
	var ratelimit rl.RLEntry
	if err := ctx.ShouldBindJSON(&ratelimit); err != nil {
		log.Println("Error parse")
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ratelimit.Domain = domain
	timeoutCtx, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)
	_, err := drivers.Cli.Database("envoy").Collection("ratelimits").DeleteOne(timeoutCtx, bson.M{"domain": domain, "key": ratelimit.Key, "value": ratelimit.Value})
	if err != nil {
		ctx.JSON(400, gin.H{"error": "failed to delete ratelimit"})
		return
	}
	ctx.Status(204)
}

// GetConfigAuthz godoc
// @Summary get configured authz
// @Success 200
// @Router /authz/ [get]
func GetConfigAuthz(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("GET GetAuthz")
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, DELETE")
	ctx.Header("Access-Control-Allow-Headers", "content-type")
	var authzconfig authz.AuthzConfigType
	timeoutCtx, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)
	if err := drivers.Cli.Database("envoy").Collection("authz").FindOne(timeoutCtx, bson.M{}).Decode(&authzconfig); err != nil {
		ctx.Header("Access-Control-Allow-Origin", "*")
		ctx.JSON(200, gin.H{
			"authz": authzconfig})
		return
	}
	ctx.JSON(400, gin.H{"authz": "{}"})
	return
}

// PutConfigAuthz godoc
// @Summary Create or update authz
// @Success 204
// @Router /authz/ [put]
func PutConfigAuthz(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("PUT PutAuthz")
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, DELETE")
	ctx.Header("Access-Control-Allow-Headers", "content-type")
	var authzconfig authz.AuthzConfigType
	if err := ctx.ShouldBindJSON(&authzconfig); err != nil {
		log.Println("Error parse")
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	newAZ, err := bson.Marshal(authzconfig)
	timeoutCtx, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)
	repOpts := &options.ReplaceOptions{}
	_, err = drivers.Cli.Database("envoy").Collection("authz").ReplaceOne(timeoutCtx, bson.M{}, newAZ, repOpts.SetUpsert(true))
	if err != nil {
		ctx.JSON(400, gin.H{"error": "failed to update authz"})
		return
	}
	ctx.Status(204)
}

// GetConfigAuthzS3 godoc
// @Summary get configured authzs3
// @Success 200
// @Router /authzs3/ [get]
func GetConfigAuthzS3(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("GET GetAuthzS3")
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, DELETE")
	ctx.Header("Access-Control-Allow-Headers", "content-type")
	var authzconfig authzs3.Authzs3ConfigType
	timeoutCtx, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)
	if err := drivers.Cli.Database("envoy").Collection("authzs3").FindOne(timeoutCtx, bson.M{}).Decode(&authzconfig); err == nil {
		ctx.Header("Access-Control-Allow-Origin", "*")
		ctx.JSON(200, gin.H{
			"authzs3": authzconfig})
		return
	}
	ctx.JSON(200, gin.H{"authzs3": "{}"})
	return
}

// PutConfigAuthzS3 godoc
// @Summary Create or update authz
// @Success 204
// @Router /authzs3/ [put]
func PutConfigAuthzS3(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("PUT PutAuthzS3")
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, DELETE")
	ctx.Header("Access-Control-Allow-Headers", "content-type")
	var authzconfig authzs3.Authzs3ConfigType
	if err := ctx.ShouldBindJSON(&authzconfig); err != nil {
		log.Println("Error parse")
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	newAZ, err := bson.Marshal(authzconfig)
	timeoutCtx, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)
	repOpts := &options.ReplaceOptions{}
	_, err = drivers.Cli.Database("envoy").Collection("authzs3").ReplaceOne(timeoutCtx, bson.M{}, newAZ, repOpts.SetUpsert(true))
	if err != nil {
		ctx.JSON(400, gin.H{"error": "failed to update authz"})
		return
	}
	ctx.Status(204)
}

// GetConfigStatus godoc
// @Summary get node config status
// @Param node path string true "node"
// @Success 200
// @Router /config/{node}/configstatus [get]
func GetConfigStatus(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("GET GetConfigStatus")
	node := ctx.Param("node")
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, DELETE")
	ctx.Header("Access-Control-Allow-Headers", "content-type")
	if l, c, e, s, err := myload.LoadNode(ctx, node); err == nil {
		snap := cache.NewSnapshot(node, e, c, nil, l, nil, s)
		if err := snap.Consistent(); err == nil {
			ctx.JSON(200, gin.H{
				"configstatus": "consistent"})
			return
		} else {
			ctx.JSON(200, gin.H{
				"configstatus": "inconsistent"})
			return
		}
	} else {
		ctx.JSON(400, gin.H{
			"error": err})
		return
	}
}

// GetLiveSyncStatus godoc
// @Summary Retrieves worker sync status
// @Produce json
// @Success 200
// @Router /live/nodes/{node}/sync [get]
func GetLiveSyncStatus(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("GET GetLiveSyncStatus")
	var syncStatus apitypes.WorkerSyncConfigType
	var nodeStatus apitypes.NodeStaticConfigType
	node := ctx.Param("node")
	timeoutCtx, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)
	ctx.Header("Access-Control-Allow-Origin", "*")
	err := drivers.Cli.Database("envoy").Collection("nodes").FindOne(timeoutCtx, bson.M{"name": node}).Decode(&nodeStatus)
	if err == nil {
		errSync := drivers.Cli.Database("envoy").Collection("workers").FindOne(timeoutCtx, bson.M{"name": node}).Decode(&syncStatus)
		if errSync == nil {
			ctx.Header("Access-Control-Allow-Origin", "*")
			if nodeStatus.Version == syncStatus.Version {
				ctx.JSON(200, gin.H{
					"synced":  "True",
					"updated": syncStatus.Updated,
					"status":  syncStatus.Status,
				})
			} else {
				ctx.JSON(200, gin.H{
					"synced":  "False",
					"updated": syncStatus.Updated,
					"status":  syncStatus.Status,
				})
			}
			return
		} else {
			ctx.JSON(200, gin.H{
				"synced":  "False",
				"updated": " ",
				"status":  "Not yet synced",
			})
			return
		}
	} else {
		ctx.JSON(400, gin.H{
			"error": err})
		return
	}
}

func OptionsConfigListeners(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("OPTIONS configListeners")
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, DELETE")
	ctx.Header("Access-Control-Allow-Headers", "content-type")
	ctx.Header("Content-Type", "text/plain")
	ctx.Status(204)
	return
}

func OptionsConfigClusters(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("OPTIONS configClusters")
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, DELETE")
	ctx.Header("Access-Control-Allow-Headers", "content-type")
	ctx.Header("Content-Type", "text/plain")
	ctx.Status(204)
	return
}

func OptionsConfigEndpoints(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("OPTIONS configEndpoints")
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, DELETE")
	ctx.Header("Access-Control-Allow-Headers", "content-type")
	ctx.Header("Content-Type", "text/plain")
	ctx.Status(204)
	return
}

func OptionsConfigNode(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("OPTIONS configNode")
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, DELETE")
	ctx.Header("Access-Control-Allow-Headers", "content-type")
	ctx.Header("Content-Type", "text/plain")
	ctx.Status(204)
	return
}

func OptionsConfigInterfaces(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("OPTIONS configInterfaces")
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, DELETE")
	ctx.Header("Access-Control-Allow-Headers", "content-type")
	ctx.Header("Content-Type", "text/plain")
	ctx.Status(204)
	return
}

func OptionsConfigRoutes(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("OPTIONS configRoutes")
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, DELETE")
	ctx.Header("Access-Control-Allow-Headers", "content-type")
	ctx.Header("Content-Type", "text/plain")
	ctx.Status(204)
	return
}

func OptionsConfigSecrets(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("OPTIONS configSecrets")
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, DELETE")
	ctx.Header("Access-Control-Allow-Headers", "content-type")
	ctx.Header("Content-Type", "text/plain")
	ctx.Status(204)
	return
}

func OptionsRateLimit(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("OPTIONS rateLimit")
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, DELETE")
	ctx.Header("Access-Control-Allow-Headers", "content-type")
	ctx.Header("Content-Type", "text/plain")
	ctx.Status(204)
	return
}
func OptionsAuthz(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("OPTIONS authz")
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, DELETE")
	ctx.Header("Access-Control-Allow-Headers", "content-type")
	ctx.Header("Content-Type", "text/plain")
	ctx.Status(204)
	return
}
func OptionsAuthzS3(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("OPTIONS authzs3")
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, DELETE")
	ctx.Header("Access-Control-Allow-Headers", "content-type")
	ctx.Header("Content-Type", "text/plain")
	ctx.Status(204)
	return
}
func OptionsConfigStatus(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("OPTIONS configftatus")
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, DELETE")
	ctx.Header("Access-Control-Allow-Headers", "content-type")
	ctx.Header("Content-Type", "text/plain")
	ctx.Status(204)
	return
}
func OptionsLiveLogging(ctx *gin.Context) {
	log.WithFields(log.Fields{"Module": "RAPI"}).Debug("OPTIONS configftatus")
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, DELETE")
	ctx.Header("Access-Control-Allow-Headers", "content-type")
	ctx.Header("Content-Type", "text/plain")
	ctx.Status(204)
	return
}
