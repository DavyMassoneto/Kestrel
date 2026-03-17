package cfg

import (
	"github.com/caarlos0/env/v11"
)

type Config struct {
	Port      int    `env:"PORT"      envDefault:"8080"`
	LogLevel  string `env:"LOG_LEVEL" envDefault:"info"`
	LogFormat string `env:"LOG_FORMAT" envDefault:"json"`
}

func Load() (Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
