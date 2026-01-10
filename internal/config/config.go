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
	API      APIConfig
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

type APIConfig struct {
	Host            string
	Port            int
	EnableCORS      bool
	CORSOrigins     []string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
	RateLimit       float64
	RateBurst       int
	Production      bool
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
	API: APIConfig{
		Host:            "localhost",
		Port:            8080,
		EnableCORS:      true,
		CORSOrigins:     []string{"*"},
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    30 * time.Second,
		ShutdownTimeout: 10 * time.Second,
		RateLimit:       100.0,
		RateBurst:       200,
		Production:      false,
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

	cfg.API.Host = v.GetString("api.host")
	cfg.API.Port = v.GetInt("api.port")
	cfg.API.EnableCORS = v.GetBool("api.enable_cors")
	cfg.API.CORSOrigins = v.GetStringSlice("api.cors_origins")
	cfg.API.ReadTimeout = v.GetDuration("api.read_timeout")
	cfg.API.WriteTimeout = v.GetDuration("api.write_timeout")
	cfg.API.ShutdownTimeout = v.GetDuration("api.shutdown_timeout")
	cfg.API.RateLimit = v.GetFloat64("api.rate_limit")
	cfg.API.RateBurst = v.GetInt("api.rate_burst")
	cfg.API.Production = v.GetBool("api.production")

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

	v.SetDefault("api.host", defaultConfig.API.Host)
	v.SetDefault("api.port", defaultConfig.API.Port)
	v.SetDefault("api.enable_cors", defaultConfig.API.EnableCORS)
	v.SetDefault("api.cors_origins", defaultConfig.API.CORSOrigins)
	v.SetDefault("api.read_timeout", defaultConfig.API.ReadTimeout)
	v.SetDefault("api.write_timeout", defaultConfig.API.WriteTimeout)
	v.SetDefault("api.shutdown_timeout", defaultConfig.API.ShutdownTimeout)
	v.SetDefault("api.rate_limit", defaultConfig.API.RateLimit)
	v.SetDefault("api.rate_burst", defaultConfig.API.RateBurst)
	v.SetDefault("api.production", defaultConfig.API.Production)
}

func userConfigDir() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return dir
	}
	return ""
}
