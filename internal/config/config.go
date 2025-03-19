package config

import (
	"context"
	"fmt"
	"sync"

	"github.com/fsnotify/fsnotify"
	zl "github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	"github.com/Alturino/ecommerce/internal/constants"
)

type Application struct {
	Env       string `mapstructure:"env"        json:"env"`
	Host      string `mapstructure:"host"       json:"host"`
	SecretKey string `mapstructure:"secret_key" json:"secret_key"`
	Port      int    `mapstructure:"port"       json:"port"`
}

type Database struct {
	Name           string `mapstructure:"name"            json:"name"`
	Host           string `mapstructure:"host"            json:"host"`
	MigrationPath  string `mapstructure:"migration_path"  json:"migration_path"`
	Password       string `mapstructure:"password"        json:"password"`
	TimeZone       string `mapstructure:"timezone"        json:"timezone"`
	Username       string `mapstructure:"username"        json:"username"`
	MaxConnections int    `mapstructure:"max_connections" json:"max_connections"`
	MinConnections int    `mapstructure:"min_connections" json:"min_connections"`
	Port           uint16 `mapstructure:"port"            json:"port"`
}

type Cache struct {
	Host     string `mapstructure:"host"     json:"host"`
	Password string `mapstructure:"password" json:"password"`
	Database int    `mapstructure:"database" json:"database"`
	Port     uint16 `mapstructure:"port"     json:"port"`
}

type Otel struct {
	Host string `mapstructure:"host" json:"host"`
	Port int    `mapstructure:"port" json:"port"`
}

type Config struct {
	Database    `mapstructure:"db"          json:"db"`
	Cache       `mapstructure:"cache"       json:"cache"`
	Application `mapstructure:"application" json:"application"`
	Otel        `mapstructure:"otel"        json:"otel"`
}

var (
	once   sync.Once
	config Config
)

func Get(c context.Context, filename string) Config {
	logger := zl.Logger.
		With().
		Str(constants.KEY_TAG, "main InitConfig").
		Str(constants.KEY_PROCESS, "init config").
		Str("filename", filename).
		Logger()

	viper.SetConfigName(filename)
	viper.AddConfigPath("./env/")
	viper.SetConfigType("yaml")
	viper.AutomaticEnv()
	viper.WatchConfig()
	viper.OnConfigChange(func(in fsnotify.Event) {
		Get(c, filename)
	})

	logger = logger.With().Str(constants.KEY_PROCESS, "reading config").Logger()
	logger.Trace().Msg("reading config")
	err := viper.ReadInConfig()
	if err != nil {
		err = fmt.Errorf("error when reading config with error=%w", err)
		logger.Fatal().Err(err).Msg(err.Error())
	}
	logger.Info().Msg("read config")

	logger = logger.With().Str(constants.KEY_PROCESS, "unmarshaling config").Logger()
	logger.Trace().Msg("unmarshaling config")
	err = viper.Unmarshal(&config)
	if err != nil {
		err = fmt.Errorf("error unmarshaling config with error=%w", err)
		logger.Fatal().Err(err).Msg(err.Error())
	}
	logger.Info().Any(constants.KEY_CONFIG, config).Msg("marshalled config")
	return config
}
