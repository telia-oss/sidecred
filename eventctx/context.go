package eventctx

import (
	"context"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

var (
	loggerKey = struct{}{}
	statsKey  = struct{}{}
)

type Stats struct {
	CallsToGithub int
}

func (s *Stats) IncGithubCalls() {
	s.CallsToGithub++
}

func GetLogger(ctx context.Context) *zap.Logger {
	logger, ok := ctx.Value(loggerKey).(*zap.Logger)
	if !ok {
		return zap.NewNop()
	}

	return logger
}

func SetLogger(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

func TestContext(t *testing.T) context.Context {
	return SetLogger(context.TODO(), zaptest.NewLogger(t))
}

func GetStats(ctx context.Context) *Stats {
	stats, ok := ctx.Value(statsKey).(*Stats)
	if !ok {
		return &Stats{}
	}

	return stats
}

func SetStats(ctx context.Context, stats *Stats) context.Context {
	return context.WithValue(ctx, statsKey, stats)
}
