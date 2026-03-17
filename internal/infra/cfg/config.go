package cfg

import (
	"github.com/caarlos0/env/v11"
)

type Config struct {
	Port          int    `env:"PORT"           envDefault:"8080"`
	LogLevel      string `env:"LOG_LEVEL"      envDefault:"info"`
	LogFormat     string `env:"LOG_FORMAT"     envDefault:"json"`
	EncryptionKey string `env:"ENCRYPTION_KEY,required"`
	AdminKey      string `env:"ADMIN_KEY,required"`
	ClaudeAPIKey  string `env:"CLAUDE_API_KEY,required"`
	ClaudeBaseURL string `env:"CLAUDE_BASE_URL" envDefault:"https://api.anthropic.com"`
	DBPath           string `env:"DB_PATH"             envDefault:"kestrel.db"`
	OAuthClientID    string `env:"OAUTH_CLIENT_ID"     envDefault:""`
	OAuthRedirectURI string `env:"OAUTH_REDIRECT_URI"  envDefault:"http://localhost:8080/api/oauth/callback"`
	OAuthAuthURL     string `env:"OAUTH_AUTH_URL"       envDefault:"https://console.anthropic.com/oauth/authorize"`
	OAuthTokenURL    string `env:"OAUTH_TOKEN_URL"      envDefault:"https://console.anthropic.com/oauth/token"`
	OAuthScope       string `env:"OAUTH_SCOPE"          envDefault:"org:create_api_key user:profile user:inference user:sessions:claude_code user:mcp_servers"`
}

func Load() (Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
