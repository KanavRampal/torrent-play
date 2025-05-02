package config

import (
	"flag"
)

// AppConfig holds the application configuration.
type AppConfig struct {
	ListenAddr string
	DataDir    string
}

// LoadConfig parses command-line flags and returns the configuration.
func LoadConfig() *AppConfig {
	cfg := &AppConfig{}
	flag.StringVar(&cfg.ListenAddr, "addr", "localhost:8080", "HTTP listen address")
	flag.StringVar(&cfg.DataDir, "data-dir", "./data", "Directory for torrent client data")
	flag.Parse()
	return cfg
}