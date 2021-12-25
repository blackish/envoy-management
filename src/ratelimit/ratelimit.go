package ratelimit

import (
	"context"
	freecache "github.com/coocood/freecache"
	v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/common/ratelimit/v3"
	pb "github.com/envoyproxy/go-control-plane/envoy/service/ratelimit/v3"
	units "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	radix "github.com/mediocregopher/radix/v3"
	log "github.com/sirupsen/logrus"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	cacheSize = 100 * 1024 * 1024
)

type limitType struct {
	Key    string
	Limit  uint32
	Hits   uint32
	Result *uint32
	Expire uint64
}
type limitDescriptor struct {
	Descriptors []limitType
}

type Service struct {
	configLock sync.RWMutex
	config     *CacheConfig
	client     radix.Client
	localCache *freecache.Cache
}

func (s *Service) Init(config *CacheConfig, rc radix.Client) {
	s.SetConfig(config)
	s.client = rc
	log.Printf("RateLimit service init")
	s.localCache = freecache.NewCache(cacheSize)
}

func (s *Service) SetConfig(newConfig *CacheConfig) {
	s.configLock.Lock()
	defer s.configLock.Unlock()
	s.config = newConfig
}

func (s *Service) ShouldRateLimit(ctx context.Context, req *pb.RateLimitRequest) (*pb.RateLimitResponse, error) {
	var currentConfig *CacheConfig
	var pipeliner radix.Action
	var err error
	s.configLock.RLock()
	currentConfig = s.config
	s.configLock.RUnlock()
	res := &pb.RateLimitResponse{OverallCode: pb.RateLimitResponse_OK}
	res.Statuses = make([]*pb.RateLimitResponse_DescriptorStatus, 0)
	var limits []limitDescriptor
	limits = make([]limitDescriptor, 0)
	hits := max(1, req.HitsAddend)
	if domain, ok := currentConfig.Domains[req.Domain]; ok {
		for _, descriptor := range req.Descriptors {
			limits = append(limits, findDescriptor(*domain, descriptor, hits))
		}
	}
	for _, limit := range limits {
		for _, entry := range limit.Descriptors {
			if _, err := s.localCache.Get([]byte(entry.Key)); err == nil {
				entry.Result = &entry.Limit
				res.OverallCode = pb.RateLimitResponse_OVER_LIMIT
				res.Statuses = append(res.Statuses, &pb.RateLimitResponse_DescriptorStatus{Code: pb.RateLimitResponse_OVER_LIMIT})
			} else {
				pipeliner = radix.Pipeline(
					radix.Cmd(entry.Result, "INCRBY", entry.Key, strconv.FormatUint(uint64(entry.Hits), 10)),
					radix.Cmd(nil, "EXPIRE", entry.Key, strconv.FormatUint(entry.Expire, 10)),
				)
				err = s.client.Do(pipeliner)
				if *entry.Result > entry.Limit {
					res.OverallCode = pb.RateLimitResponse_OVER_LIMIT
					res.Statuses = append(res.Statuses, &pb.RateLimitResponse_DescriptorStatus{Code: pb.RateLimitResponse_OVER_LIMIT})
					s.localCache.Set([]byte(entry.Key), []byte("limit"), int(entry.Expire))
				} else {
					res.Statuses = append(res.Statuses, &pb.RateLimitResponse_DescriptorStatus{Code: pb.RateLimitResponse_OK})
				}
			}
		}
	}
	return res, err
}

func findDescriptor(config CacheDescriptors, descriptor *v3.RateLimitDescriptor, hits uint32) limitDescriptor {
	var res limitDescriptor
	var builder strings.Builder
	var unit uint64
	var divider uint64
	unit = uint64(time.Now().Unix())
	res.Descriptors = make([]limitType, 0)
	for _, entry := range descriptor.Entries {
		builder.Reset()
		builder.WriteString(entry.Key)
		if config.StripDomain {
			builder.WriteString(strings.Split(entry.Value, ".")[0])
		} else {
			builder.WriteString(entry.Value)
		}
		if entry, ok := config.Entries[builder.String()]; ok {
			divider = unitToDivider(entry.Unit)
			builder.WriteString(strconv.FormatUint((unit/divider)*divider, 10))
			res.Descriptors = append(res.Descriptors, limitType{Key: builder.String(), Limit: entry.Limit, Hits: hits, Result: new(uint32), Expire: divider})
		}
	}
	return res
}

func max(a, b uint32) uint32 {
	if a > b {
		return a
	}
	return b
}

func unitToDivider(unit units.RateLimitUnit) uint64 {
	switch unit {
	case units.RateLimitUnit_SECOND:
		return 1
	case units.RateLimitUnit_MINUTE:
		return 60
	case units.RateLimitUnit_HOUR:
		return 60 * 60
	case units.RateLimitUnit_DAY:
		return 60 * 60 * 24
	}

	panic("should not get here")
}
