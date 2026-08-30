package logger

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type ctxKey struct{}

func Setup(env, version string, debug bool) zerolog.Logger {
	zerolog.TimeFieldFormat = time.RFC3339Nano
	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	var out io.Writer = os.Stdout
	if env == "development" {
		out = zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "15:04:05"}
	}

	l := zerolog.New(out).With().
		Timestamp().
		Str("service", "happyfeet-api").
		Str("version", version).
		Logger()

	log.Logger = l
	return l
}

func ToContext(ctx context.Context, l *zerolog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

func FromContext(ctx context.Context) *zerolog.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*zerolog.Logger); ok {
		return l
	}
	l := log.Logger
	return &l
}
