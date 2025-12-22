package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
)

const testCanonicalSite = "lds.li"

// TestCase defines a single test scenario
type TestCase struct {
	Name        string
	Request     Request
	Validator   func(response Response) error
	Description string
}

// Request models the CloudFront function event request
type Request struct {
	URI         string
	Host        string
	Method      string
	Querystring map[string]string
	Headers     map[string]string // Additional headers
}

// Response models the CloudFront function output (inner object)
type Response struct {
	StatusCode        int                  `json:"statusCode"`
	StatusDescription string               `json:"statusDescription"`
	Headers           map[string]HeaderVal `json:"headers"`
	Body              *Body                `json:"body"`
	// If it returns the request (pass-through)
	URI *string `json:"uri"`
}

// TestOutputWrapper handles the outer JSON object returned by TestFunction
type TestOutputWrapper struct {
	Response *Response              `json:"response"`
	Request  map[string]interface{} `json:"request"`
}

type HeaderVal struct {
	Value string `json:"value"`
}

type Body struct {
	Encoding string `json:"encoding"`
	Data     string `json:"data"`
}

// Suite returns the list of tests to run
func Suite(email string) []TestCase {
	return []TestCase{
		{
			Name: "Canonical Host Redirect",
			Request: Request{
				URI:  "/foo",
				Host: "non-canonical.example.com", // Assume canonicalHost is lds.li
			},
			Validator: func(resp Response) error {
				if resp.StatusCode != 301 {
					return fmt.Errorf("expected status 301, got %d", resp.StatusCode)
				}
				loc := resp.Headers["location"].Value
				if loc != "https://"+testCanonicalSite+"/foo" {
					return fmt.Errorf("expected location https://"+testCanonicalSite+"/foo, got %s", loc)
				}
				return nil
			},
		}, {
			Name: "Webfinger",
			Request: Request{
				URI:  "/.well-known/webfinger",
				Host: testCanonicalSite,
				Querystring: map[string]string{
					"resource": url.QueryEscape("acct:" + email),
				},
			},
			Validator: func(resp Response) error {
				if resp.StatusCode != 200 {
					return fmt.Errorf("expected status 200, got %d", resp.StatusCode)
				}
				if resp.Headers["content-type"].Value != "application/json" {
					return fmt.Errorf("expected json content type")
				}
				if resp.Body == nil || !strings.Contains(resp.Body.Data, "acct:"+email) {
					return fmt.Errorf("expected webfinger body to contain email")
				}
				return nil
			},
		},
		{
			Name: "Go Module Meta (go-get=1)",
			Request: Request{
				URI:  "/oauth2ext",
				Host: testCanonicalSite,
				Querystring: map[string]string{
					"go-get": "1",
				},
			},
			Validator: func(resp Response) error {
				if resp.StatusCode != 200 {
					return fmt.Errorf("expected status 200, got %d", resp.StatusCode)
				}
				if resp.Body == nil || !strings.Contains(resp.Body.Data, "go-import") {
					return fmt.Errorf("expected go-import meta tag")
				}
				return nil
			},
		},
		{
			Name: "Go Module Redirect (Godoc)",
			Request: Request{
				URI:  "/oauth2ext",
				Host: testCanonicalSite,
			},
			Validator: func(resp Response) error {
				if resp.StatusCode != 302 {
					return fmt.Errorf("expected status 302, got %d", resp.StatusCode)
				}
				loc := resp.Headers["location"].Value
				// Verify redirect target matches the configured URL
				if loc != "https://pkg.go.dev/lds.li/oauth2ext" {
					return fmt.Errorf("unexpected location: %s", loc)
				}
				return nil
			},
		},
		{
			Name: "Go Module Subpackage Redirect",
			Request: Request{
				URI:  "/oauth2ext/subpkg",
				Host: testCanonicalSite,
			},
			Validator: func(resp Response) error {
				if resp.StatusCode != 302 {
					return fmt.Errorf("expected status 302, got %d", resp.StatusCode)
				}
				loc := resp.Headers["location"].Value
				if loc != "https://pkg.go.dev/lds.li/oauth2ext/subpkg" {
					return fmt.Errorf("unexpected location: %s", loc)
				}
				return nil
			},
		},
		{
			Name: "Pass-through (Static Asset)",
			Request: Request{
				URI:  "/static/style.css",
				Host: testCanonicalSite,
			},
			Validator: func(resp Response) error {
				if resp.StatusCode != 0 {
					return fmt.Errorf("expected pass-through (no status code), got %d", resp.StatusCode)
				}
				return nil
			},
		},
	}
}

// Run executes the tests against the specified CloudFront Function
func RunTests(ctx context.Context, client *cloudfront.Client, name, etag, email string, logger *slog.Logger) error {
	tests := Suite(email)
	failed := 0

	for _, tc := range tests {
		logger.Info("Running test", "name", tc.Name)

		eventBytes, err := buildEvent(tc.Request)
		if err != nil {
			return fmt.Errorf("failed to build event for %s: %w", tc.Name, err)
		}

		out, err := client.TestFunction(ctx, &cloudfront.TestFunctionInput{
			Name:        &name,
			IfMatch:     &etag,
			Stage:       types.FunctionStageDevelopment,
			EventObject: eventBytes,
		})
		if err != nil {
			return fmt.Errorf("AWS API error testing %s: %w", tc.Name, err)
		}

		if out.TestResult.FunctionErrorMessage != nil && *out.TestResult.FunctionErrorMessage != "" {
			msg := *out.TestResult.FunctionErrorMessage
			logger.Error("Test failed (runtime error)", "name", tc.Name, "error", msg)

			if len(out.TestResult.FunctionExecutionLogs) > 0 {
				fmt.Println("---")
				for _, l := range out.TestResult.FunctionExecutionLogs {
					fmt.Println(l)
				}
				fmt.Println("---")
			}

			if out.TestResult.FunctionOutput != nil {
				fmt.Printf("Partial Output: %s\n", *out.TestResult.FunctionOutput)
			}

			failed++
			continue
		}

		var wrapper TestOutputWrapper
		if err := json.Unmarshal([]byte(*out.TestResult.FunctionOutput), &wrapper); err != nil {
			logger.Error("Test failed (invalid output JSON)", "name", tc.Name, "error", err)
			fmt.Println("Output:", *out.TestResult.FunctionOutput)
			failed++
			continue
		}

		var resp Response
		if wrapper.Response != nil {
			resp = *wrapper.Response
		} else if wrapper.Request != nil {
			// Pass-through
			resp = Response{StatusCode: 0}
		} else {
			logger.Error("Test failed (unknown output structure)", "name", tc.Name)
			fmt.Println("Output:", *out.TestResult.FunctionOutput)
			failed++
			continue
		}

		if err := tc.Validator(resp); err != nil {
			logger.Error("Test failed (assertion)", "name", tc.Name, "error", err)
			failed++
		} else {
			util := "unknown"
			if out.TestResult.ComputeUtilization != nil {
				util = *out.TestResult.ComputeUtilization
			}
			logger.Info("Test passed", "name", tc.Name, "compute_utilization", util)
		}
	}

	if failed > 0 {
		return fmt.Errorf("%d tests failed", failed)
	}
	return nil
}

func buildEvent(req Request) ([]byte, error) {
	hdrs := make(map[string]HeaderVal)
	hdrs["host"] = HeaderVal{Value: req.Host}
	for k, v := range req.Headers {
		hdrs[strings.ToLower(k)] = HeaderVal{Value: v}
	}

	qs := make(map[string]HeaderVal)
	for k, v := range req.Querystring {
		qs[k] = HeaderVal{Value: v}
	}

	event := map[string]interface{}{
		"version": "1.0",
		"context": map[string]string{
			"eventType": "viewer-request",
		},
		"viewer": map[string]string{
			"ip": "1.2.3.4",
		},
		"request": map[string]interface{}{
			"method":      req.Method,
			"uri":         req.URI,
			"headers":     hdrs,
			"querystring": qs,
			"cookies":     map[string]interface{}{}, // Required by CloudFront
		},
	}

	if event["request"].(map[string]interface{})["method"] == "" {
		event["request"].(map[string]interface{})["method"] = "GET"
	}

	return json.Marshal(event)
}
