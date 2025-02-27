package controllers

import (
	"log/slog"
	"os"

	slogfiber "github.com/samber/slog-fiber"
	slogmulti "github.com/samber/slog-multi"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/log"
)

func InitSlog(logProvider log.LoggerProvider) (*slog.Logger, slogfiber.Config) {
	// Create OpenTelemetry bridge handler
	otelHandler := otelslog.NewHandler("zeroshare-backend",
		otelslog.WithSource(true),
		otelslog.WithLoggerProvider(logProvider),
	)

	handler := slogmulti.Fanout(
		otelHandler,
		slog.NewTextHandler(os.Stdout, nil), // Keep console output for development
	)

	config := slogfiber.Config{
		WithRequestBody:    true,
		WithResponseBody:   false,
		WithRequestHeader:  false,
		WithResponseHeader: false,
		Filters: []slogfiber.Filter{
			slogfiber.IgnoreStatus(401, 404),
			slogfiber.IgnorePathContains("/oauth/google", "/auth/google/callback", "/refresh"),
		},
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logger, config
}
