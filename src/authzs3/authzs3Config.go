package authzs3

type Authzs3ConfigType struct {
	Config Authzs3Config `bson:"config" json:"config"`
}

type Authzs3Config struct {
	Namespaces map[string]NamespaceConfig `bson:"namespaces" json:"namespaces"`
}

type NamespaceConfig struct {
	Action  bool                    `bson:"action" json:"action"`
	Sources map[string]SourceConfig `bson:"sources" json:"sources"`
}

type SourceConfig struct {
	Action bool `bson:"action" json:"action"`
}
