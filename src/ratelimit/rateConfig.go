package ratelimit

import (
	pb "github.com/envoyproxy/go-control-plane/envoy/type/v3"
)

type RLConfig struct {
	Entries []RLEntry
}

type RLEntry struct {
	Domain      string `bson:"domain" json:"domain"`
	Key         string `bson:"key" json:"key"`
	Value       string `bson:"value" json:"value"`
	Limit       uint32 `bson:"limit" json:"limit"`
	Unit        string `bson:"unit" json:"unit"`
	StripDomain bool   `bson:"stripDomain" json:"stripDomain"`
}

type CacheConfig struct {
	Domains map[string]*CacheDescriptors
}

type CacheDescriptors struct {
	Entries     map[string]*CacheEntry
	StripDomain bool
}

type CacheEntry struct {
	Limit uint32
	Unit  pb.RateLimitUnit
}
