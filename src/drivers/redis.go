package drivers

import (
	"crypto/tls"
	radix "github.com/mediocregopher/radix/v3"
	"strings"
)

type LocalPipeline []radix.CmdAction

func NewClient(uri string, useTls bool, auth string, poolSize int, redisType string) (radix.Client, error) {
	connFunc := func(network, addr string) (radix.Conn, error) {
		dialOpts := make([]radix.DialOpt, 0)
		if useTls {
			dialOpts = append(dialOpts, radix.DialUseTLS(&tls.Config{}))
		}
		if auth != "" {
			dialOpts = append(dialOpts, radix.DialAuthPass(auth))
		}
		return radix.Dial(network, addr, dialOpts...)
	}
	poolOpts := []radix.PoolOpt{radix.PoolConnFunc(connFunc)}
	var client radix.Client
	var err error
	poolFunc := func(network, addr string) (radix.Client, error) {
		return radix.NewPool(network, addr, poolSize, poolOpts...)
	}
	switch strings.ToLower(redisType) {
	case "single":
		client, err = poolFunc("tcp", uri)
	case "cluster":
		uris := strings.Split(uri, ",")
		client, err = radix.NewCluster(uris, radix.ClusterPoolFunc(poolFunc))
	default:
		panic("Unknown redis type")
	}
	return client, err
}

func DoPipeline(client radix.Client, pipeline LocalPipeline) error {
	execPipeline := radix.Pipeline(pipeline...)
	err := client.Do(execPipeline)
	return err
}
