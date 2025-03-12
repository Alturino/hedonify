package log

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/natefinch/lumberjack"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"

	"github.com/Alturino/ecommerce/internal/config"
	"github.com/Alturino/ecommerce/internal/constants"
)

type requestId struct{}

func RequestIDFromContext(c context.Context) string {
	return c.Value(requestId{}).(string)
}

func AttachRequestIDToContext(c context.Context, h string) context.Context {
	return context.WithValue(c, requestId{}, h)
}

var (
	once   sync.Once
	logger zerolog.Logger
)

func Get(filepath string, config config.Application) zerolog.Logger {
	once.Do(func() {
		zerolog.DurationFieldUnit = time.Microsecond
		zerolog.ErrorFieldName = "error"
		zerolog.ErrorStackFieldName = "stack-trace"
		zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
		zerolog.LevelFieldName = "level"
		zerolog.MessageFieldName = "message"
		zerolog.TimestampFieldName = "timestamp"

		logLevel := zerolog.InfoLevel
		if config.Env == "development" {
			logLevel = zerolog.TraceLevel
		}

		fileWriter := &lumberjack.Logger{
			Filename: filepath,
			Compress: true,
		}
		output := zerolog.MultiLevelWriter(os.Stdout, fileWriter)

		logger = zerolog.New(output).
			Level(logLevel).
			With().
			Timestamp().
			Caller().
			Stack().
			Int("pid", os.Getpid()).
			Int("gid", os.Getgid()).
			Int("uid", os.Getuid()).
			Logger()

		logger.Info().
			Str(constants.KEY_TAG, "InitLogger").
			Str(constants.KEY_PROCESS, "InitLogger").
			Msg("finish initiating logging")
	})
	return logger
}
