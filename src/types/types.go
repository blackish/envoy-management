package types

import ()

var LoggingModules = map[string]bool{
	"client":         true,
	"config":         true,
	"connection":     true,
	"conn_handler":   true,
	"decompression":  true,
	"grpc":           true,
	"hc":             true,
	"health_checker": true,
	"http":           true,
	"http2":          true,
	"init":           true,
	"main":           true,
	"misc":           true,
	"quic":           true,
	"quic_stream":    true,
	"pool":           true,
	"router":         true,
	"runtime":        true,
	"stats":          true,
	"secret":         true,
	"tracing":        true,
	"upstream":       true,
	"udp":            true,
}
var LoggingLevels = map[string]bool{
	"info":  true,
	"debug": true,
	"trace": true,
}

type WorkerSyncConfigType struct {
	Name    string `json:"name" bson:"name"`
	Version string `json:"version" bson:"version"`
	Status  string `json:"status" bson:"status"`
	Updated string `json:"updated" bson:"updated"`
}

type NodeStaticConfigType struct {
	Name      string   `json:"name" bson:"name"`
	Address   string   `json:"address" bson:"address"`
	Version   string   `json:"version" bson:"version"`
	Addresses []string `json:"addresses" bson:"addresses"`
}

type NodeInfoType struct {
	Version            string `json:"version" bson:"version"`
	State              string `json:"state" bson:"state"`
	UptimeCurrentEpoch string `json:"uptime_current_epoch" bson:"uptime_current_epoch"`
	UptimeAllEpoch     string `json:"uptime_all_epochs" bson:"uptime_all_epochs"`
}

type LoggingLiveType struct {
	Module   string `json:"module" bson:"module"`
	Loglevel string `json:"loglevel" bson:"loglevel"`
}

type ListenerLiveType struct {
	Name    string `json:"listener.name" bson:"listener.name"`
	Address string `json:"listener.address" bson:"listener.address"`
}

type EndpointLiveType struct {
	Address         string `json:"endpoint.address" bson:"endpoint.address"`
	Priority        int    `json:"endpoint.priority" bson:"endpoint.priority"`
	Health          string `json:"endpoint.health" bson:"endpoint.health"`
	RequestsSuccess uint64 `json:"endpoint.rq_success" bson:"endpoint.rq_success"`
	RequestsTotal   uint64 `json:"endpoint.rq_total" bson:"endpoint.rq_total"`
}

type ClustersLiveType struct {
	Name      string             `json:"cluster.name" bson:"cluster.name"`
	Endpoints []EndpointLiveType `json:"cluster.endpoints" bson:"cluster.endpoints"`
}

type NodeConfigType struct {
	Address string `json:"address" bson:"address"`
}
type EndpointAddressType struct {
	SocketAddress SocketAddressConfigType `json:"socketAddress" bson:"socketAddress"`
}
type EndpointV2ConfigType struct {
	Address EndpointAddressType `json:"address" bson:"address"`
}
type EndpointConfigType struct {
	Endpoint            EndpointV2ConfigType `json:"endpoint" bson:"endpoint"`
	LoadBalancingWeight *int                 `json:"loadBalancingWeight" bson:"loadBalancingWeight"`
}
type LbEndpointConfigType struct {
	Priority    int                  `json:"priority" bson:"priority" omitempty`
	LbEndpoints []EndpointConfigType `json:"lbEndpoints" bson:"lbEndpoints"`
}
type EndpointPolicyConfigType struct {
	OverprovisioningFactor *int `json:"overprovisioningFactor" bson:"overprovisioningFactor" omitempty`
}

type EndpointsConfigType struct {
	NodeID      string                    `json:"nodeId" bson:"nodeId" omitempty`
	ClusterName string                    `json:"clusterName" bson:"clusterName"`
	Endpoints   []LbEndpointConfigType    `json:"endpoints" bson:"endpoints"`
	Policy      *EndpointPolicyConfigType `json:"policy" bson:"policy" omitempty`
}

type Int64RangeConfigType struct {
	Start int64 `json:"start" bson:"start"`
	End   int64 `json:"end" bson:"end"`
}

type HttpHealthCheckConfigType struct {
	Path             string                 `json:"path" bson:"path"`
	Host             *string                `json:"host" bson:"host"`
	ExpectedStatuses []Int64RangeConfigType `json:"expectedStatuses" bson:"expectedStatuses"`
}

type HealthChecksConfigType struct {
	Timeout            string                     `json:"timeout" bson:"timeout"`
	Interval           string                     `json:"interval" bson:"interval"`
	UnhealthyThreshold int                        `json:"unhealthyThreshold" bson:"unhealthyThreshold"`
	HealthyThreshold   int                        `json:"healthyThreshold" bson:"healthyThreshold"`
	HttpHealthCheck    *HttpHealthCheckConfigType `json:"httpHealthCheck" bson:"httpHealthCheck" omitempty`
}
type LeastRequestLbConfigType struct {
	ChoiceCount int `json:"choiceCount" bson:"choiceCount"`
}
type PercentValueConfigType struct {
	Value int `json:"value" bson:"value"`
}
type LocalityWeightedLbConfigType struct {
}

type CommonLbConfigConfigType struct {
	HealthyPanicThreshold    PercentValueConfigType        `json:"healthyPanicThreshold" bson:"healthyPanicThreshold" omitempty`
	LocalityWeightedLbConfig *LocalityWeightedLbConfigType `json: "localityWeightedLbConfig" bson:"localityWeightedLbConfig" omitempty`
}

type HttpProtocolOptionsConfigType struct {
	AcceptHttp_10 *bool `json:"acceptHttp10" bson:"acceptHttp10"`
}

type Http2ProtocolOptionsConfigType struct {
}

type UseDownstreamProtocolConfigConfigType struct {
}

type ExplicitHttpConfigConfigType struct {
	Http2ProtocolOptions *Http2ProtocolOptionsConfigType `json:"http2ProtocolOptions" bson:"http2ProtocolOptions" omytempty`
	HttpProtocolOptions  *HttpProtocolOptionsConfigType  `json:"httpProtocolOptions" bson:"httpProtocolOptions" omytempty`
}

type TypedExtensionProtocolOptionsConfigType struct {
	CommonHttpProtocolOptions   *CommonHttpProtocolOptionsConfigType   `json:"commonHttpProtocolOptions" bson:"commonHttpProtocolOptions" omitempty`
	UseDownstreamProtocolConfig *UseDownstreamProtocolConfigConfigType `json:"useDownstreamProtocolConfig" bson:"useDownstreamProtocolConfig" omitempty`
	ExplicitHttpConfig          *ExplicitHttpConfigConfigType          `json:"explicitHttpConfig" bson:"explicitHttpConfig" omitempty`
}

type CircuitBreakers_ThresholdsConfigType struct {
	Priority           int32   `json:"priority" bson:"priority"`
	MaxConnections     *uint32 `json:"maxConnections" bson:"maxConnections" omitempty`
	MaxPendingRequests *uint32 `json:"maxPendingRequests" bson:"maxPendingRequests" omitempty`
	MaxRequests        *uint32 `json:"maxRequests" bson:"maxRequests" omitempty`
	MaxRetries         *uint32 `json:"maxRetries" bson:"maxRetries" omitempty`
}

type CircuitBreakersConfigType struct {
	Thresholds []CircuitBreakers_ThresholdsConfigType `json:"thresholds" bson:"thresholds" omitempty`
}

type ClusterConfigType struct {
	NodeID                        string                                              `json:"nodeId" bson:"nodeId" omitempty`
	PerConnectionBufferLimitBytes *int                                                `json:"perConnectionBufferLimitBytes" bson:"perConnectionBufferLimitBytes" omitempty`
	Name                          string                                              `json:"name" bson:"name"`
	ConnectTimeout                string                                              `json:"connectTimeout" bson:"connectTimeout"`
	LbPolicy                      *string                                             `json:"lbPolicy" bson:"lbPolicy"`
	LeastRequestLbConfig          *LeastRequestLbConfigType                           `json:"leastRequestLbConfig" bson:"leastRequestLbConfig" omitempty`
	TransportSocket               *TransportSocketConfigType                          `json:"transportSocket" bson:"transportSocket" omitempty`
	HealthChecks                  []HealthChecksConfigType                            `json:"healthChecks" bson:"healthChecks" omitempty`
	CommonLbConfig                *CommonLbConfigConfigType                           `json:"commonLbConfig" bson:"commonLbConfig" omitempty`
	MaxRequestsPerConnection      *int                                                `json:"maxRequestsPerConnection" bson:"maxRequestsPerConnection"`
	CommonHttpProtocolOptions     *CommonHttpProtocolOptionsConfigType                `json:"commonHttpProtocolOptions" bson:"commonHttpProtocolOptions"`
	TypedExtensionProtocolOptions map[string]*TypedExtensionProtocolOptionsConfigType `json:"typedExtensionProtocolOptions" bson:"typedExtensionProtocolOptions"`
	CircuitBreakers               *CircuitBreakersConfigType                          `json:"circuitBreakers" bson:"circuitBreakers" omitempty`
}

type SocketAddressConfigType struct {
	Address   string `json:"address" bson:"address"`
	PortValue int    `json:"portValue" bson:"portValue"`
}
type AddressConfigType struct {
	SocketAddress SocketAddressConfigType `json:"socketAddress" bson:"socketAddress"`
}
type PrefixConfigType struct {
	AddressPrefix string `json:"addressPrefix" bson:"addressPrefix"`
	PrefixLen     int    `json:"prefixLen" bson:"prefixLen"`
}
type FilterChainMatchConfigType struct {
	ServerNames        []string           `json:"serverNames" bson:"serverNames" omitempty`
	SourcePrefixRanges []PrefixConfigType `json:"sourcePrefixRanges" bson:"sourcePrefixRanges" omitempty`
	PrefixRanges       []PrefixConfigType `json:"prefixRanges" bson:"prefixRanges" omitempty`
}
type RegexConfigType struct {
	Regex string `json:"regex" bson:"regex"`
}
type HeaderMatcherConfigType struct {
	Name           string           `json:"name" bson:"name"`
	ExactMatch     *string          `json:"exactMatch" bson:"exactMatch"`
	SafeRegexMatch *RegexConfigType `json:"safeRegexMatch" bson:"safeRegexMatch"`
}
type MatchConfigType struct {
	Prefix  *string                   `json:"prefix" bson:"prefix" omitempty`
	Path    *string                   `json:"path" bson:"path" omitempty`
	Regex   *RegexConfigType          `json:"safeRegex" bson:"safeRegex" omitempty`
	Headers []HeaderMatcherConfigType `json:"headers" bson:"headers" omitempty`
}
type RegexRewriteConfigType struct {
	Substitution string          `json:"substitute" bson:"substitution"`
	Pattern      RegexConfigType `json:"pattern" bson:"pattern"`
}
type RouteActionConfigType struct {
	Timeout       *string                 `json:"timeout" bson:"timeout" omitempty`
	Cluster       string                  `json:"cluster" bson:"cluster"`
	PrefixRewrite *string                 `json:"prefixRewrite" bson:"prefixRewrite" omitempty`
	HostRewrite   *string                 `json:"hostRewriteLiteral" bson:"hostRewriteLiteral" omitempty`
	RegexRewrite  *RegexRewriteConfigType `json:"regexRewrite" bson:"regexRewrite" omitempty`
	HashPolicy    []HashPolicyConfigType  `json:"hashPolicy" bson:"hashPolicy" omitempty`
	RateLimits    []RateLimitConfigType   `json:"rateLimits" bson:"rateLimits" omitempty`
}
type HttpRedirectConfigType struct {
	HttpsRedirect *bool   `json:"httpsRedirect" bson:"httpsRedirect" omitempty`
	HostRedirect  *string `json:"hostRedirect" bson:"hostRedirect" omitempty`
	PathRedirect  *string `json:"pathRedirect" bson:"pathRedirect" omitempty`
	PrefixRewrite *string `json:"prefixRewrite" bson:"prefixRewrite" omitempty`
}
type HashPolicyHeaderConfigType struct {
	HeaderName   string                  `json:"headerName" bson:"headerName"`
	RegexRewrite *RegexRewriteConfigType `json:"regexRewrite" bson:"regexRewrite"`
}
type HashPolicyCookieConfigType struct {
	Name string `json:"name" bson:"name"`
	Ttl  string `json:"ttl" bson:"ttl"`
}
type HashPolicyConnectionPropertiesConfigType struct {
	SourceIp bool `json:"sourceIp" bson:"sourceIp"`
}
type HashPolicyQueryConfigType struct {
	Name string `json:"name" bson:"name"`
}
type HashPolicyConfigType struct {
	Terminal             bool                                      `json:"terminal" bson:"terminal"`
	Cookie               *HashPolicyCookieConfigType               `json:"cookie" bson:"cookie" omitempty`
	ConnectionProperties *HashPolicyConnectionPropertiesConfigType `json:"connectionProperties" bson:"connectionProperties" omitempty`
	QueryParameter       *HashPolicyQueryConfigType                `json:"queryParameter" bson:"queryParameter" omitempty`
	Header               *HashPolicyHeaderConfigType               `json:"header" bson:"header" omitempty`
}
type RouteConfigType struct {
	Name     string                  `json:"name" bson:"name"`
	Match    *MatchConfigType        `json:"match" bson:"match" omitempty`
	Route    *RouteActionConfigType  `json:"route" bson:"route" omitempty`
	Redirect *HttpRedirectConfigType `json:"redirect" bson:"redirect" omitempty`
}
type RateLimitDescriptorEntryConfigType struct {
	Key   string `json:"key" bson:"key"`
	value string `json:"value" bson:"value"`
}

type RateLimitDescriptorConfigType struct {
	Entries []RateLimitDescriptorEntryConfigType `json:"entries" bson:"entries"`
}
type HttpFilterTypedConfigType struct {
	Domain           *string                         `json:"domain" bson:"domain"`
	Descriptors      []RateLimitDescriptorConfigType `json:"descriptors" bson:"descriptors"`
	FailureModeDeny  *bool                           `json:"failureModeDeny" bson:"failureModeDeny"`
	VhRateLimits     *string                         `json:"vhRateLimits" bson:"vhRateLimits"`
	FailureModeAllow *bool                           `json:"failureModeAllow" bson:"failireModeAllow"` //authz, authzs3
}
type RateLimitRequestHeadersConfigType struct {
	HeaderName    string `json:"headerName" bson:"headerName"`
	DescriptorKey string `json:"descriptorKey" bson:"descriptorKey"`
	SkipIfAbsent  bool   `json:"skipIfAbsent" bson:"skipIfAbsent"`
}
type RateLimitGenericKeyConfigType struct {
	DescriptorValue string `json:"descriptorValue" bson:"descriptorValue"`
	DescriptorKey   string `json:"descriptorKey" bson:"descriptorKey"`
}
type RateLimitActionConfigType struct {
	RequestHeaders *RateLimitRequestHeadersConfigType `json:"requestHeaders" bson:"requestHeaders"`
	GenericKey     *RateLimitGenericKeyConfigType     `json:"genericKey" bson:"genericKey"`
}
type RateLimitConfigType struct {
	Actions []RateLimitActionConfigType `json:"actions" bson:"actions"`
}
type VirtualHostConfigType struct {
	Name       string                `json:"name" bson:"name"`
	Domains    []string              `json:"domains" bson:"domains" omitempty`
	Routes     []RouteConfigType     `json:"routes" bson:"routes"`
	RateLimits []RateLimitConfigType `json:"rateLimits" bson:"rateLimits" omitempty`
}
type RouteConfigConfigType struct {
	Name         string                  `json:"name" bson:"name"`
	VirtualHosts []VirtualHostConfigType `json:"virtualHosts" bson:"virtualHosts"`
}
type AccessLogTypedConfigType struct {
	AdditionalResponseHeadersToLog []string `json:"additionalResponseHeadersToLog" bson:"additionalResponseHeadersToLog" omitempty`
	AdditionalRequestHeadersToLog  []string `json:"additionalRequestHeadersToLog" bson:"additionalRequestHeadersToLog" omitempty`
}
type AccessLogConfigType struct {
	Name        string                   `json:"name" bson:"name"`
	TypedConfig AccessLogTypedConfigType `json:"typedConfig" bson:"typedConfig"`
}
type TokenBucketConfigType struct {
	MaxTokens     int    `json:"maxTokens" bson:"maxTokens"`
	TokensPerFill int    `json:"tokensPerFill" bson:"tokensPerFill"`
	FillInterval  string `json:"fillInterval" bson:"fillInterval"`
}
type HttpFiltersConfigType struct {
	Name        string                    `json:"name" bson:"name"`
	TypedConfig HttpFilterTypedConfigType `json:"typedConfig" bson:"typedConfig"`
}
type CommonHttpProtocolOptionsConfigType struct {
	MaxHeadersCount   *int    `json:"maxHeadersCount" bson:"maxHeadersCount"`
	MaxStreamDuration *string `json:"maxStreamDuration" bson:"maxStreamDuration"`
}

type UpgradeConfigConfigType struct {
	UpgradeType string `json:"upgradeType" bson:"upgradeType"`
}

type TcpHashPolicySourceIpConfigType struct {
}

type TcpHashPolicyConfigType struct {
	SourceIp *TcpHashPolicySourceIpConfigType `json:"sourceIp" bson:"sourceIp"`
}

type FilterTypedConfigType struct {
	StatPrefix                string                               `json:"statPrefix" bson:"statPrefix"`
	RouteConfig               *RouteConfigConfigType               `json:"routeConfig" bson:"routeConfig" omitempty`
	AccessLog                 []*AccessLogConfigType               `json:"accessLog" bson:"accessLog" omitempty`
	Cluster                   *string                              `json:"cluster" bson:"cluster" omitempty`
	TokenBucket               *TokenBucketConfigType               `json:"tokenBucket" bson:"tokenBucket" omitempty`
	IdleTimeout               *string                              `json:"idleTimeout" bson:"idleTimeout" omitempty`
	UseRemoteAddress          *bool                                `json:"useRemoteAddress" bson:"useRemoteAddress" omitempty`
	SkipXffAppend             *bool                                `json:"skipXffAppend" bson:"skipXffAppend" omitempty`
	HttpFilters               []HttpFiltersConfigType              `json:"httpFilters" bson:"httpFilters omitempty"`
	MaxRequestHeadersKb       *int                                 `json:"maxRequestHeadersKb" bson:"maxRequestHeadersKb"`
	CommonHttpProtocolOptions *CommonHttpProtocolOptionsConfigType `json:"commonHttpProtocolOptions" bson:"commonHttpProtocolOptions"`
	HttpProtocolOptions       *HttpProtocolOptionsConfigType       `json:"httpProtocolOptions" bson:"httpProtocolOptions"`
	UpgradeConfigs            []UpgradeConfigConfigType            `json:"upgradeConfigs" bson:"upgradeConfigs"`
	HashPolicy                []TcpHashPolicyConfigType            `json:"hashPolicy" bson:"hashPolicy"`
}
type FilterConfigType struct {
	Name        string                `json:"name" bson:"name"`
	TypedConfig FilterTypedConfigType `json:"typedConfig" bson:"typedConfig"`
}
type InlineStringConfigType struct {
	InlineString string `json:"inlineString" bson:"inlineString"`
}
type TlsCertificateConfigType struct {
	CertificateChain InlineStringConfigType  `json:"certificateChain" bson:"certificateChain"`
	PrivateKey       InlineStringConfigType  `json:"privateKey" bson:"privateKey"`
	Password         *InlineStringConfigType `json:"password" bson:"password" omitempty`
}
type TlsParamsConfigType struct {
	TlsMinimumProtocolVersion *string  `json:"tlsMinimumProtocolVersion" bson:"tlsMinimumProtocolVersion" omitempty`
	TlsMaximumProtocolVersion *string  `json:"tlsMaximumProtocolVersion" bson:"tlsMaximumProtocolVersion" omitempty`
	CipherSuites              []string `json:"cipherSuites" bson:"cipherSuites" omitempty`
}
type ValidationContextConfigType struct {
	TrustedCa *InlineStringConfigType `json:"trustedCa" bson:"trustedCa" omitempty`
	SdsConfig *SdsConfigType          `json:"validationContextSdsSecretConfig" bson:"validationContextSdsSecretConfig" omitempty`
}
type SdsConfigType struct {
	Name string `json:"name" bson:"name"`
}
type CommonTlsContextType struct {
	TlsCertificates          []TlsCertificateConfigType   `json:"tlsCertificates" bson:"tlsCertificates" omitempty`
	SdsConfig                []SdsConfigType              `json:"tlsCertificateSdsSecretConfigs" bson:"tlsCertificateSdsSecretConfigs" omitempty`
	RequestClientCertificate *bool                        `json:"requestClientCertificate" bson:"requestClientCertificate" omitempty`
	TlsParams                *TlsParamsConfigType         `json:"tlsParams" bson:"tlsParams" omitempty`
	ValidationContext        *ValidationContextConfigType `json:"validationContext" bson:"validationContext" omitempty`
}
type TransportSocketTypedConfigType struct {
	CommonTlsContext CommonTlsContextType `json:"commonTlsContext" bson:"commonTlsContext"`
	Sni              *string              `json:"sni" bson:"sni" omitempty`
}
type TransportSocketConfigType struct {
	TypedConfig TransportSocketTypedConfigType `json:"typedConfig" bson:"typedConfig"`
}
type FilterChainsConfigType struct {
	FilterChainMatch *FilterChainMatchConfigType `json:"filterChainMatch" bson:"filterChainMatch" omitempty`
	Filters          []FilterConfigType          `json:"filters" bson:"filters" omitempty`
	TransportSocket  *TransportSocketConfigType  `json:"transportSocket" bson:"transportSocket" omitempty`
}
type ListenerConfigType struct {
	NodeID                        string                   `json:"nodeId" bson:"nodeId" omitempty`
	Name                          string                   `json:"name" bson:"name"`
	Address                       AddressConfigType        `json:"address" bson:"address"`
	FilterChains                  []FilterChainsConfigType `json:"filterChains" bson:"filterChains"`
	PerConnectionBufferLimitBytes *int                     `json:"perConnectionBufferLimitBytes" bson:"perConnectionBufferLimitBytes" omitempty`
}

type SecretConfigType struct {
	NodeID            string                       `json:"nodeId" bson:"nodeId" omitempty`
	Name              string                       `json:"name" bson:"name"`
	TlsCertificate    *TlsCertificateConfigType    `json:"tlsCertificate" bson:"tlsCertificate" omitempty`
	ValidationContext *ValidationContextConfigType `json:"validationContext" bson:"validationContext" omitempty`
}

type TlsCertificateViewType struct {
	Subject   string `json:"subject"`
	Issuer    string `json:"issuer"`
	NotBefore string `json:"notBefore"`
	NotAfter  string `json:"notAfter"`
}

type SecretViewType struct {
	Name                string                  `json:"name"`
	TlsCertificate      *TlsCertificateViewType `json:"tlsCertificate" omitempty`
	ValidationContextCa *TlsCertificateViewType `json:"validationContextCa" omitempty`
}
