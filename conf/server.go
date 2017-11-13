package conf

// Server is the structure that holds the server configuration
type Server struct {
	Host   string `yaml:"host" env:"SERVER_HOST" default:"127.0.0.1"`
	Port   int    `yaml:"port" env:"SERVER_PORT" default:"8080"`
	Debug  bool   `yaml:"debug" env:"SERVER_DEBUG"`
	Cors   Cors   `yaml:"cors"`
	Prefix Prefix `yaml:"prefix"`
}

// Cors is a simple structure holding the information about CORS configuration
type Cors struct {
	AllowOrigins   []string `yaml:"allow_origins"`
	Enabled        bool     `yaml:"enabled" env:"CORS_ENABLED"`
	PermissiveMode bool     `yaml:"permissive_mode" env:"CORS_PERMISSIVE"`
}

//Prefix is a simple structure holding the information about prefix apis
type Prefix struct {
	Api   string `yaml:"api"`
	Admin string `yaml:"admin"`
}
