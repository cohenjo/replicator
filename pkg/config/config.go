package config

import (
	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

type WaterFlowsConfig struct {
	Type       string
	Host       string
	Port       int
	Schema     string
	Collection string
}

type TransformOperation struct {
	Operation string                 `json:"operation"`
	Spec      map[string]interface{} `json:"spec"`
}

//Configuration defines the base configuration that can be passed to the WASTE system
type Configuration struct {
	Debug              bool
	Execute            bool
	StreamQueueLength  int
	EstuaryQueueLength int
	DBUser             string
	DBPasswd           string
	MyDBUser           string
	MyDBPasswd         string
	Streams            []WaterFlowsConfig
	Estuaries          []WaterFlowsConfig
	Transforms         []TransformOperation
}

// Config is the global configuration variable
var Config *Configuration

// LoadConfiguration loads configuration using viper
func LoadConfiguration() *Configuration {

	viper.SetDefault("Debug", true)
	viper.SetDefault("Execute", false)
	viper.SetDefault("StreamQueueLength", 10000)
	viper.SetDefault("EstuaryQueueLength", 10000)

	viper.SetConfigName("replicator.conf")   // name of config file (without extension)
	viper.AddConfigPath("/etc/replicator/")  // path to look for the config file in
	viper.AddConfigPath("$HOME/.replicator") // call multiple times to add many search paths
	viper.AddConfigPath("./conf")            // optionally look for config in the working directory
	err := viper.ReadInConfig()              // Find and read the config file
	if err != nil {                          // Handle errors reading the config file
		log.Error().Err(err).Msg("Fatal error config file")
	}

	viper.WatchConfig()
	viper.OnConfigChange(reloadConfig)
	var cfg Configuration
	err = viper.Unmarshal(&cfg)
	if err != nil {
		log.Error().Err(err).Msg("unable to decode into struct")
	}

	log.Debug().Msgf("configuration loaded: %+v", cfg)

	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if cfg.Debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
	Config = &cfg
	return &cfg
}

func reloadConfig(e fsnotify.Event) {
	log.Info().Msgf("Config file changed: %v", e.Name)
	var cfg Configuration
	err := viper.Unmarshal(&cfg)
	if err != nil {
		log.Error().Err(err).Msg("unable to decode into struct")
	}
	Config = &cfg
}
