package authzs3

import (
	"context"
	"errors"
	v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	pb "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	v3types "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	wrapper "github.com/golang/protobuf/ptypes/wrappers"
	log "github.com/sirupsen/logrus"
	status "google.golang.org/genproto/googleapis/rpc/status"
	"strings"
	"sync"
)

type Service struct {
	config         *Authzs3ConfigType
	namespaces     map[string]string
	configLock     sync.RWMutex
	namespacesLock sync.RWMutex
}

func (s *Service) SetConfig(newConfig *Authzs3ConfigType) {
	s.configLock.Lock()
	defer s.configLock.Unlock()
	s.config = newConfig
	log.Debug("SetConfig")
	if s.config != nil {
		log.Debug(*s.config)
	}
}

func (s *Service) SetNamespaces(newNamespaces map[string]string) {
	s.namespacesLock.Lock()
	defer s.namespacesLock.Unlock()
	s.namespaces = newNamespaces
	log.Debug(s.namespaces)
}

func (s *Service) GetNamespaces() map[string]string {
	s.namespacesLock.RLock()
	defer s.namespacesLock.RUnlock()
	res := s.namespaces
	return res
}

func (s *Service) Check(ctx context.Context, req *pb.CheckRequest) (*pb.CheckResponse, error) {
	var currentConfig *Authzs3ConfigType
	var currentNamespaces map[string]string
	var requestNamespace string
	requestNamespace = ""
	s.configLock.RLock()
	currentConfig = s.config
	s.configLock.RUnlock()
	s.namespacesLock.RLock()
	currentNamespaces = s.namespaces
	s.namespacesLock.RUnlock()
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
	log.Debug(req)
	if currentConfig != nil {
		log.Debug(*currentConfig)
	}

	requestNamespace, err := getRequestNamespace(req, currentNamespaces)
	if err != nil {
		setStatus(result, true, "")
		log.Debug("No namespace found")
		return result, nil
	}

	setStatus(result, true, requestNamespace)
	if currentConfig == nil {
		return result, nil
	}

	if currentConfig.Config.Namespaces == nil {
		return result, nil
	}

	if r, ok := currentConfig.Config.Namespaces[requestNamespace]; ok {
		log.Debug(r)
		log.Debug(r.Action)
		setStatus(result, r.Action, requestNamespace)
		if r.Sources == nil {
			return result, nil
		}
		log.Debug(result)
		switch addr := req.Attributes.Source.Address.Address.(type) {
		case *v3.Address_SocketAddress:
			if a, ok := r.Sources[addr.SocketAddress.Address]; ok {
				setStatus(result, a.Action, requestNamespace)
				return result, nil
			}
		default:
			return result, nil
		}
	}
	return result, nil
}

func getRequestNamespace(req *pb.CheckRequest, currentNamespaces map[string]string) (string, error) {
	if hdr, ok := req.Attributes.Request.Http.Headers["authorization"]; ok {
		acc := getHeaderNamespace(hdr)
		if namespace, ok := currentNamespaces[acc]; ok {
			return namespace, nil
		}
	}
	query := strings.Split(req.Attributes.Request.Http.Path, "?")
	if len(query) > 1 {
		acc := getQueryNamespace(query[1])
		if namespace, ok := currentNamespaces[acc]; ok {
			return namespace, nil
		}
	}
	if ns, ok := req.Attributes.Request.Http.Headers["x-emc-namespace"]; ok {
		return ns, nil
	}
	hosts := strings.Split(req.Attributes.Request.Http.Host, ".")
	if len(hosts) >= 3 {
		return hosts[0], nil
	}
	return "", errors.New("namespace not found")
}

func getHeaderNamespace(hdr string) string {
	fields := strings.Fields(hdr)
	log.Debug(fields)
	if strings.HasPrefix(strings.ToUpper(fields[0]), "AWS4") {
		for i := range fields {
			if strings.HasPrefix(strings.ToLower(fields[i]), "credential=") {
				creds := strings.Split(fields[i][11:], "/")
				return creds[0]
			}
		}
	}
	if strings.HasPrefix(strings.ToUpper(fields[0]), "AWS") {
		creds := strings.Split(fields[1], ":")
		return creds[0]
	}
	return ""
}

func getQueryNamespace(qry string) string {
	fields := strings.Split(qry, "&")
	log.Debug(fields)
	for i := range fields {
		if strings.HasPrefix(strings.ToLower(fields[i]), "awsaccesskeyid=") {
			return fields[i][15:]
		}
		if strings.HasPrefix(strings.ToUpper(fields[i]), "X-AMZ-CREDENTIAL") {
			creds := strings.Split(fields[i], "%2F")
			return creds[0][17:]
		}
	}
	return ""
}

func setStatus(res *pb.CheckResponse, action bool, namespace string) {
	if action {
		if len(namespace) > 0 {
			res.HttpResponse = &pb.CheckResponse_OkResponse{
				OkResponse: &pb.OkHttpResponse{
					Headers: []*v3.HeaderValueOption{{
						Header: &v3.HeaderValue{
							Key:   "x-envoy-s3-namespace",
							Value: namespace,
						},
						Append: &wrapper.BoolValue{
							Value: false,
						},
					}},
				},
			}
		} else {
			res.HttpResponse = &pb.CheckResponse_OkResponse{OkResponse: &pb.OkHttpResponse{}}
		}
		res.Status.Code = 0
		return
	} else {
		if len(namespace) > 0 {
			res.HttpResponse = &pb.CheckResponse_DeniedResponse{
				DeniedResponse: &pb.DeniedHttpResponse{
					Status: &v3types.HttpStatus{Code: 403},
					Headers: []*v3.HeaderValueOption{{
						Header: &v3.HeaderValue{
							Key:   "x-envoy-s3-namespace",
							Value: namespace,
						},
						Append: &wrapper.BoolValue{
							Value: false,
						},
					}},
				},
			}
		} else {
			res.HttpResponse = &pb.CheckResponse_DeniedResponse{DeniedResponse: &pb.DeniedHttpResponse{Status: &v3types.HttpStatus{Code: 403}}}
		}
		res.Status.Code = 7
		return
	}
}
