package config

type Config struct {
	Listen       string `mapstructure:"listen"`
	LogLevel     string `mapstructure:"log"`
	RegistryFile string `mapstructure:"registry_file"`

	TLS         TLS         `mapstructure:"tls"`
	Hosts       Hosts       `mapstructure:"hosts"`
	Offline     Offline     `mapstructure:"offline"`
	AutoApprove AutoApprove `mapstructure:"auto_approve"`
	API         API         `mapstructure:"api"`
}

// API configures the versioned REST API (/api/v1). The ACME-style client
// endpoints are always on; AdminToken (when set) gates the /api/v1/admin/*
// management endpoints behind a bearer token.
type API struct {
	AdminToken string `mapstructure:"admin_token"`
}

// AutoApprove configures automatic approval of hosts that first contact the
// server from a trusted source address, instead of leaving them PENDING.
type AutoApprove struct {
	Enabled  bool     `mapstructure:"enabled"`
	Private  bool     `mapstructure:"private"`  // shorthand: trust RFC1918 / ULA / loopback / link-local
	Networks []string `mapstructure:"networks"` // extra trusted CIDRs, e.g. 10.0.0.0/8
}

type TLS struct {
	Mode  string   `mapstructure:"mode"`
	Cert  string   `mapstructure:"cert"`
	Key   string   `mapstructure:"key"`
	Names []string `mapstructure:"names"`
}

type Hosts struct {
	File  string   `mapstructure:"file"`
	IP    string   `mapstructure:"ip"`
	Names []string `mapstructure:"names"`
}

type Offline struct {
	PrivateKey string `mapstructure:"private_key"`
	PublicKey  string `mapstructure:"public_key"`
}
