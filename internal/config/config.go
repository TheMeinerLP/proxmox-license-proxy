package config

type Config struct {
	Listen       string `mapstructure:"listen"`
	LogLevel     string `mapstructure:"log"`
	RegistryFile string `mapstructure:"registry_file"`

	TLS     TLS     `mapstructure:"tls"`
	Hosts   Hosts   `mapstructure:"hosts"`
	Offline Offline `mapstructure:"offline"`
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
