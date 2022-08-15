package accesslogs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/cenkalti/backoff/v4"
	"strings"
	"time"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	alf "github.com/envoyproxy/go-control-plane/envoy/data/accesslog/v3"
	als "github.com/envoyproxy/go-control-plane/envoy/service/accesslog/v3"

	slog "github.com/sirupsen/logrus"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esutil"
	wrappers "github.com/golang/protobuf/ptypes/wrappers"
)

type logger struct{}

func (logger logger) Infof(format string, args ...interface{}) {
	slog.Infof(format, args...)
}
func (logger logger) Errorf(format string, args ...interface{}) {
	slog.Errorf(format, args...)
}

type HTTPLogEntry struct {
	Method                      string            `json:"method"`
	Scheme                      string            `json:"scheme"`
	Host                        string            `json:"host"`
	Path                        string            `json:"path"`
	UserAgent                   string            `json:"user.agent"`
	Referer                     string            `json:"referer"`
	RequestId                   string            `json:"request.id"`
	RequestHeaderBytes          uint64            `json:"request.header.bytes"`
	RequestHeaders              map[string]string `json:"request.header.content"`
	RequestBodyBytes            uint64            `json:"request.body.bytes"`
	ResponseCode                uint32            `json:"response.code"`
	ResponseHeaderBytes         uint64            `json:"response.header.bytes"`
	ResponseHeaders             map[string]string `json:"response.header.content"`
	ResponseBodyBytes           uint64            `json:"response.body.bytes"`
	ResponseDetails             string            `json:"response.details"`
	DownstreamRemoteAddress     string            `json:"downstream.address"`
	StartTime                   time.Time         `json:"start.time"`
	TimeToLastRxByte            uint32            `json:"time.toLastRxByte"`
	TimeToFirstUpstreamTxByte   uint32            `json:"time.toFirstUpstreamTxByte"`
	TimeToLastUpstreamTxByte    uint32            `json:"time.toLastUpstreamTxByte"`
	TimeToFirstUpstreamRxByte   uint32            `json:"time.tiFirstUpstreamRxByte"`
	TimeToLastUpstreamRxByte    uint32            `json:"time.toLastUpstreamRxByte"`
	TimeToFirstDownstreamTxByte uint32            `json:"time.toFirstDownstreamTxByte"`
	TimeToLastDownstreamTxByte  uint32            `json:"time.toLastDownstreamTxByte"`
	UpstreamRemoteAddress       string            `json:"upstream.address"`
	UpstreamCluster             string            `json:"upstream.cluster"`
	RouteName                   string            `json:"route.name"`
}

type TCPLogEntry struct {
	DownstreamRemoteAddress     string    `json:"downstream.address"`
	StartTime                   time.Time `json:"start.time"`
	TimeToLastRxByte            uint32    `json:"time.toLastRxByte"`
	TimeToFirstUpstreamTxByte   uint32    `json:"time.toFirstUpstreamTxByte"`
	TimeToLastUpstreamTxByte    uint32    `json:"time.toLastUpstreamTxByte"`
	TimeToFirstUpstreamRxByte   uint32    `json:"time.tiFirstUpstreamRxByte"`
	TimeToLastUpstreamRxByte    uint32    `json:"time.toLastUpstreamRxByte"`
	TimeToFirstDownstreamTxByte uint32    `json:"time.toFirstDownstreamTxByte"`
	TimeToLastDownstreamTxByte  uint32    `json:"time.toLastDownstreamTxByte"`
	UpstreamRemoteAddress       string    `json:"upstream.address"`
	UpstreamCluster             string    `json:"upstream.cluster"`
}

type BackendType int

const (
	ES BackendType = 0
	CH BackendType = 1
)

type LogWriter struct {
	Es *elasticsearch.Client
	BI esutil.BulkIndexer
	BE BackendType
	Ch ch.Conn
}

func (lw *LogWriter) Init(endpoint string, cls string, u string, p string, w int, back string) {
	if back == "clickhouse" {
		lw.BE = CH
	} else {
		lw.BE = ES
	}
	if lw.BE == ES {
		retryBackoff := backoff.NewExponentialBackOff()
		esConfig := &elasticsearch.Config{
			Addresses:     strings.Split(endpoint, ","),
			RetryOnStatus: []int{502, 503, 504, 429},
			RetryBackoff: func(i int) time.Duration {
				if i == 1 {
					retryBackoff.Reset()
				}
				return retryBackoff.NextBackOff()
			},
			MaxRetries: 5,
		}
		if len(u) > 0 && len(p) > 0 {
			esConfig.Username = u
			esConfig.Password = p
		}
		es, err := elasticsearch.NewClient(*esConfig)
		if err != nil {
			slog.Fatalf("Error creating the client: %s", err)
		}
		lw.Es = es
		bi, err := esutil.NewBulkIndexer(esutil.BulkIndexerConfig{
			Index:         "envoy",          // The default index name
			Client:        lw.Es,            // The Elasticsearch client
			NumWorkers:    w,                // The number of worker goroutines
			FlushBytes:    int(5248000),     // The flush threshold in bytes
			FlushInterval: 30 * time.Second, // The periodic flush interval
		})
		if err != nil {
			slog.Fatalf("Error creating the indexer: %s", err)
		}
		lw.BI = bi
	} else if lw.BE == CH {
		Cho, err := ch.Open(&ch.Options{
			Addr: strings.Split(cls, ","),
			Auth: ch.Auth{
				Database: "envoy",
				Username: u,
				Password: p,
			},
			//Debug:           true,
			DialTimeout:     time.Second,
			MaxOpenConns:    w,
			MaxIdleConns:    5,
			ConnMaxLifetime: time.Hour,
		})
		if err != nil {
			slog.Fatalf("Error creating the client: %s", err)
		}
		lw.Ch = Cho
	}
}

func (lw *LogWriter) WriteHTTPLog(l HTTPLogEntry) {
	if lw.BE == ES {
		if data, err := json.Marshal(l); err == nil {
			lw.BI.Add(context.Background(),
				esutil.BulkIndexerItem{
					Index:  "envoy-http-" + fmt.Sprintf("%d-%.2d-%.2d-%.2d", time.Now().Year(), time.Now().Month(), time.Now().Day(), time.Now().Hour()),
					Action: "index",
					Body:   bytes.NewReader(data),
				},
			)
		}
	} else if lw.BE == CH {
		qstring := fmt.Sprintf("INSERT INTO http_log VALUES ('%s','%s','%s','%s','%s','%s','%s',%d,{", l.Method, l.Scheme, l.Host, l.Path, l.UserAgent, l.Referer, l.RequestId, l.RequestHeaderBytes)
		for k, v := range l.RequestHeaders {
			qstring += fmt.Sprintf("'%s':'%s',", k, v)
		}
		qstring = strings.TrimRight(qstring, ",")
		qstring += fmt.Sprintf("},%d,%d,%d,{", l.RequestBodyBytes, l.ResponseCode, l.ResponseHeaderBytes)
		for k, v := range l.ResponseHeaders {
			qstring += fmt.Sprintf("'%s':'%s',", k, v)
		}
		qstring = strings.TrimRight(qstring, ",")
		qstring += fmt.Sprintf("},%d,'%s','%s',%d,%d,%d,%d,%d,%d,%d,%d,'%s','%s','%s')", l.ResponseBodyBytes, l.ResponseDetails, l.DownstreamRemoteAddress, l.StartTime.Unix(), l.TimeToLastRxByte, l.TimeToFirstUpstreamTxByte, l.TimeToLastUpstreamTxByte, l.TimeToFirstUpstreamRxByte, l.TimeToLastUpstreamRxByte, l.TimeToFirstDownstreamTxByte, l.TimeToLastDownstreamTxByte, l.UpstreamRemoteAddress, l.UpstreamCluster, l.RouteName)
		err := lw.Ch.AsyncInsert(context.Background(), qstring, false)
		if err != nil {
			slog.Print(err)
		}
	}
}

func (lw *LogWriter) WriteTCPLog(l TCPLogEntry) {
	if lw.BE == ES {
		if data, err := json.Marshal(l); err == nil {
			lw.BI.Add(context.Background(),
				esutil.BulkIndexerItem{
					// Action field configures the operation to perform (index, create, delete, update)
					Index:  "envoy-tcp-" + fmt.Sprintf("%d-%.2d-%.2d", time.Now().Year(), time.Now().Month(), time.Now().Day()),
					Action: "index",
					// Body is an `io.Reader` with the payload
					Body: bytes.NewReader(data),
				},
			)
		}
	} else if lw.BE == CH {
		qstring := fmt.Sprintf("INSERT INTO tcp_log VALUES ('%s',%d,%d,%d,%d,%d,%d,%d,%d,'%s','%s')", l.DownstreamRemoteAddress, l.StartTime.Unix(), l.TimeToLastRxByte, l.TimeToFirstUpstreamTxByte, l.TimeToLastUpstreamTxByte, l.TimeToFirstUpstreamRxByte, l.TimeToLastUpstreamRxByte, l.TimeToFirstDownstreamTxByte, l.TimeToLastDownstreamTxByte, l.UpstreamRemoteAddress, l.UpstreamCluster)
		err := lw.Ch.AsyncInsert(context.Background(), qstring, false)
		if err != nil {
			slog.Print(err)
		}
	}
}

// AccessLogService buffers access logs from the remote Envoy nodes.
type AccessLogServiceServer struct {
	LW LogWriter
}

func (svc *AccessLogServiceServer) Init(endpoint string, clickhouse string, u string, p string, w int, back string) {
	svc.LW.Init(endpoint, clickhouse, u, p, w, back)
}

// StreamAccessLogs implements the access log service.
func (svc *AccessLogServiceServer) StreamAccessLogs(stream als.AccessLogService_StreamAccessLogsServer) error {
	for {
		msg, err := stream.Recv()
		if err != nil {
			continue
		}
		switch entries := msg.LogEntries.(type) {
		case *als.StreamAccessLogsMessage_HttpLogs:
			for _, entry := range entries.HttpLogs.LogEntry {
				if entry != nil {
					common := entry.CommonProperties
					req := entry.Request
					resp := entry.Response
					if common == nil {
						common = &alf.AccessLogCommon{}
					}
					if req == nil {
						req = &alf.HTTPRequestProperties{}
					}
					if resp == nil {
						resp = &alf.HTTPResponseProperties{}
					}
					if resp.ResponseCode == nil {
						resp.ResponseCode = &wrappers.UInt32Value{Value: 0}
					}

					logEntry := HTTPLogEntry{
						Method:                      core.RequestMethod_name[int32(req.RequestMethod)],
						Scheme:                      req.Scheme,
						Host:                        req.Authority,
						Path:                        req.Path,
						UserAgent:                   req.UserAgent,
						Referer:                     req.Referer,
						RequestId:                   req.RequestId,
						RequestHeaderBytes:          req.RequestHeadersBytes,
						RequestBodyBytes:            req.RequestBodyBytes,
						ResponseCode:                resp.ResponseCode.Value,
						ResponseHeaderBytes:         resp.ResponseHeadersBytes,
						ResponseBodyBytes:           resp.ResponseBodyBytes,
						ResponseDetails:             resp.ResponseCodeDetails,
						UpstreamCluster:             common.UpstreamCluster,
						RouteName:                   common.RouteName,
						RequestHeaders:              make(map[string]string),
						ResponseHeaders:             make(map[string]string),
						TimeToLastRxByte:            0,
						TimeToFirstUpstreamTxByte:   0,
						TimeToLastUpstreamTxByte:    0,
						TimeToFirstUpstreamRxByte:   0,
						TimeToLastUpstreamRxByte:    0,
						TimeToFirstDownstreamTxByte: 0,
						TimeToLastDownstreamTxByte:  0,
					}
					for k, v := range req.RequestHeaders {
						logEntry.RequestHeaders[k] = v
					}
					for k, v := range resp.ResponseHeaders {
						logEntry.ResponseHeaders[k] = v
					}
					if common.DownstreamRemoteAddress != nil {
						if ad, ok := common.DownstreamRemoteAddress.Address.(*core.Address_SocketAddress); ok {
							logEntry.DownstreamRemoteAddress = ad.SocketAddress.Address
						}
					}
					if common.UpstreamRemoteAddress != nil {
						if ad, ok := common.UpstreamRemoteAddress.Address.(*core.Address_SocketAddress); ok {
							logEntry.UpstreamRemoteAddress = ad.SocketAddress.Address
						}
					}
					if common.StartTime != nil {
						logEntry.StartTime = time.Unix(common.StartTime.Seconds, int64(common.StartTime.Nanos))
					}
					if common.TimeToLastRxByte != nil {
						logEntry.TimeToLastRxByte = uint32(common.TimeToLastRxByte.Seconds)*1000000000 + uint32(common.TimeToLastRxByte.Nanos)
					}
					if common.TimeToFirstUpstreamTxByte != nil {
						logEntry.TimeToFirstUpstreamTxByte = uint32(common.TimeToFirstUpstreamTxByte.Seconds)*1000000000 + uint32(common.TimeToFirstUpstreamTxByte.Nanos)
					}
					if common.TimeToLastUpstreamTxByte != nil {
						logEntry.TimeToLastUpstreamTxByte = uint32(common.TimeToLastUpstreamTxByte.Seconds)*1000000000 + uint32(common.TimeToLastUpstreamTxByte.Nanos)
					}
					if common.TimeToFirstUpstreamRxByte != nil {
						logEntry.TimeToFirstUpstreamRxByte = uint32(common.TimeToFirstUpstreamRxByte.Seconds)*1000000000 + uint32(common.TimeToFirstUpstreamRxByte.Nanos)
					}
					if common.TimeToLastUpstreamRxByte != nil {
						logEntry.TimeToLastUpstreamRxByte = uint32(common.TimeToLastUpstreamRxByte.Seconds)*1000000000 + uint32(common.TimeToLastUpstreamRxByte.Nanos)
					}
					if common.TimeToFirstDownstreamTxByte != nil {
						logEntry.TimeToFirstDownstreamTxByte = uint32(common.TimeToFirstDownstreamTxByte.Seconds)*1000000000 + uint32(common.TimeToFirstDownstreamTxByte.Nanos)
					}
					if common.TimeToLastDownstreamTxByte != nil {
						logEntry.TimeToLastDownstreamTxByte = uint32(common.TimeToLastDownstreamTxByte.Seconds)*1000000000 + uint32(common.TimeToLastDownstreamTxByte.Nanos)
					}
					svc.LW.WriteHTTPLog(logEntry)
				}
			}
		case *als.StreamAccessLogsMessage_TcpLogs:
			for _, entry := range entries.TcpLogs.LogEntry {
				if entry != nil {
					common := entry.CommonProperties
					logEntry := TCPLogEntry{
						UpstreamCluster:             common.UpstreamCluster,
						DownstreamRemoteAddress:     "",
						TimeToLastRxByte:            0,
						TimeToFirstUpstreamTxByte:   0,
						TimeToLastUpstreamTxByte:    0,
						TimeToFirstUpstreamRxByte:   0,
						TimeToLastUpstreamRxByte:    0,
						TimeToFirstDownstreamTxByte: 0,
						TimeToLastDownstreamTxByte:  0,
					}
					if common == nil {
						common = &alf.AccessLogCommon{}
					}
					if common.DownstreamRemoteAddress != nil {
						if ad, ok := common.DownstreamRemoteAddress.Address.(*core.Address_SocketAddress); ok {
							logEntry.DownstreamRemoteAddress = ad.SocketAddress.Address
						}
					}
					if common.UpstreamRemoteAddress != nil {
						if ad, ok := common.UpstreamRemoteAddress.Address.(*core.Address_SocketAddress); ok {
							logEntry.UpstreamRemoteAddress = ad.SocketAddress.Address
						}
					}
					if common.StartTime != nil {
						logEntry.StartTime = time.Unix(common.StartTime.Seconds, int64(common.StartTime.Nanos))
					}
					if common.TimeToLastRxByte != nil {
						logEntry.TimeToLastRxByte = uint32(common.TimeToLastRxByte.Seconds)*1000000000 + uint32(common.TimeToLastRxByte.Nanos)
					}
					if common.TimeToFirstUpstreamTxByte != nil {
						logEntry.TimeToFirstUpstreamTxByte = uint32(common.TimeToFirstUpstreamTxByte.Seconds)*1000000000 + uint32(common.TimeToFirstUpstreamTxByte.Nanos)
					}
					if common.TimeToLastUpstreamTxByte != nil {
						logEntry.TimeToLastUpstreamTxByte = uint32(common.TimeToLastUpstreamTxByte.Seconds)*1000000000 + uint32(common.TimeToLastUpstreamTxByte.Nanos)
					}
					if common.TimeToFirstUpstreamRxByte != nil {
						logEntry.TimeToFirstUpstreamRxByte = uint32(common.TimeToFirstUpstreamRxByte.Seconds)*1000000000 + uint32(common.TimeToFirstUpstreamRxByte.Nanos)
					}
					if common.TimeToLastUpstreamRxByte != nil {
						logEntry.TimeToLastUpstreamRxByte = uint32(common.TimeToLastUpstreamRxByte.Seconds)*1000000000 + uint32(common.TimeToLastUpstreamRxByte.Nanos)
					}
					if common.TimeToFirstDownstreamTxByte != nil {
						logEntry.TimeToFirstDownstreamTxByte = uint32(common.TimeToFirstDownstreamTxByte.Seconds)*1000000000 + uint32(common.TimeToFirstDownstreamTxByte.Nanos)
					}
					if common.TimeToLastDownstreamTxByte != nil {
						logEntry.TimeToLastDownstreamTxByte = uint32(common.TimeToLastDownstreamTxByte.Seconds)*1000000000 + uint32(common.TimeToLastDownstreamTxByte.Nanos)
					}
					svc.LW.WriteTCPLog(logEntry)
				}
			}
		}
	}
}
