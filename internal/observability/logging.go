package observability

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// 1.- LoggerConfig captures configuration inputs required to build a Zap logger.
type LoggerConfig struct {
	Environment      string
	Level            string
	Service          string
	OutputPaths      []string
	ErrorOutputPaths []string
}

// 1.- requestIDKey is a private struct type used as the context key for request IDs.
type requestIDKey struct{}

// 1.- ginRequestIDKey mirrors the key used by the Gin middleware for backward compatibility.
const ginRequestIDKey = "request_id"

// 1.- NewLogger constructs a Zap logger configured for the supplied environment.
func NewLogger(cfg LoggerConfig) (*zap.Logger, error) {
	// 1.- Select the appropriate base configuration depending on the runtime environment.
	zapCfg := selectZapConfig(strings.TrimSpace(strings.ToLower(cfg.Environment)))

	// 2.- Apply custom log level configuration when provided.
	if strings.TrimSpace(cfg.Level) != "" {
		if err := zapCfg.Level.UnmarshalText([]byte(cfg.Level)); err != nil {
			return nil, fmt.Errorf("invalid log level %q: %w", cfg.Level, err)
		}
	}

	// 3.- Ensure output paths are well defined so logs do not get lost silently.
	if len(cfg.OutputPaths) > 0 {
		zapCfg.OutputPaths = cfg.OutputPaths
	}
	if len(cfg.ErrorOutputPaths) > 0 {
		zapCfg.ErrorOutputPaths = cfg.ErrorOutputPaths
	}

	// 4.- Enrich every log entry with the service label for correlation across components.
	service := strings.TrimSpace(cfg.Service)
	if service == "" {
		service = "Larago API"
	}
	if zapCfg.InitialFields == nil {
		zapCfg.InitialFields = map[string]interface{}{}
	}
	zapCfg.InitialFields["service"] = service

	// 5.- Normalize timestamp rendering so production logs remain machine readable.
	zapCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// 6.- Build the logger instance and surface configuration errors to callers.
	logger, err := zapCfg.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build zap logger: %w", err)
	}

	return logger, nil
}

// 1.- selectZapConfig returns a Zap configuration tuned for the chosen environment.
func selectZapConfig(environment string) zap.Config {
	// 1.- Prefer the development encoder when the environment indicates debug operation.
	switch environment {
	case "development", "dev", "debug":
		cfg := zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		return cfg
	default:
		// 2.- Fall back to production defaults when no explicit environment has been set.
		cfg := zap.NewProductionConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder
		return cfg
	}
}

// 1.- ContextWithRequestID associates a sanitized request ID with the provided context.
func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	// 1.- Treat nil contexts as background to avoid panics when callers forget to supply one.
	if ctx == nil {
		ctx = context.Background()
	}

	// 2.- Ignore empty request identifiers to prevent polluting contexts with useless values.
	trimmed := strings.TrimSpace(requestID)
	if trimmed == "" {
		return ctx
	}

	// 3.- Store the request ID under the private key while keeping backward compatibility keys.
	ctx = context.WithValue(ctx, requestIDKey{}, trimmed)
	ctx = context.WithValue(ctx, ginRequestIDKey, trimmed)
	return ctx
}

// 1.- RequestIDFromContext extracts the request identifier when present.
func RequestIDFromContext(ctx context.Context) (string, bool) {
	// 1.- Bail out early when no context is available.
	if ctx == nil {
		return "", false
	}

	// 2.- Look for the private key first to avoid clashing with other packages.
	if value := ctx.Value(requestIDKey{}); value != nil {
		if requestID, ok := value.(string); ok && strings.TrimSpace(requestID) != "" {
			return requestID, true
		}
	}

	// 3.- Inspect the legacy Gin key to support middleware that has not been migrated yet.
	if value := ctx.Value(ginRequestIDKey); value != nil {
		if requestID, ok := value.(string); ok && strings.TrimSpace(requestID) != "" {
			return requestID, true
		}
	}

	return "", false
}

// 1.- WithContext annotates the supplied logger with context derived fields such as request ID.
func WithContext(ctx context.Context, logger *zap.Logger) *zap.Logger {
	// 1.- Guard against nil loggers to keep call sites resilient.
	if logger == nil {
		return logger
	}

	// 2.- Inject the request ID field when one is available.
	if requestID, ok := RequestIDFromContext(ctx); ok {
		return logger.With(zap.String("request_id", requestID))
	}

	return logger
}
