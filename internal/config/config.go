// Package config provides application configuration via Viper.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Database DatabaseConfig
	Scraper  ScraperConfig
	Log      LogConfig
}

type DatabaseConfig struct {
	Path string
}

type ScraperConfig struct {
	RateLimit       float64
	MaxDepth        int
	RequestTimeout  time.Duration
	MaxConcurrent   int
	UserAgent       string
	WikipediaAPIURL string
}

type LogConfig struct {
	Level string
}

var defaultConfig = Config{
	Database: DatabaseConfig{
		Path: "wikigraph.db",
	},
	Scraper: ScraperConfig{
		RateLimit:       100.0,
		MaxDepth:        3,
		RequestTimeout:  30 * time.Second,
		MaxConcurrent:   30,
		UserAgent:       "WikiGraph/1.0 (https://github.com/Thinh-nguyen-03/wikigraph)",
		WikipediaAPIURL: "https://en.wikipedia.org/api/rest_v1",
	},
	Log: LogConfig{
		Level: "info",
	},
}

// Reads configuration from file and environment variables.
// Locations: ./config.yaml, ~/.config/wikigraph/config.yaml
// Env vars prefixed with WIKIGRAPH_ (e.g., WIKIGRAPH_DATABASE_PATH).
func Load() (*Config, error) {
	v := viper.New()

	setDefaults(v)

	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath(filepath.Join(userConfigDir(), "wikigraph"))

	v.SetEnvPrefix("WIKIGRAPH")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("reading config: %w", err)
		}
	}

	cfg := &Config{}
	cfg.Database.Path = v.GetString("database.path")
	cfg.Scraper.RateLimit = v.GetFloat64("scraper.rate_limit")
	cfg.Scraper.MaxDepth = v.GetInt("scraper.max_depth")
	cfg.Scraper.RequestTimeout = v.GetDuration("scraper.request_timeout")
	cfg.Scraper.MaxConcurrent = v.GetInt("scraper.max_concurrent")
	cfg.Scraper.UserAgent = v.GetString("scraper.user_agent")
	cfg.Scraper.WikipediaAPIURL = v.GetString("scraper.wikipedia_api_url")
	cfg.Log.Level = v.GetString("log.level")

	return cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("database.path", defaultConfig.Database.Path)
	v.SetDefault("scraper.rate_limit", defaultConfig.Scraper.RateLimit)
	v.SetDefault("scraper.max_depth", defaultConfig.Scraper.MaxDepth)
	v.SetDefault("scraper.request_timeout", defaultConfig.Scraper.RequestTimeout)
	v.SetDefault("scraper.max_concurrent", defaultConfig.Scraper.MaxConcurrent)
	v.SetDefault("scraper.user_agent", defaultConfig.Scraper.UserAgent)
	v.SetDefault("scraper.wikipedia_api_url", defaultConfig.Scraper.WikipediaAPIURL)
	v.SetDefault("log.level", defaultConfig.Log.Level)
}

func userConfigDir() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return dir
	}
	return ""
}
