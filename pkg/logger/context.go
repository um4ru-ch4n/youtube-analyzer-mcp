package logger

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"go.uber.org/zap"
)

type contextKey int

const (
	loggerContextKey contextKey = iota
)

func ToContext(ctx context.Context, l *zap.SugaredLogger) context.Context {
	return context.WithValue(ctx, loggerContextKey, l)
}

func FromContext(ctx context.Context) *zap.SugaredLogger {
	if l, ok := ctx.Value(loggerContextKey).(*zap.SugaredLogger); ok {
		return l
	}
	return global
}

func WithName(ctx context.Context, name string) context.Context {
	return ToContext(ctx, FromContext(ctx).Named(name))
}

func WithKV(ctx context.Context, key string, value interface{}) context.Context {
	return ToContext(ctx, FromContext(ctx).With(key, value))
}

func WithFields(ctx context.Context, fields ...zap.Field) context.Context {
	return ToContext(ctx, FromContext(ctx).Desugar().With(fields...).Sugar())
}

func WithCorrelationID(ctx context.Context, id string) context.Context {
	return WithKV(ctx, "correlation_id", id)
}

func WithTaskID(ctx context.Context, taskID string) context.Context {
	return WithKV(ctx, "task_id", taskID)
}

func GenerateCorrelationID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Convenience functions that use context logger.
func Debug(ctx context.Context, args ...interface{})                   { FromContext(ctx).Debug(args...) }
func Debugf(ctx context.Context, format string, args ...interface{})   { FromContext(ctx).Debugf(format, args...) }
func DebugKV(ctx context.Context, message string, kvs ...interface{})  { FromContext(ctx).Debugw(message, kvs...) }
func Info(ctx context.Context, args ...interface{})                    { FromContext(ctx).Info(args...) }
func Infof(ctx context.Context, format string, args ...interface{})    { FromContext(ctx).Infof(format, args...) }
func InfoKV(ctx context.Context, message string, kvs ...interface{})   { FromContext(ctx).Infow(message, kvs...) }
func Warn(ctx context.Context, args ...interface{})                    { FromContext(ctx).Warn(args...) }
func Warnf(ctx context.Context, format string, args ...interface{})    { FromContext(ctx).Warnf(format, args...) }
func WarnKV(ctx context.Context, message string, kvs ...interface{})   { FromContext(ctx).Warnw(message, kvs...) }
func Error(ctx context.Context, args ...interface{})                   { FromContext(ctx).Error(args...) }
func Errorf(ctx context.Context, format string, args ...interface{})   { FromContext(ctx).Errorf(format, args...) }
func ErrorKV(ctx context.Context, message string, kvs ...interface{})  { FromContext(ctx).Errorw(message, kvs...) }
func Fatal(ctx context.Context, args ...interface{})                   { FromContext(ctx).Fatal(args...) }
func Fatalf(ctx context.Context, format string, args ...interface{})   { FromContext(ctx).Fatalf(format, args...) }
func FatalKV(ctx context.Context, message string, kvs ...interface{})  { FromContext(ctx).Fatalw(message, kvs...) }
