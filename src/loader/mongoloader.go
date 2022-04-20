package loader

import (
	"context"
	"strings"
	"time"
	apitypes "types"

	v3 "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v3"
	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	lrfconfig "github.com/envoyproxy/go-control-plane/envoy/config/ratelimit/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	als "github.com/envoyproxy/go-control-plane/envoy/extensions/access_loggers/grpc/v3"
	authz "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_authz/v3"
	lrf "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ratelimit/v3"
	lftls "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/listener/tls_inspector/v3"
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	lrm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/local_ratelimit/v3"
	tcp "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/tcp_proxy/v3"
	auth "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	upstreamprotoopts "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/http/v3"
	matcher "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	typesv3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	any "github.com/golang/protobuf/ptypes/any"

	"drivers"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"github.com/golang/protobuf/ptypes"
	wrappers "github.com/golang/protobuf/ptypes/wrappers"
	"go.mongodb.org/mongo-driver/bson"
)

func parseSecret(s apitypes.SecretConfigType) *auth.Secret {
	sec := &auth.Secret{
		Name: s.Name,
	}
	if s.TlsCertificate != nil {
		sec.Type = &auth.Secret_TlsCertificate{
			TlsCertificate: &auth.TlsCertificate{
				CertificateChain: &core.DataSource{
					Specifier: &core.DataSource_InlineString{
						InlineString: s.TlsCertificate.CertificateChain.InlineString,
					},
				},
				PrivateKey: &core.DataSource{
					Specifier: &core.DataSource_InlineString{
						InlineString: s.TlsCertificate.PrivateKey.InlineString,
					},
				},
			},
		}
	}
	if s.ValidationContext != nil {
		sec.Type = &auth.Secret_ValidationContext{
			ValidationContext: &auth.CertificateValidationContext{
				TrustedCa: &core.DataSource{
					Specifier: &core.DataSource_InlineString{
						InlineString: s.ValidationContext.TrustedCa.InlineString,
					},
				},
			},
		}
	}
	return sec
}

func parseListener(l apitypes.ListenerConfigType) *listener.Listener {
	var cfc *listener.FilterChain
	res := &listener.Listener{
		Name: l.Name,
		Address: &core.Address{
			Address: &core.Address_SocketAddress{
				SocketAddress: &core.SocketAddress{
					Address:  l.Address.SocketAddress.Address,
					Protocol: core.SocketAddress_TCP,
					PortSpecifier: &core.SocketAddress_PortValue{
						PortValue: uint32(l.Address.SocketAddress.PortValue),
					},
				},
			},
		},
		ListenerFilters: make([]*listener.ListenerFilter, 0),
		FilterChains:    make([]*listener.FilterChain, 0),
	}
	if l.PerConnectionBufferLimitBytes != nil {
		res.PerConnectionBufferLimitBytes = &wrappers.UInt32Value{Value: uint32(*l.PerConnectionBufferLimitBytes)}
	}
	for _, fc := range l.FilterChains {
		cfc = &listener.FilterChain{}
		cfc.Filters = make([]*listener.Filter, 0)
		if fc.FilterChainMatch != nil {
			cfc.FilterChainMatch = &listener.FilterChainMatch{}
			if &fc.FilterChainMatch.ServerNames != nil {
				cfc.FilterChainMatch.ServerNames = fc.FilterChainMatch.ServerNames
				if len(res.ListenerFilters) == 0 {
					lft := &lftls.TlsInspector{}
					mt, _ := ptypes.MarshalAny(lft)
					res.ListenerFilters = append(res.ListenerFilters, &listener.ListenerFilter{Name: wellknown.TlsInspector, ConfigType: &listener.ListenerFilter_TypedConfig{TypedConfig: mt}})
				}
			}
			if &fc.FilterChainMatch.SourcePrefixRanges != nil {
				cfc.FilterChainMatch.SourcePrefixRanges = make([]*core.CidrRange, 0)
				for _, spf := range fc.FilterChainMatch.SourcePrefixRanges {
					cfc.FilterChainMatch.SourcePrefixRanges = append(cfc.FilterChainMatch.SourcePrefixRanges, &core.CidrRange{AddressPrefix: spf.AddressPrefix, PrefixLen: &wrappers.UInt32Value{Value: uint32(spf.PrefixLen)}})
				}
			}
			if &fc.FilterChainMatch.PrefixRanges != nil {
				cfc.FilterChainMatch.PrefixRanges = make([]*core.CidrRange, 0)
				for _, pf := range fc.FilterChainMatch.PrefixRanges {
					cfc.FilterChainMatch.PrefixRanges = append(cfc.FilterChainMatch.PrefixRanges, &core.CidrRange{AddressPrefix: pf.AddressPrefix, PrefixLen: &wrappers.UInt32Value{Value: uint32(pf.PrefixLen)}})
				}
			}
		}
		for _, f := range fc.Filters {
			if f.Name == "envoy.filters.network.local_ratelimit" {
				var cf *listener.Filter
				if d, derr := time.ParseDuration(f.TypedConfig.TokenBucket.FillInterval); derr == nil {
					lrm := &lrm.LocalRateLimit{
						StatPrefix: f.TypedConfig.StatPrefix,
						TokenBucket: &typesv3.TokenBucket{
							MaxTokens:     uint32(f.TypedConfig.TokenBucket.MaxTokens),
							TokensPerFill: &wrappers.UInt32Value{Value: uint32(f.TypedConfig.TokenBucket.TokensPerFill)},
							FillInterval:  ptypes.DurationProto(d),
						},
					}
					mt, _ := ptypes.MarshalAny(lrm)
					cf = &listener.Filter{
						Name: "envoy.filters.network.local_ratelimit",
						ConfigType: &listener.Filter_TypedConfig{
							TypedConfig: mt,
						},
					}
				}
				cfc.Filters = append(cfc.Filters, cf)
			}
			if f.Name == "envoy.filters.network.tcp_proxy" {
				tcpp := &tcp.TcpProxy{
					StatPrefix:       f.TypedConfig.StatPrefix,
					ClusterSpecifier: &tcp.TcpProxy_Cluster{Cluster: *f.TypedConfig.Cluster},
				}
				if f.TypedConfig.HashPolicy != nil {
					tcpp.HashPolicy = make([]*typesv3.HashPolicy, 0)
					tcpp.HashPolicy = append(tcpp.HashPolicy, &typesv3.HashPolicy{PolicySpecifier: &typesv3.HashPolicy_SourceIp_{SourceIp: &typesv3.HashPolicy_SourceIp{}}})
				}
				if f.TypedConfig.IdleTimeout != nil {
					if d, derr := time.ParseDuration(*f.TypedConfig.IdleTimeout); derr == nil {
						tcpp.IdleTimeout = ptypes.DurationProto(d)
					}
				}
				if f.TypedConfig.AccessLog != nil {
					al := &als.TcpGrpcAccessLogConfig{
						CommonConfig: &als.CommonGrpcAccessLogConfig{
							TransportApiVersion: core.ApiVersion_V3,
							LogName:             f.TypedConfig.AccessLog[0].Name,
							GrpcService: &core.GrpcService{
								TargetSpecifier: &core.GrpcService_EnvoyGrpc_{
									EnvoyGrpc: &core.GrpcService_EnvoyGrpc{ClusterName: "log_cluster"},
								},
							},
						},
					}
					tcpp.AccessLog = make([]*v3.AccessLog, 1)
					mt, _ := ptypes.MarshalAny(al)
					tcpp.AccessLog[0] = &v3.AccessLog{
						Name: "RemoteAccessLog", ConfigType: &v3.AccessLog_TypedConfig{
							TypedConfig: mt,
						},
					}
				}
				mt, _ := ptypes.MarshalAny(tcpp)
				cfc.Filters = append(cfc.Filters, &listener.Filter{
					Name: wellknown.TCPProxy,
					ConfigType: &listener.Filter_TypedConfig{
						TypedConfig: mt,
					},
				})
			}
			if f.Name == "envoy.filters.network.http_connection_manager" {
				httpp := &hcm.HttpConnectionManager{
					CodecType:  hcm.HttpConnectionManager_AUTO,
					StatPrefix: f.TypedConfig.StatPrefix,
					RouteSpecifier: &hcm.HttpConnectionManager_RouteConfig{
						RouteConfig: &route.RouteConfiguration{
							Name:         f.TypedConfig.RouteConfig.Name,
							VirtualHosts: make([]*route.VirtualHost, 0),
						},
					},
				}
				// HttpProtocolOption
				if &f.TypedConfig.HttpProtocolOptions != nil {
					httpp.HttpProtocolOptions = &core.Http1ProtocolOptions{
						AcceptHttp_10: true,
					}
					if &f.TypedConfig.HttpProtocolOptions.AcceptHttp_10 != nil {
						httpp.HttpProtocolOptions.AcceptHttp_10 = *f.TypedConfig.HttpProtocolOptions.AcceptHttp_10
					}
				}

				if &f.TypedConfig.UpgradeConfigs != nil {
					httpp.UpgradeConfigs = make([]*hcm.HttpConnectionManager_UpgradeConfig, 0)
					for _, uc := range f.TypedConfig.UpgradeConfigs {
						httpp.UpgradeConfigs = append(httpp.UpgradeConfigs, &hcm.HttpConnectionManager_UpgradeConfig{UpgradeType: uc.UpgradeType})
					}
				}
				httpp.HttpFilters = make([]*hcm.HttpFilter, 0)
				if &f.TypedConfig.HttpFilters != nil {
					for _, httpf := range f.TypedConfig.HttpFilters {
						if httpf.Name == wellknown.HTTPExternalAuthorization {
							fma := true
							if httpf.TypedConfig.FailureModeAllow != nil {
								fma = *httpf.TypedConfig.FailureModeAllow
							}
							exauthz := &authz.ExtAuthz{
								Services: &authz.ExtAuthz_GrpcService{
									GrpcService: &core.GrpcService{
										TargetSpecifier: &core.GrpcService_EnvoyGrpc_{
											EnvoyGrpc: &core.GrpcService_EnvoyGrpc{
												ClusterName: "authz_cluster",
											},
										},
									},
								},
								TransportApiVersion:    core.ApiVersion_V3,
								FailureModeAllow:       fma,
								ClearRouteCache:        true,
								IncludePeerCertificate: false,
							}
							exauthzMarshal, _ := ptypes.MarshalAny(exauthz)
							httpp.HttpFilters = append(httpp.HttpFilters, &hcm.HttpFilter{
								Name: wellknown.HTTPExternalAuthorization,
								ConfigType: &hcm.HttpFilter_TypedConfig{
									TypedConfig: exauthzMarshal,
								},
							})
						}
						if httpf.Name == wellknown.HTTPRateLimit && httpf.TypedConfig.Domain != nil {
							fmd := false
							if httpf.TypedConfig.FailureModeDeny != nil {
								fmd = *httpf.TypedConfig.FailureModeDeny
							}
							rlfilter := &lrf.RateLimit{
								Domain:          *httpf.TypedConfig.Domain,
								FailureModeDeny: fmd,
								RateLimitService: &lrfconfig.RateLimitServiceConfig{
									TransportApiVersion: core.ApiVersion_V3,
									GrpcService: &core.GrpcService{
										TargetSpecifier: &core.GrpcService_EnvoyGrpc_{
											EnvoyGrpc: &core.GrpcService_EnvoyGrpc{
												ClusterName: "ratelimit_cluster"},
										},
									},
								},
							}
							rlfilterMarshal, _ := ptypes.MarshalAny(rlfilter)
							httpp.HttpFilters = append(httpp.HttpFilters, &hcm.HttpFilter{
								Name: wellknown.HTTPRateLimit,
								ConfigType: &hcm.HttpFilter_TypedConfig{
									TypedConfig: rlfilterMarshal,
								},
							})
						}
						if httpf.Name == wellknown.HTTPRateLimit && httpf.TypedConfig.VhRateLimits != nil {
							if vhRLO, ok := lrf.RateLimitPerRoute_VhRateLimitsOptions_value[strings.ToUpper(*httpf.TypedConfig.VhRateLimits)]; ok {
								rlfilter := &lrf.RateLimitPerRoute{
									VhRateLimits: lrf.RateLimitPerRoute_VhRateLimitsOptions(vhRLO),
								}
								rlfilterMarshal, _ := ptypes.MarshalAny(rlfilter)
								httpp.HttpFilters = append(httpp.HttpFilters, &hcm.HttpFilter{
									Name: wellknown.RateLimit,
									ConfigType: &hcm.HttpFilter_TypedConfig{
										TypedConfig: rlfilterMarshal,
									},
								})
							}
						}
					}
				}
				httpp.HttpFilters = append(httpp.HttpFilters, &hcm.HttpFilter{Name: "envoy.filters.http.router"})
				if f.TypedConfig.CommonHttpProtocolOptions != nil {
					httpp.CommonHttpProtocolOptions = &core.HttpProtocolOptions{}
					if f.TypedConfig.CommonHttpProtocolOptions.MaxHeadersCount != nil {
						httpp.CommonHttpProtocolOptions.MaxHeadersCount = &wrappers.UInt32Value{Value: uint32(*f.TypedConfig.CommonHttpProtocolOptions.MaxHeadersCount)}
					}
					if f.TypedConfig.CommonHttpProtocolOptions.MaxStreamDuration != nil {
						if d, derr := time.ParseDuration(*f.TypedConfig.CommonHttpProtocolOptions.MaxStreamDuration); derr == nil {
							httpp.CommonHttpProtocolOptions.MaxStreamDuration = ptypes.DurationProto(d)
						}
					}
				}
				if f.TypedConfig.MaxRequestHeadersKb != nil {
					httpp.MaxRequestHeadersKb = &wrappers.UInt32Value{Value: uint32(*f.TypedConfig.MaxRequestHeadersKb)}
				}
				if f.TypedConfig.UseRemoteAddress != nil {
					httpp.UseRemoteAddress = &wrappers.BoolValue{Value: *f.TypedConfig.UseRemoteAddress}
				}
				if f.TypedConfig.SkipXffAppend != nil {
					httpp.SkipXffAppend = *f.TypedConfig.SkipXffAppend
				}
				if f.TypedConfig.AccessLog != nil {
					al := &als.HttpGrpcAccessLogConfig{
						CommonConfig: &als.CommonGrpcAccessLogConfig{
							TransportApiVersion: core.ApiVersion_V3,
							LogName:             l.NodeID,
							GrpcService: &core.GrpcService{
								TargetSpecifier: &core.GrpcService_EnvoyGrpc_{
									EnvoyGrpc: &core.GrpcService_EnvoyGrpc{
										ClusterName: "log_cluster"},
								},
							},
						},
					}
					if &f.TypedConfig.AccessLog[0].TypedConfig.AdditionalResponseHeadersToLog != nil {
						al.AdditionalResponseHeadersToLog = f.TypedConfig.AccessLog[0].TypedConfig.AdditionalResponseHeadersToLog
					}
					if &f.TypedConfig.AccessLog[0].TypedConfig.AdditionalRequestHeadersToLog != nil {
						al.AdditionalRequestHeadersToLog = f.TypedConfig.AccessLog[0].TypedConfig.AdditionalRequestHeadersToLog
					}
					httpp.AccessLog = make([]*v3.AccessLog, 1)
					mt, _ := ptypes.MarshalAny(al)
					httpp.AccessLog[0] = &v3.AccessLog{
						Name: "RemoteAccessLog",
						ConfigType: &v3.AccessLog_TypedConfig{
							TypedConfig: mt,
						},
					}
				}
				routeConfig := httpp.RouteSpecifier.(*hcm.HttpConnectionManager_RouteConfig)
				for _, vh := range f.TypedConfig.RouteConfig.VirtualHosts {
					routeConfig.RouteConfig.VirtualHosts = append(routeConfig.RouteConfig.VirtualHosts, &route.VirtualHost{Name: vh.Name, Routes: make([]*route.Route, 0)})
					lastVirtualHost := len(routeConfig.RouteConfig.VirtualHosts) - 1
					if &vh.Domains != nil {
						routeConfig.RouteConfig.VirtualHosts[lastVirtualHost].Domains = vh.Domains
					}
					if &vh.RateLimits != nil {
						for _, vhrl := range vh.RateLimits {
							routeConfig.RouteConfig.VirtualHosts[lastVirtualHost].RateLimits = make([]*route.RateLimit, 0)
							vhrlEntry := &route.RateLimit{
								Actions: make([]*route.RateLimit_Action, 0),
							}
							for _, vhrle := range vhrl.Actions {
								if vhrle.RequestHeaders != nil {
									vhrlEntry.Actions = append(vhrlEntry.Actions, &route.RateLimit_Action{
										ActionSpecifier: &route.RateLimit_Action_RequestHeaders_{
											RequestHeaders: &route.RateLimit_Action_RequestHeaders{
												HeaderName:    vhrle.RequestHeaders.HeaderName,
												DescriptorKey: vhrle.RequestHeaders.DescriptorKey,
												SkipIfAbsent:  vhrle.RequestHeaders.SkipIfAbsent,
											},
										},
									})
								}
								if vhrle.GenericKey != nil {
									vhrlEntry.Actions = append(vhrlEntry.Actions, &route.RateLimit_Action{
										ActionSpecifier: &route.RateLimit_Action_GenericKey_{
											GenericKey: &route.RateLimit_Action_GenericKey{
												DescriptorValue: vhrle.GenericKey.DescriptorValue,
												DescriptorKey:   vhrle.GenericKey.DescriptorKey,
											},
										},
									})
								}
							}
							routeConfig.RouteConfig.VirtualHosts[lastVirtualHost].RateLimits = append(routeConfig.RouteConfig.VirtualHosts[lastVirtualHost].RateLimits, vhrlEntry)
						}
					}
					for _, r := range vh.Routes {
						routeConfig.RouteConfig.VirtualHosts[lastVirtualHost].Routes = append(routeConfig.RouteConfig.VirtualHosts[lastVirtualHost].Routes, &route.Route{Name: r.Name})
						lastRoute := len(routeConfig.RouteConfig.VirtualHosts[lastVirtualHost].Routes) - 1
						if r.Match.Prefix != nil {
							routeConfig.RouteConfig.VirtualHosts[lastVirtualHost].Routes[lastRoute].Match = &route.RouteMatch{PathSpecifier: &route.RouteMatch_Prefix{Prefix: *r.Match.Prefix}}
						}
						if r.Match.Path != nil {
							routeConfig.RouteConfig.VirtualHosts[lastVirtualHost].Routes[lastRoute].Match = &route.RouteMatch{PathSpecifier: &route.RouteMatch_Path{Path: *r.Match.Path}}
						}
						if r.Match.Regex != nil {
							routeConfig.RouteConfig.VirtualHosts[lastVirtualHost].Routes[lastRoute].Match = &route.RouteMatch{
								PathSpecifier: &route.RouteMatch_SafeRegex{
									SafeRegex: &matcher.RegexMatcher{
										Regex: r.Match.Regex.Regex,
										EngineType: &matcher.RegexMatcher_GoogleRe2{
											GoogleRe2: &matcher.RegexMatcher_GoogleRE2{
												MaxProgramSize: &wrappers.UInt32Value{
													Value: 32,
												},
											},
										},
									},
								},
							}
						}
						if r.Match.Headers != nil {
							routeConfig.RouteConfig.VirtualHosts[lastVirtualHost].Routes[lastRoute].Match.Headers = make([]*route.HeaderMatcher, 0)
							for _, hm := range r.Match.Headers {
								hmatcher := &route.HeaderMatcher{Name: hm.Name}
								if hm.ExactMatch != nil {
									hmatcher.HeaderMatchSpecifier = &route.HeaderMatcher_ExactMatch{ExactMatch: *hm.ExactMatch}
								}
								if hm.SafeRegexMatch != nil {
									hmatcher.HeaderMatchSpecifier = &route.HeaderMatcher_SafeRegexMatch{
										SafeRegexMatch: &matcher.RegexMatcher{
											Regex: hm.SafeRegexMatch.Regex,
											EngineType: &matcher.RegexMatcher_GoogleRe2{
												GoogleRe2: &matcher.RegexMatcher_GoogleRE2{
													MaxProgramSize: &wrappers.UInt32Value{
														Value: 32,
													},
												},
											},
										},
									}
								}
								routeConfig.RouteConfig.VirtualHosts[lastVirtualHost].Routes[lastRoute].Match.Headers = append(routeConfig.RouteConfig.VirtualHosts[lastVirtualHost].Routes[lastRoute].Match.Headers, hmatcher)
							}
						}
						if r.Route != nil {
							routeConfig.RouteConfig.VirtualHosts[lastVirtualHost].Routes[lastRoute].Action = &route.Route_Route{
								Route: &route.RouteAction{
									ClusterSpecifier: &route.RouteAction_Cluster{
										Cluster: r.Route.Cluster,
									},
								},
							}
							localRoute := routeConfig.RouteConfig.VirtualHosts[lastVirtualHost].Routes[lastRoute].Action.(*route.Route_Route)
							if r.Route.Timeout != nil {
								if d, derr := time.ParseDuration(*r.Route.Timeout); derr == nil {
									localRoute.Route.Timeout = ptypes.DurationProto(d)
								}
							}

							if &r.Route.RateLimits != nil {
								for _, rrl := range r.Route.RateLimits {
									localRoute.Route.RateLimits = make([]*route.RateLimit, 0)
									rrlEntry := &route.RateLimit{
										Actions: make([]*route.RateLimit_Action, 0),
									}
									for _, rrle := range rrl.Actions {
										if rrle.RequestHeaders != nil {
											rrlEntry.Actions = append(rrlEntry.Actions, &route.RateLimit_Action{
												ActionSpecifier: &route.RateLimit_Action_RequestHeaders_{
													RequestHeaders: &route.RateLimit_Action_RequestHeaders{
														HeaderName:    rrle.RequestHeaders.HeaderName,
														DescriptorKey: rrle.RequestHeaders.DescriptorKey,
														SkipIfAbsent:  rrle.RequestHeaders.SkipIfAbsent,
													},
												},
											})
										}
										if rrle.GenericKey != nil {
											rrlEntry.Actions = append(rrlEntry.Actions, &route.RateLimit_Action{
												ActionSpecifier: &route.RateLimit_Action_GenericKey_{
													GenericKey: &route.RateLimit_Action_GenericKey{
														DescriptorValue: rrle.GenericKey.DescriptorValue,
														DescriptorKey:   rrle.GenericKey.DescriptorKey,
													},
												},
											})
										}
									}
									localRoute.Route.RateLimits = append(localRoute.Route.RateLimits, rrlEntry)
								}
							}

							if r.Route.PrefixRewrite != nil {
								localRoute.Route.PrefixRewrite = *r.Route.PrefixRewrite
							}
							if r.Route.HostRewrite != nil {
								localRoute.Route.HostRewriteSpecifier = &route.RouteAction_HostRewriteLiteral{HostRewriteLiteral: *r.Route.HostRewrite}
							}
							if r.Route.RegexRewrite != nil {
								localRoute.Route.RegexRewrite = &matcher.RegexMatchAndSubstitute{
									Pattern: &matcher.RegexMatcher{
										Regex: r.Route.RegexRewrite.Pattern.Regex,
										EngineType: &matcher.RegexMatcher_GoogleRe2{
											GoogleRe2: &matcher.RegexMatcher_GoogleRE2{
												MaxProgramSize: &wrappers.UInt32Value{
													Value: 32,
												},
											},
										},
									},
									Substitution: r.Route.RegexRewrite.Substitution,
								}
							}
							if r.Route.HashPolicy != nil {
								localRoute.Route.HashPolicy = make([]*route.RouteAction_HashPolicy, 0)
								for _, hp := range r.Route.HashPolicy {
									hash := &route.RouteAction_HashPolicy{
										Terminal: hp.Terminal,
									}
									if hp.Cookie != nil {
										if d, derr := time.ParseDuration(hp.Cookie.Ttl); derr == nil {
											hash.PolicySpecifier = &route.RouteAction_HashPolicy_Cookie_{
												Cookie: &route.RouteAction_HashPolicy_Cookie{
													Name: hp.Cookie.Name,
													Ttl:  ptypes.DurationProto(d),
												},
											}
										}
									}
									if hp.ConnectionProperties != nil {
										hash.PolicySpecifier = &route.RouteAction_HashPolicy_ConnectionProperties_{
											ConnectionProperties: &route.RouteAction_HashPolicy_ConnectionProperties{
												SourceIp: hp.ConnectionProperties.SourceIp,
											},
										}
									}
									if hp.QueryParameter != nil {
										hash.PolicySpecifier = &route.RouteAction_HashPolicy_QueryParameter_{
											QueryParameter: &route.RouteAction_HashPolicy_QueryParameter{
												Name: hp.QueryParameter.Name,
											},
										}
									}
									if hp.Header != nil {
										if hp.Header.RegexRewrite != nil {
											hash.PolicySpecifier = &route.RouteAction_HashPolicy_Header_{
												Header: &route.RouteAction_HashPolicy_Header{
													HeaderName: hp.Header.HeaderName,
													RegexRewrite: &matcher.RegexMatchAndSubstitute{
														Pattern: &matcher.RegexMatcher{
															Regex: hp.Header.RegexRewrite.Pattern.Regex,
															EngineType: &matcher.RegexMatcher_GoogleRe2{
																GoogleRe2: &matcher.RegexMatcher_GoogleRE2{
																	MaxProgramSize: &wrappers.UInt32Value{
																		Value: 32,
																	},
																},
															},
														},
														Substitution: hp.Header.RegexRewrite.Substitution,
													},
												},
											}
										} else {
											hash.PolicySpecifier = &route.RouteAction_HashPolicy_Header_{
												Header: &route.RouteAction_HashPolicy_Header{
													HeaderName: hp.Header.HeaderName,
												},
											}
										}
									}
									localRoute.Route.HashPolicy = append(localRoute.Route.HashPolicy, hash)
								}
							}
						}
						if r.Redirect != nil {
							routeConfig.RouteConfig.VirtualHosts[lastVirtualHost].Routes[lastRoute].Action = &route.Route_Redirect{Redirect: &route.RedirectAction{SchemeRewriteSpecifier: &route.RedirectAction_HttpsRedirect{HttpsRedirect: false}}}
							redir := routeConfig.RouteConfig.VirtualHosts[lastVirtualHost].Routes[lastRoute].Action.(*route.Route_Redirect)
							if r.Redirect.HttpsRedirect != nil {
								rewriteSpecifier := redir.Redirect.SchemeRewriteSpecifier.(*route.RedirectAction_HttpsRedirect)
								rewriteSpecifier.HttpsRedirect = true
							}
							if r.Redirect.HostRedirect != nil {
								redir.Redirect.HostRedirect = *r.Redirect.HostRedirect
							}
							if r.Redirect.PathRedirect != nil {
								redir.Redirect.PathRewriteSpecifier = &route.RedirectAction_PathRedirect{PathRedirect: *r.Redirect.PathRedirect}
							}
							if r.Redirect.PrefixRewrite != nil {
								redir.Redirect.PathRewriteSpecifier = &route.RedirectAction_PrefixRewrite{PrefixRewrite: *r.Redirect.PrefixRewrite}
							}
						}
					}
				}
				mt, _ := ptypes.MarshalAny(httpp)
				cfc.Filters = append(cfc.Filters, &listener.Filter{
					Name: wellknown.HTTPConnectionManager,
					ConfigType: &listener.Filter_TypedConfig{
						TypedConfig: mt,
					},
				})
			}
		}
		if fc.TransportSocket != nil {
			tls := &auth.DownstreamTlsContext{
				CommonTlsContext: &auth.CommonTlsContext{
					//					TlsCertificates: make([]*auth.TlsCertificate, 0),
				},
			}
			if &fc.TransportSocket.TypedConfig.CommonTlsContext.SdsConfig != nil {
				tls.CommonTlsContext.TlsCertificateSdsSecretConfigs = make([]*auth.SdsSecretConfig, 0)
				for _, sds := range fc.TransportSocket.TypedConfig.CommonTlsContext.SdsConfig {
					tls.CommonTlsContext.TlsCertificateSdsSecretConfigs = append(tls.CommonTlsContext.TlsCertificateSdsSecretConfigs, &auth.SdsSecretConfig{
						Name: sds.Name,
						SdsConfig: &core.ConfigSource{
							ResourceApiVersion: core.ApiVersion_V3,
							ConfigSourceSpecifier: &core.ConfigSource_Ads{
								Ads: &core.AggregatedConfigSource{},
							},
						},
					})
				}
			}
			if &fc.TransportSocket.TypedConfig.CommonTlsContext.TlsCertificates != nil {
				tls.CommonTlsContext.TlsCertificates = make([]*auth.TlsCertificate, 0)
				for _, tlsCert := range fc.TransportSocket.TypedConfig.CommonTlsContext.TlsCertificates {
					if &tlsCert.CertificateChain != nil && &tlsCert.PrivateKey != nil {
						cert := &auth.TlsCertificate{
							CertificateChain: &core.DataSource{
								Specifier: &core.DataSource_InlineString{
									InlineString: tlsCert.CertificateChain.InlineString,
								},
							},
							PrivateKey: &core.DataSource{
								Specifier: &core.DataSource_InlineString{
									InlineString: tlsCert.PrivateKey.InlineString,
								},
							},
						}
						if tlsCert.Password != nil {
							cert.Password = &core.DataSource{Specifier: &core.DataSource_InlineString{InlineString: tlsCert.Password.InlineString}}
						}
						tls.CommonTlsContext.TlsCertificates = append(tls.CommonTlsContext.TlsCertificates, cert)
					}
				}
			}
			if fc.TransportSocket.TypedConfig.CommonTlsContext.RequestClientCertificate != nil {
				tls.RequireClientCertificate = &wrappers.BoolValue{Value: true}
			}
			if fc.TransportSocket.TypedConfig.CommonTlsContext.TlsParams != nil {
				tls.CommonTlsContext.TlsParams = &auth.TlsParameters{}
				if fc.TransportSocket.TypedConfig.CommonTlsContext.TlsParams.TlsMinimumProtocolVersion != nil {
					if v, ok := auth.TlsParameters_TlsProtocol_value[*fc.TransportSocket.TypedConfig.CommonTlsContext.TlsParams.TlsMinimumProtocolVersion]; ok {
						tls.CommonTlsContext.TlsParams.TlsMinimumProtocolVersion = auth.TlsParameters_TlsProtocol(v)
					}
				}
				if fc.TransportSocket.TypedConfig.CommonTlsContext.TlsParams.TlsMaximumProtocolVersion != nil {
					if v, ok := auth.TlsParameters_TlsProtocol_value[*fc.TransportSocket.TypedConfig.CommonTlsContext.TlsParams.TlsMaximumProtocolVersion]; ok {
						tls.CommonTlsContext.TlsParams.TlsMaximumProtocolVersion = auth.TlsParameters_TlsProtocol(v)
					}
				}
				if &fc.TransportSocket.TypedConfig.CommonTlsContext.TlsParams.CipherSuites != nil {
					tls.CommonTlsContext.TlsParams.CipherSuites = fc.TransportSocket.TypedConfig.CommonTlsContext.TlsParams.CipherSuites
				}
				if fc.TransportSocket.TypedConfig.CommonTlsContext.ValidationContext != nil {
					if fc.TransportSocket.TypedConfig.CommonTlsContext.ValidationContext.TrustedCa != nil {
						tls.CommonTlsContext.ValidationContextType = &auth.CommonTlsContext_ValidationContext{
							ValidationContext: &auth.CertificateValidationContext{
								TrustedCa: &core.DataSource{
									Specifier: &core.DataSource_InlineString{
										InlineString: fc.TransportSocket.TypedConfig.CommonTlsContext.ValidationContext.TrustedCa.InlineString,
									},
								},
							},
						}
					}
					if fc.TransportSocket.TypedConfig.CommonTlsContext.ValidationContext.SdsConfig != nil {
						tls.CommonTlsContext.ValidationContextType = &auth.CommonTlsContext_ValidationContextSdsSecretConfig{
							ValidationContextSdsSecretConfig: &auth.SdsSecretConfig{
								Name: fc.TransportSocket.TypedConfig.CommonTlsContext.ValidationContext.SdsConfig.Name,
								SdsConfig: &core.ConfigSource{
									ConfigSourceSpecifier: &core.ConfigSource_Ads{
										Ads: &core.AggregatedConfigSource{},
									},
								},
							},
						}
					}
				}
			}
			mt, _ := ptypes.MarshalAny(tls)
			cfc.TransportSocket = &core.TransportSocket{
				Name: "envoy.transport_sockets.tls",
				ConfigType: &core.TransportSocket_TypedConfig{
					TypedConfig: mt,
				},
			}
		}
		res.FilterChains = append(res.FilterChains, cfc)
	}
	return res
}

func parseEndpoint(e apitypes.EndpointsConfigType) *endpoint.ClusterLoadAssignment {
	res := &endpoint.ClusterLoadAssignment{
		ClusterName: e.ClusterName,
		Endpoints:   make([]*endpoint.LocalityLbEndpoints, 0),
	}
	if e.Policy != nil {
		if e.Policy.OverprovisioningFactor != nil {
			res.Policy = &endpoint.ClusterLoadAssignment_Policy{
				OverprovisioningFactor: &wrappers.UInt32Value{
					Value: uint32(*e.Policy.OverprovisioningFactor),
				},
			}
		}
	}
	for _, en := range e.Endpoints {
		res.Endpoints = append(res.Endpoints, &endpoint.LocalityLbEndpoints{LbEndpoints: make([]*endpoint.LbEndpoint, 0)})
		lastEndpoint := len(res.Endpoints) - 1
		if &en.Priority != nil {
			res.Endpoints[lastEndpoint].Priority = uint32(en.Priority)
		}
		for _, lben := range en.LbEndpoints {
			if lben.LoadBalancingWeight != nil {
				res.Endpoints[lastEndpoint].LbEndpoints = append(res.Endpoints[lastEndpoint].LbEndpoints, &endpoint.LbEndpoint{
					LoadBalancingWeight: &wrappers.UInt32Value{
						Value: uint32(*lben.LoadBalancingWeight),
					},
					HostIdentifier: &endpoint.LbEndpoint_Endpoint{
						Endpoint: &endpoint.Endpoint{
							Address: &core.Address{
								Address: &core.Address_SocketAddress{
									SocketAddress: &core.SocketAddress{
										Address:  lben.Endpoint.Address.SocketAddress.Address,
										Protocol: core.SocketAddress_TCP,
										PortSpecifier: &core.SocketAddress_PortValue{
											PortValue: uint32(lben.Endpoint.Address.SocketAddress.PortValue),
										},
									},
								},
							},
						},
					},
				})
			} else {
				res.Endpoints[lastEndpoint].LbEndpoints = append(res.Endpoints[lastEndpoint].LbEndpoints, &endpoint.LbEndpoint{
					HostIdentifier: &endpoint.LbEndpoint_Endpoint{
						Endpoint: &endpoint.Endpoint{
							Address: &core.Address{
								Address: &core.Address_SocketAddress{
									SocketAddress: &core.SocketAddress{
										Address:  lben.Endpoint.Address.SocketAddress.Address,
										Protocol: core.SocketAddress_TCP,
										PortSpecifier: &core.SocketAddress_PortValue{
											PortValue: uint32(lben.Endpoint.Address.SocketAddress.PortValue),
										},
									},
								},
							},
						},
					},
				})
			}
		}
	}
	return res
}

func parseCluster(c apitypes.ClusterConfigType) *cluster.Cluster {
	d, derr := time.ParseDuration(c.ConnectTimeout)
	if derr != nil {
		d = time.Duration(5 * time.Second)
	}
	res := &cluster.Cluster{
		Name:           c.Name,
		ConnectTimeout: ptypes.DurationProto(d),
		CommonLbConfig: &cluster.Cluster_CommonLbConfig{
			HealthyPanicThreshold: &typesv3.Percent{
				Value: 0,
			},
		},
		ClusterDiscoveryType: &cluster.Cluster_Type{Type: cluster.Cluster_EDS},
		EdsClusterConfig: &cluster.Cluster_EdsClusterConfig{
			EdsConfig: &core.ConfigSource{
				ResourceApiVersion: core.ApiVersion_V3,
				ConfigSourceSpecifier: &core.ConfigSource_Ads{
					Ads: &core.AggregatedConfigSource{},
				},
			},
		},
	}
	//protocol options extension
	protoopts := &upstreamprotoopts.HttpProtocolOptions{}
	protoopts.UpstreamProtocolOptions = &upstreamprotoopts.HttpProtocolOptions_UseDownstreamProtocolConfig{UseDownstreamProtocolConfig: &upstreamprotoopts.HttpProtocolOptions_UseDownstreamHttpConfig{}}
	if c.CommonHttpProtocolOptions != nil {
		protoopts.CommonHttpProtocolOptions = &core.HttpProtocolOptions{}
		if c.CommonHttpProtocolOptions.MaxHeadersCount != nil {
			protoopts.CommonHttpProtocolOptions.MaxHeadersCount = &wrappers.UInt32Value{Value: uint32(*c.CommonHttpProtocolOptions.MaxHeadersCount)}
		}
		if c.CommonHttpProtocolOptions.MaxStreamDuration != nil {
			if d, derr := time.ParseDuration(*c.CommonHttpProtocolOptions.MaxStreamDuration); derr == nil {
				protoopts.CommonHttpProtocolOptions.MaxStreamDuration = ptypes.DurationProto(d)
			}
		}
	}
	if val, ok := c.TypedExtensionProtocolOptions["envoy.extensions.upstreams.http.v3.HttpProtocolOptions"]; ok {
		if val != nil {
			if val.ExplicitHttpConfig != nil {
				if val.ExplicitHttpConfig.HttpProtocolOptions != nil {
					protoopts.UpstreamProtocolOptions = &upstreamprotoopts.HttpProtocolOptions_ExplicitHttpConfig_{ExplicitHttpConfig: &upstreamprotoopts.HttpProtocolOptions_ExplicitHttpConfig{ProtocolConfig: &upstreamprotoopts.HttpProtocolOptions_ExplicitHttpConfig_HttpProtocolOptions{HttpProtocolOptions: &core.Http1ProtocolOptions{}}}}
				} else if val.ExplicitHttpConfig.Http2ProtocolOptions != nil {
					protoopts.UpstreamProtocolOptions = &upstreamprotoopts.HttpProtocolOptions_ExplicitHttpConfig_{ExplicitHttpConfig: &upstreamprotoopts.HttpProtocolOptions_ExplicitHttpConfig{ProtocolConfig: &upstreamprotoopts.HttpProtocolOptions_ExplicitHttpConfig_Http2ProtocolOptions{Http2ProtocolOptions: &core.Http2ProtocolOptions{}}}}
				} else {
					protoopts.UpstreamProtocolOptions = &upstreamprotoopts.HttpProtocolOptions_ExplicitHttpConfig_{}
				}
			}
			if val.CommonHttpProtocolOptions != nil {
				protoopts.CommonHttpProtocolOptions = &core.HttpProtocolOptions{}
				if val.CommonHttpProtocolOptions.MaxHeadersCount != nil {
					protoopts.CommonHttpProtocolOptions.MaxHeadersCount = &wrappers.UInt32Value{Value: uint32(*val.CommonHttpProtocolOptions.MaxHeadersCount)}
				}
				if val.CommonHttpProtocolOptions.MaxStreamDuration != nil {
					if d, derr := time.ParseDuration(*val.CommonHttpProtocolOptions.MaxStreamDuration); derr == nil {
						protoopts.CommonHttpProtocolOptions.MaxStreamDuration = ptypes.DurationProto(d)
					}
				}
			}
		}
	}
	//	protoopts.UpstreamHttpProtocolOptions = &core.UpstreamHttpProtocolOptions{}

	res.TypedExtensionProtocolOptions = make(map[string]*any.Any)
	mt, _ := ptypes.MarshalAny(protoopts)
	res.TypedExtensionProtocolOptions["envoy.extensions.upstreams.http.v3.HttpProtocolOptions"] = mt

	if c.MaxRequestsPerConnection != nil {
		res.MaxRequestsPerConnection = &wrappers.UInt32Value{Value: uint32(*c.MaxRequestsPerConnection)}
	}
	if c.CommonLbConfig != nil {
		if &c.CommonLbConfig.HealthyPanicThreshold != nil {
			res.CommonLbConfig.HealthyPanicThreshold.Value = float64(c.CommonLbConfig.HealthyPanicThreshold.Value)
		}
		if c.CommonLbConfig.LocalityWeightedLbConfig != nil {
			res.CommonLbConfig.LocalityConfigSpecifier = &cluster.Cluster_CommonLbConfig_LocalityWeightedLbConfig_{}
		}
	}
	if c.PerConnectionBufferLimitBytes != nil {
		res.PerConnectionBufferLimitBytes = &wrappers.UInt32Value{
			Value: uint32(*c.PerConnectionBufferLimitBytes),
		}
	}
	if c.LbPolicy == nil {
		res.LbPolicy = cluster.Cluster_ROUND_ROBIN
	} else {
		switch *c.LbPolicy {
		case "LEAST_REQUEST":
			res.LbPolicy = cluster.Cluster_LEAST_REQUEST
		case "MAGLEV":
			res.LbPolicy = cluster.Cluster_MAGLEV
		default:
			res.LbPolicy = cluster.Cluster_ROUND_ROBIN
		}
	}
	if c.LeastRequestLbConfig != nil {
		res.LbConfig = &cluster.Cluster_LeastRequestLbConfig_{LeastRequestLbConfig: &cluster.Cluster_LeastRequestLbConfig{ChoiceCount: &wrappers.UInt32Value{Value: uint32(c.LeastRequestLbConfig.ChoiceCount)}}}
	}
	if c.TransportSocket != nil {
		tls := &auth.UpstreamTlsContext{}
		if c.TransportSocket.TypedConfig.Sni != nil {
			tls.Sni = *c.TransportSocket.TypedConfig.Sni
		}
		mt, _ := ptypes.MarshalAny(tls)
		res.TransportSocket = &core.TransportSocket{
			Name: "envoy.transport_sockets.tls",
			ConfigType: &core.TransportSocket_TypedConfig{
				TypedConfig: mt,
			},
		}
	}
	if &c.HealthChecks != nil {
		res.HealthChecks = make([]*core.HealthCheck, 0)
		for _, hc := range c.HealthChecks {
			tm, tmerr := time.ParseDuration(hc.Timeout)
			if tmerr != nil {
				tm = time.Duration(5 * time.Second)
			}
			inv, inverr := time.ParseDuration(hc.Interval)
			if inverr != nil {
				inv = time.Duration(5 * time.Second)
			}
			thc := &core.HealthCheck{
				HealthyThreshold: &wrappers.UInt32Value{
					Value: uint32(hc.HealthyThreshold),
				},
				UnhealthyThreshold: &wrappers.UInt32Value{
					Value: uint32(hc.UnhealthyThreshold),
				},
				Timeout:  ptypes.DurationProto(tm),
				Interval: ptypes.DurationProto(inv),
			}
			if hc.HttpHealthCheck != nil {
				httpHC := &core.HealthCheck_HttpHealthCheck{
					Path: hc.HttpHealthCheck.Path,
				}
				if hc.HttpHealthCheck.Host != nil {
					httpHC.Host = *hc.HttpHealthCheck.Host
				}
				if &hc.HttpHealthCheck.ExpectedStatuses != nil {
					httpHC.ExpectedStatuses = make([]*typesv3.Int64Range, 0)
					for _, hces := range hc.HttpHealthCheck.ExpectedStatuses {
						httpHC.ExpectedStatuses = append(httpHC.ExpectedStatuses, &typesv3.Int64Range{Start: hces.Start, End: hces.End})
					}
				}
				thc.HealthChecker = &core.HealthCheck_HttpHealthCheck_{
					HttpHealthCheck: httpHC,
				}
			} else {
				thc.HealthChecker = &core.HealthCheck_TcpHealthCheck_{TcpHealthCheck: &core.HealthCheck_TcpHealthCheck{}}
			}
			res.HealthChecks = append(res.HealthChecks, thc)
		}
	}
	return res
}
func LoadNode(ctx context.Context, node string) ([]types.Resource, []types.Resource, []types.Resource, []types.Resource, error) {
	lres := make([]types.Resource, 0)
	cres := make([]types.Resource, 0)
	eres := make([]types.Resource, 0)
	sres := make([]types.Resource, 0)
	ctxTimeout, _ := context.WithTimeout(ctx, drivers.ConnectTimeout)
	if cursor, err := drivers.Cli.Database("envoy").Collection("listeners").Find(ctxTimeout, bson.M{"nodeId": node}); err == nil {
		for cursor.TryNext(ctxTimeout) {
			var listener apitypes.ListenerConfigType
			if err := cursor.Decode(&listener); err == nil {
				lres = append(lres, parseListener(listener))
			}
		}
		cursor.Close(ctxTimeout)
	}
	if cursor, err := drivers.Cli.Database("envoy").Collection("clusters").Find(ctxTimeout, bson.M{"nodeId": node}); err == nil {
		for cursor.TryNext(ctxTimeout) {
			var cluster apitypes.ClusterConfigType
			if err := cursor.Decode(&cluster); err == nil {
				cres = append(cres, parseCluster(cluster))
			}
		}
		cursor.Close(ctxTimeout)
	}
	if cursor, err := drivers.Cli.Database("envoy").Collection("endpoints").Find(ctxTimeout, bson.M{"nodeId": node}); err == nil {
		for cursor.TryNext(ctxTimeout) {
			var endpoint apitypes.EndpointsConfigType
			if err := cursor.Decode(&endpoint); err == nil {
				eres = append(eres, parseEndpoint(endpoint))
			}
		}
		cursor.Close(ctxTimeout)
	}
	if cursor, err := drivers.Cli.Database("envoy").Collection("secrets").Find(ctxTimeout, bson.M{"nodeId": node}); err == nil {
		for cursor.TryNext(ctxTimeout) {
			var secret apitypes.SecretConfigType
			if err := cursor.Decode(&secret); err == nil {
				sres = append(sres, parseSecret(secret))
			}
		}
		cursor.Close(ctxTimeout)
	}

	return lres, cres, eres, sres, nil
}
