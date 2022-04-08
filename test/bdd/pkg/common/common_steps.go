/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package common

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"text/template"

	"github.com/cucumber/godog"
	"github.com/tidwall/gjson"

	"github.com/trustbloc/ace/test/bdd/pkg/internal/httputil"
)

const (
	healthCheckURL = "https://%s:%d/healthcheck"
)

// Steps defines context for common scenario steps.
type Steps struct {
	HTTPClient         *http.Client
	responseStatus     string
	responseStatusCode int
	responseBody       []byte
}

// NewSteps returns new Steps context.
func NewSteps(tlsConfig *tls.Config) *Steps {
	return &Steps{
		HTTPClient: &http.Client{Transport: &http.Transport{TLSClientConfig: tlsConfig}},
	}
}

// RegisterSteps registers common scenario steps.
func (s *Steps) RegisterSteps(sc *godog.ScenarioContext) {
	sc.Step(`^an HTTP GET is sent to "([^"]*)"$`, s.httpGet)
	sc.Step(`^an HTTP POST is sent to "([^"]*)"$`, s.httpPost)
	sc.Step(`^an HTTP PUT is sent to "([^"]*)"$`, s.httpPut)
	sc.Step(`^response status is "([^"]*)"$`, s.checkResponseStatus)
	sc.Step(`^response contains "([^"]*)" with value "([^"]*)"$`, s.checkResponseValue)
	sc.Step(`^response contains non-empty "([^"]*)"$`, s.checkNonEmptyResponseValue)
}

type healthCheckResponse struct {
	Status string `json:"status"`
}

// HealthCheck checks if service on host:port is up and running.
func (s *Steps) HealthCheck(ctx context.Context, host string, port int) error {
	url := fmt.Sprintf(healthCheckURL, host, port)

	var healthCheckResp healthCheckResponse

	resp, err := httputil.DoRequest(ctx, url, httputil.WithHTTPClient(s.HTTPClient),
		httputil.WithParsedResponse(&healthCheckResp))
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}

	s.responseStatus = resp.Status
	s.responseBody = resp.Body

	if resp.StatusCode == http.StatusOK && healthCheckResp.Status == "success" {
		return nil
	}

	return fmt.Errorf("health check failed")
}

func (s *Steps) httpGet(ctx context.Context, url string) error {
	resp, err := httputil.DoRequest(ctx, url, httputil.WithHTTPClient(s.HTTPClient))
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}

	s.responseStatus = resp.Status
	s.responseBody = resp.Body

	return nil
}

func (s *Steps) httpPost(ctx context.Context, url string, docStr *godog.DocString) error {
	return s.httpDo(ctx, http.MethodPost, url, docStr)
}

func (s *Steps) httpPut(ctx context.Context, url string, docStr *godog.DocString) error {
	return s.httpDo(ctx, http.MethodPut, url, docStr)
}

type requestParamsKey struct{}

// ContextWithRequestParams creates a new context.Context with request params value.
// Later HTTP POST request gets that value under requestParamsKey and prepares request body.
func ContextWithRequestParams(ctx context.Context, params interface{}) context.Context {
	return context.WithValue(ctx, requestParamsKey{}, params)
}

func (s *Steps) httpDo(ctx context.Context, method, url string, docStr *godog.DocString) error {
	var buf bytes.Buffer

	t, err := template.New("request").Parse(docStr.Content)
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	err = t.Execute(&buf, ctx.Value(requestParamsKey{}))
	if err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	resp, err := httputil.DoRequest(ctx, url, httputil.WithHTTPClient(s.HTTPClient),
		httputil.WithMethod(method), httputil.WithBody(buf.Bytes()))
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}

	s.responseStatus = resp.Status
	s.responseStatusCode = resp.StatusCode
	s.responseBody = resp.Body

	return nil
}

func (s *Steps) checkResponseStatus(status string) error {
	if s.responseStatus != status {
		return fmt.Errorf("got %q", s.responseStatus)
	}

	return nil
}

func (s *Steps) checkResponseValue(path, value string) error {
	res := gjson.Get(string(s.responseBody), path)

	if res.Str != value {
		return fmt.Errorf("got %q", res.Str)
	}

	return nil
}

func (s *Steps) checkNonEmptyResponseValue(path string) error {
	res := gjson.Get(string(s.responseBody), path)

	if res.Str == "" {
		return fmt.Errorf("got empty value")
	}

	return nil
}
