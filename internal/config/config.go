package config

import (
	"context"
	"fmt"
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
			Str(log.KeyProcess, "init config").
			Str("filename", filename).
			Logger()

		viper.SetConfigName(filename)
		viper.AddConfigPath("./env")
		viper.SetConfigType("yaml")
		viper.AutomaticEnv()

		logger = logger.With().Str(log.KeyProcess, "reading config").Logger()
		logger.Info().Msg("reading config")
		err := viper.ReadInConfig()
		if err != nil {
			err = fmt.Errorf("error when reading config with error=%w", err)
			logger.Fatal().Err(err).Msg(err.Error())
		}
		logger.Info().Msg("read config")

		logger = logger.With().Str(log.KeyProcess, "unmarshaling config").Logger()
		logger.Info().Msg("unmarshaling config")
		err = viper.Unmarshal(&cfg)
		if err != nil {
			err = fmt.Errorf("error unmarshaling config with error=%w", err)
			logger.Fatal().Err(err).Msg(err.Error())
		}
		config = &cfg
		logger = logger.With().Any(log.KeyConfig, cfg).Logger()
		logger.Info().Msg("marshalled config")
	})
	return config
}
