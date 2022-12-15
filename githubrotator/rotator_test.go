package githubrotator_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-github/v45/github"
	. "github.com/onsi/gomega"
	"github.com/telia-oss/githubapp"
	"go.uber.org/zap"

	"github.com/telia-oss/sidecred/githubrotator"
	"github.com/telia-oss/sidecred/githubrotator/fakes"
)

var (
	tA        = "a"
	tB        = "b"
	tC        = "c"
	tomorrow  = time.Now().AddDate(0, 0, 1)
	timestamp = time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)
)

var (
	errorUnexpected = errors.New("unexpected")
	errorRateLimit  = &github.RateLimitError{Rate: github.Rate{Limit: 1, Remaining: 1, Reset: github.Timestamp{Time: timestamp}}}
	rateLimitsAbove = &github.RateLimits{Core: &github.Rate{Limit: 50, Remaining: 51}}
	rateLimitsBelow = &github.RateLimits{Core: &github.Rate{Limit: 50, Remaining: 49}}
	responseEmpty   = &github.Response{}
	tokenA          = &githubapp.Token{InstallationToken: &github.InstallationToken{Token: &tA, ExpiresAt: &tomorrow}}
	tokenB          = &githubapp.Token{InstallationToken: &github.InstallationToken{Token: &tB, ExpiresAt: &tomorrow}}
	tokenC          = &githubapp.Token{InstallationToken: &github.InstallationToken{Token: &tC, ExpiresAt: &tomorrow}}
)

func TestRotator_CreateInstallationToken(t *testing.T) {
	var (
		logger = testLogger()
		assert = NewGomegaWithT(t)
	)

	rateLimits := returnsRateLimits(map[int]fakeRateLimitsResults{
		0: {rateLimitsBelow, responseEmpty, nil},
		1: {nil, nil, errorUnexpected},
		2: {rateLimitsAbove, responseEmpty, nil},
	})

	appFactory := returnsAppFactory(map[int]fakeAppFactoryResults{
		0: returnsApp(map[int]fakeAppResults{
			0: {nil, errorRateLimit},
		}),
		1: returnsApp(map[int]fakeAppResults{
			0: {nil, errorUnexpected},
			1: {tokenB, nil},
			2: {tokenC, nil},
		}),
		2: returnsApp(map[int]fakeAppResults{
			0: {tokenA, nil},
		}),
	})

	rotator := githubrotator.New(&githubrotator.Config{
		IntegrationIDs:     []string{"App0", "App1", "App2"},
		PrivateKeys:        []string{"Key0", "Key1", "Key2"},
		Logger:             logger,
		OptRateLimitClient: rateLimits,
		OptAppFactory:      appFactory,
	})

	// App 0 called, returns RateLimitError, rotates, App 1 called, returns Error, rotates, App 2 called, returns tokenA
	token, err := rotator.CreateInstallationToken("telia-oss", []string{"sidecred"}, nil)
	fmt.Println("TOKEN", token)
	fmt.Println("ERROR", err)
	assert.Expect(token.GetToken()).To(Equal("a"))
	assert.Expect(err).To(BeNil())

	// tokenA GetTokenRateLimits called, returns low rate limit, rotate, App 0 by passed, App 1 called, returns tokenB
	token, err = rotator.CreateInstallationToken("telia-oss", []string{"sidecred"}, nil)
	assert.Expect(token.GetToken()).To(Equal("b"))
	assert.Expect(err).To(BeNil())

	// tokenB GetTokenRateLimits called, returns Error
	token, err = rotator.CreateInstallationToken("telia-oss", []string{"sidecred"}, nil)
	assert.Expect(token).To(BeNil())
	assert.Expect(err).To(HaveOccurred())

	// tokenB GetTokenRateLimits called, returns rate limit above cutoff, returns tokenC
	token, err = rotator.CreateInstallationToken("telia-oss", []string{"sidecred"}, nil)
	assert.Expect(token).To(Equal(tokenC))
	assert.Expect(err).To(BeNil())
}

type fakeAppResults struct {
	result1 *githubapp.Token
	result2 error
}

func returnsApp(res map[int]fakeAppResults) fakeAppFactoryResults {
	fakeApp := &fakes.FakeApp{}
	for i, r := range res {
		fakeApp.CreateInstallationTokenReturnsOnCall(i, r.result1, r.result2)
	}

	return fakeAppFactoryResults{fakeApp, nil}
}

type fakeAppFactoryResults struct {
	result1 githubrotator.App
	result2 error
}

func returnsAppFactory(res map[int]fakeAppFactoryResults) *fakes.FakeAppFactory {
	fake := &fakes.FakeAppFactory{}
	for i, r := range res {
		fake.CreateReturnsOnCall(i, r.result1, r.result2)
	}

	return fake
}

type fakeRateLimitsResults struct {
	result1 *github.RateLimits
	result2 *github.Response
	result3 error
}

func returnsRateLimits(res map[int]fakeRateLimitsResults) *fakes.FakeRateLimits {
	fake := &fakes.FakeRateLimits{}
	for i, r := range res {
		fake.GetTokenRateLimitsReturnsOnCall(i, r.result1, r.result2, r.result3)
	}

	return fake
}

func testLogger() *zap.Logger {
	loggerMgr, _ := zap.NewDevelopmentConfig().Build()
	return loggerMgr
}
