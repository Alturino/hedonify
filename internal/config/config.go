package config

import (
	"context"
	"sync"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"

	"github.com/Alturino/ecommerce/internal/log"
)

type Application struct {
	Host      string `mapstructure:"host"`
	SecretKey string `mapstructure:"secretkey"`
	Port      int    `mapstructure:"port"`
}

type Database struct {
	DbName         string `mapstructure:"name"`
	Host           string `mapstructure:"host"`
	MigrationPath  string `mapstructure:"migration_path"`
	Password       string `mapstructure:"password"`
	TimeZone       string `mapstructure:"timezone"`
	Username       string `mapstructure:"username"`
	Port           uint16 `mapstructure:"port"`
	MaxConnections byte   `mapstructure:"max_connections"`
	MinConnections byte   `mapstructure:"min_connections"`
}

type Config struct {
	Env         string `mapstructure:"env"`
	Database    `mapstructure:"db"`
	Application `mapstructure:"application"`
}

var (
	once   sync.Once
	config *Config
)

func InitConfig(c context.Context, filename string) *Config {
	cfg := Config{}
	once.Do(func() {
		logger := zerolog.Ctx(c).
			With().
			Str(log.KeyTag, "main InitConfig").
			Logger()

		logger.Info().
			Str(log.KeyProcess, "InitConfig").
			Msg("starting InitConfig")

		viper.SetConfigName(filename)
		viper.AddConfigPath("./env")
		viper.SetConfigType("yaml")
		viper.AutomaticEnv()

		err := viper.ReadInConfig()
		if err != nil {
			logger.Fatal().
				Err(err).
				Str("filename", filename).
				Str(log.KeyProcess, "InitConfig").
				Msgf("error when reading config with error=%s", err.Error())
		}

		err = viper.Unmarshal(&cfg)
		if err != nil {
			logger.Fatal().
				Err(err).
				Str(log.KeyProcess, "InitConfig").
				Msgf("error unmarshaling config with error=%s", err.Error())
		}
		config = &cfg
		logger.Info().
			Str(log.KeyProcess, "InitConfig").
			Any(log.KeyConfig, config).
			Msg("marshalled config")
	})
	return config
}
