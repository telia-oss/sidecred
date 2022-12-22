package githubrotator

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/go-github/v45/github"
	"github.com/telia-oss/githubapp"
	"go.uber.org/zap"

	"github.com/telia-oss/sidecred/eventctx"
)

const (
	defaultRateLimitCutoff = 50
)

type Config struct {
	IntegrationIDs     []string
	PrivateKeys        []string
	Logger             *zap.Logger
	OptAppFactory      AppFactory
	OptRateLimitClient RateLimits
}

type app struct {
	app            App
	integrationID  string
	token          *githubapp.Token
	rateLimitError *github.RateLimitError
}

func (app app) hasZeroRateLimit() bool {
	return app.rateLimitError != nil && app.rateLimitError.Rate.Reset.Sub(time.Now()).Seconds() <= 0 //nolint:gosimple,gocritic
}

func (app app) hasValidToken() bool {
	return app.token != nil && !hasTokenExpired(app.token)
}

type Rotator struct {
	apps            []app
	logger          *zap.Logger
	rateLimitClient RateLimits
}

func (r *Rotator) CreateInstallationToken(ctx context.Context, owner string, repositories []string, permissions *githubapp.Permissions) (*githubapp.Token, error) {
	if r.apps[0].hasValidToken() {
		r.logger.Debug("retrieving rate limits for token",
			zap.String("token_expires_at", r.apps[0].token.ExpiresAt.String()),
			zap.String("app", r.apps[0].integrationID))

		rateLimits, _, err := r.rateLimitClient.GetTokenRateLimits(ctx, r.apps[0].token.GetToken())
		switch {
		case err != nil:
			r.logger.Debug("unexpected error when retrieving rate limits",
				zap.String("app", r.apps[0].integrationID),
				zap.Error(err))

			return nil, fmt.Errorf("unexpected error getting rate limits (app='%s'): %w", r.apps[0].integrationID, err)
		case rateLimits.Core.Remaining >= defaultRateLimitCutoff:
			r.logger.Debug("rate limits above cutoff",
				zap.Int("rate_limit_max", rateLimits.Core.Limit),
				zap.Int("rate_limit_remaining", rateLimits.Core.Remaining),
				zap.String("rate_limit_reset", rateLimits.Core.Reset.String()),
				zap.String("app", r.apps[0].integrationID))

		case rateLimits.Core.Remaining < defaultRateLimitCutoff:
			r.logger.Debug("rate limits below cutoff",
				zap.Int("rate_limit_max", rateLimits.Core.Limit),
				zap.Int("rate_limit_remaining", rateLimits.Core.Remaining),
				zap.String("rate_limit_reset", rateLimits.Core.Reset.String()),
				zap.String("app", r.apps[0].integrationID))

			r.rotate()
		}
	}

	return r.createInstallationToken(ctx, owner, repositories, permissions)
}

func hasTokenExpired(token *githubapp.Token) bool {
	return token.ExpiresAt.Sub(time.Now()).Seconds() <= 5 //nolint:gosimple,gocritic
}

func (r *Rotator) createInstallationToken(ctx context.Context, owner string, repositories []string, permissions *githubapp.Permissions) (*githubapp.Token, error) {
	r.logger.Debug("createInstallationToken called",
		zap.String("app", r.apps[0].integrationID),
		zap.Int("number_of_apps", len(r.apps)))

	for i := 0; i < len(r.apps); i++ {

		if r.apps[0].hasZeroRateLimit() {
			r.logger.Debug("app has zero limit",
				zap.String("app", r.apps[0].integrationID))

			r.rotate()
			continue
		}

		r.logger.Debug("create installation token",
			zap.String("app", r.apps[0].integrationID))

		eventctx.GetStats(ctx).IncGithubCalls()
		var rateLimitError *github.RateLimitError
		token, err := r.apps[0].app.CreateInstallationToken(owner, repositories, permissions)
		switch {
		case errors.As(err, &rateLimitError):
			r.logger.Debug("rate limit error",
				zap.Int("rate_limit_max", rateLimitError.Rate.Limit),
				zap.Int("rate_limit_remaining", rateLimitError.Rate.Remaining),
				zap.String("rate_limit_reset", rateLimitError.Rate.Reset.String()),
				zap.String("app", r.apps[0].integrationID))

			r.apps[0].token = nil
			r.apps[0].rateLimitError = rateLimitError
			r.rotate()
		case err != nil:
			r.logger.Warn("create installation token, unexpected error",
				zap.String("app", r.apps[0].integrationID),
				zap.Error(err))

			r.apps[0].token = nil
			r.apps[0].rateLimitError = nil
			r.rotate()
		default:
			r.logger.Debug("found token",
				zap.String("token_expires_at", token.ExpiresAt.String()),
				zap.String("app", r.apps[0].integrationID))

			r.apps[0].token = token
			r.apps[0].rateLimitError = nil
			return token, nil
		}
	}

	r.logger.Debug("unable to retrieve any token")
	return nil, fmt.Errorf("unable to retrieve token")
}

func (r *Rotator) rotate() {
	tmp := r.apps[0]
	for i := 0; i < (len(r.apps) - 1); i++ {
		r.apps[i] = r.apps[i+1]
	}
	r.apps[len(r.apps)-1] = tmp
}

func New(config *Config) *Rotator {
	if config.OptRateLimitClient == nil {
		config.OptRateLimitClient = defaultRateLimitClient{}
	}

	if config.OptAppFactory == nil {
		config.OptAppFactory = defaultAppFactory{}
	}

	r := Rotator{
		rateLimitClient: config.OptRateLimitClient,
		logger:          config.Logger,
		apps:            []app{},
	}

	for i := 0; i < len(config.IntegrationIDs); i++ {

		ghubapp, err := config.OptAppFactory.Create(config.IntegrationIDs[i], config.PrivateKeys[i])
		if err != nil {
			r.logger.Fatal("failed to create github app", zap.Error(err))
		}

		r.apps = append(r.apps, app{
			app:            ghubapp,
			integrationID:  config.IntegrationIDs[i],
			token:          nil,
			rateLimitError: nil,
		})
	}

	return &r
}
