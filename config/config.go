package config

import (
	"flag"
	"log"
	"strings"

	"github.com/spf13/viper"
)

// AppConfig holds the application configuration.
type AppConfig struct {
	ListenAddr string
	DataDir    string
	ImdbAPIKey string
}

// LoadConfig parses command-line flags and returns the configuration.
func LoadConfig() *AppConfig {
	cfg := &AppConfig{}
	flag.StringVar(&cfg.ListenAddr, "addr", "localhost:8080", "HTTP listen address")
	flag.StringVar(&cfg.DataDir, "data-dir", "./data", "Directory for torrent client data")
	// ImdbAPIKey will be loaded via Viper from env or .env file
	flag.Parse()

	// Initialize Viper
	viper.SetConfigName(".env")                            // Name of config file (without extension)
	viper.SetConfigType("env")                             // REQUIRED if the config file does not have the extension in the name
	viper.AddConfigPath(".")                               // Look for config in the working directory
	viper.AutomaticEnv()                                   // Read in environment variables that match
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_")) // Example: SOME.VAR becomes SOME_VAR

	// Attempt to read the .env file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// .env file not found; ignore error, rely on environment variables
			log.Println("INFO: .env file not found. Will rely on environment variables for configuration.")
		} else {
			// .env file was found but another error was produced
			log.Printf("WARN: Error reading .env file: %s. Proceeding with environment variables and defaults.", err)
		}
	}

	// Get the IMDB API Key from Viper (env var: OMDB_API_KEY or from .env file)
	cfg.ImdbAPIKey = viper.GetString("OMDB_API_KEY") // Viper keys are case-insensitive by default

	return cfg
}
