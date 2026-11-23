package checker

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bidmybl/api-health-monitor/internal/features/checks"
)

const (
	defaultTimeout       = 10 * time.Second
	maxResponseBodyBytes = 64 * 1024
)

type Checker struct {
	client *http.Client
}

func NewChecker(timeout time.Duration) *Checker {
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	return &Checker{
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (c *Checker) Check(ctx context.Context, rawURL string) checks.Result {
	startedAt := time.Now()

	result := checks.Result{
		StartedAt: startedAt,
	}

	if err := validateURL(rawURL); err != nil {
		return c.finishWithError(result, startedAt, err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		rawURL,
		nil,
	)
	if err != nil {
		return c.finishWithError(result, startedAt, err)
	}

	response, err := c.client.Do(req)
	if err != nil {
		return c.finishWithError(result, startedAt, err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	statusCode := response.StatusCode
	result.HTTPStatusCode = &statusCode

	result = c.finish(result, startedAt)

	_, _ = io.CopyN(
		io.Discard,
		response.Body,
		maxResponseBodyBytes,
	)

	return result
}

func validateURL(rawURL string) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse URL: %w", err)
	}

	if !strings.EqualFold(parsedURL.Scheme, "http") &&
		!strings.EqualFold(parsedURL.Scheme, "https") {
		return fmt.Errorf("unsupported URL scheme")
	}

	if parsedURL.Hostname() == "" {
		return fmt.Errorf("URL host is missing")
	}

	if parsedURL.User != nil {
		return fmt.Errorf("URL must not contain credentials")
	}

	return nil
}

func (c *Checker) finishWithError(
	result checks.Result,
	startedAt time.Time,
	err error,
) checks.Result {
	message := err.Error()
	result.ErrorMessage = &message

	return c.finish(result, startedAt)
}

func (c *Checker) finish(
	result checks.Result,
	startedAt time.Time,
) checks.Result {
	elapsed := time.Since(startedAt)

	result.CompletedAt = startedAt.Add(elapsed)
	result.ResponseTimeMS = int(elapsed.Milliseconds())

	return result
}
