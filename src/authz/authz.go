package authz

import (
	"context"
	v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	pb "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	v3types "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	status "google.golang.org/genproto/googleapis/rpc/status"
	"net/textproto"
	"sync"
)

type Service struct {
	config     *AuthzConfigType
	configLock sync.RWMutex
}

func (s *Service) SetConfig(newConfig *AuthzConfigType) {
	s.configLock.Lock()
	defer s.configLock.Unlock()
	s.config = newConfig
}

func (s *Service) Check(ctx context.Context, req *pb.CheckRequest) (*pb.CheckResponse, error) {
	var currentConfig *AuthzConfigType
	s.configLock.RLock()
	currentConfig = s.config
	s.configLock.RUnlock()
	result := &pb.CheckResponse{
		Status: &status.Status{Code: 200},
		HttpResponse: &pb.CheckResponse_OkResponse{
			OkResponse: &pb.OkHttpResponse{},
		},
	}
	if req.Attributes.Request == nil {
		return result, nil
	}
	if req.Attributes.Request.Http == nil {
		return result, nil
	}
	if currentConfig == nil {
		return result, nil
	}
	if currentConfig.Config.Domains == nil {
		return result, nil
	}
	if r, ok := currentConfig.Config.Domains[req.Attributes.Request.Http.Host]; ok {
		setStatus(result, r.Action)
		if r.Sources == nil {
			return result, nil
		}
		switch addr := req.Attributes.Source.Address.Address.(type) {
		case *v3.Address_SocketAddress:
			if a, ok := r.Sources[addr.SocketAddress.Address]; ok {
				if a.Headers == nil {
					setStatus(result, a.Action)
					return result, nil
				}
				if h, ok := req.Attributes.Request.Http.Headers[textproto.CanonicalMIMEHeaderKey(a.Headers.Headers[0].Name)]; ok {
					if h == a.Headers.Headers[0].Value {
						setStatus(result, a.Headers.Action)
						return result, nil
					}
				}
			}
		default:
			return result, nil
		}
	}
	return result, nil
}

func setStatus(res *pb.CheckResponse, action bool) {
	if action {
		res.HttpResponse = &pb.CheckResponse_OkResponse{OkResponse: &pb.OkHttpResponse{}}
		res.Status.Code = 0
		return
	}
	res.HttpResponse = &pb.CheckResponse_DeniedResponse{DeniedResponse: &pb.DeniedHttpResponse{Status: &v3types.HttpStatus{Code: 403}}}
	res.Status.Code = 7
	return
}
