package authz

type AuthzConfigType struct {
	Config AuthzConfig `bson:"config" json:"config"`
}

type AuthzConfig struct {
	Domains map[string]DomainConfig `bson:"domains" json:"domains"`
}

type DomainConfig struct {
	Action  bool                    `bson:"action" json:"action"`
	Sources map[string]SourceConfig `bson:"sources" json:"sources"`
}

type SourceConfig struct {
	Action  bool           `bson:"action" json:"action"`
	Headers *HeadersConfig `bson:"headers" json:"headers"`
}

type HeadersConfig struct {
	Action  bool           `bson:"action" json:"action"`
	Headers []HeaderConfig `bson:"headers" json:"headers"`
}

type HeaderConfig struct {
	Name  string `bson:"name" json:"name"`
	Value string `bson:"value" json:"value"`
}
