package log

import (
	"context"
	"io"
	"os"
	"sync"
	"time"

	"github.com/natefinch/lumberjack"
	"github.com/rs/zerolog"
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
	logger *zerolog.Logger
)

func InitLogger(filepath string) *zerolog.Logger {
	once.Do(func() {
		zerolog.DurationFieldUnit = time.Microsecond
		zerolog.ErrorFieldName = "error"
		zerolog.ErrorStackFieldName = "stack-trace"
		zerolog.LevelFieldName = "level"
		zerolog.MessageFieldName = "message"
		zerolog.TimestampFieldName = "timestamp"

		fileWriter := &lumberjack.Logger{
			Filename:   filepath,
			MaxSize:    10,
			MaxBackups: 10,
			MaxAge:     10,
			Compress:   true,
		}
		var logOutput io.Writer = os.Stdout
		if os.Getenv("env") == "dev" {
			logOutput = zerolog.ConsoleWriter{
				Out:          os.Stdout,
				TimeFormat:   time.RFC3339Nano,
				NoColor:      false,
				TimeLocation: time.UTC,
			}
		}
		output := zerolog.MultiLevelWriter(logOutput, fileWriter)

		log := zerolog.New(output).
			Level(zerolog.TraceLevel).
			With().
			Timestamp().
			Caller().
			Stack().
			Int("pid", os.Getpid()).
			Int("gid", os.Getgid()).
			Int("uid", os.Getuid()).
			Logger()

		logger = &log

		logger.Info().
			Str(KeyTag, "InitLogger").
			Str(KeyProcess, "InitLogger").
			Msg("finish initiating logging")
	})
	return logger
}
