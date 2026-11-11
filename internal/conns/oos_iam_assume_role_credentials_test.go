// Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package conns

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	smithy "github.com/aws/smithy-go"
	awsbase "github.com/hashicorp/aws-sdk-go-base/v2"
)

// TC-OOS-PROVIDER-26 AssumeRole Credentials Provider Single Role verifies that the XML response for a single role can be converted into complete temporary credentials.
func TestUnitOOSIAMAssumeRoleCredentialsProvider_SingleRole(t *testing.T) {
	t.Parallel()

	const expiration = "2026-08-11T12:34:56.123456Z"
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("starting mock AssumeRole listener: %v", err)
	}
	server := &httptest.Server{Listener: listener, Config: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.FormValue("DurationSeconds"); got != "" {
			t.Errorf("DurationSeconds = %q, want omitted when duration is zero", got)
		}
		if r.Method != http.MethodPost {
			t.Errorf("request method = %s, want POST", r.Method)
		}
		if _, err := io.WriteString(w, `<AssumeRoleResponse><AssumeRoleResult><Credentials><AccessKeyId>assumed-access-key</AccessKeyId><SecretAccessKey>assumed-secret-key</SecretAccessKey><SessionToken>assumed-session-token</SessionToken><Expiration>`+expiration+`</Expiration></Credentials></AssumeRoleResult></AssumeRoleResponse>`); err != nil {
			t.Errorf("writing mock AssumeRole response: %v", err)
		}
	})}}
	server.Start()
	defer server.Close()

	source := aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{
			AccessKeyID:     "source-access-key",
			SecretAccessKey: "source-secret-key",
		}, nil
	})
	provider := newOOSIAMAssumeRoleCredentialsProvider(source, server.URL, server.Client(), []awsbase.AssumeRole{
		{
			RoleARN:     "arn:ctyun:iam::32fefj64y54gc:role/AssumeRole",
			SessionName: "test-session",
		},
	}, "shanghai")

	got, err := provider.Retrieve(t.Context())
	if err != nil {
		t.Fatalf("retrieving assumed credentials: %v", err)
	}

	wantExpiration, err := time.Parse(time.RFC3339Nano, expiration)
	if err != nil {
		t.Fatal(err)
	}
	want := aws.Credentials{
		AccessKeyID:     "assumed-access-key",
		SecretAccessKey: "assumed-secret-key",
		SessionToken:    "assumed-session-token",
		Source:          "OOS IAM AssumeRole",
		CanExpire:       true,
		Expires:         wantExpiration,
	}
	if got != want {
		t.Fatalf("assumed credentials = %#v, want %#v", got, want)
	}
}

// TC-OOS-PROVIDER-26 AssumeRole Credentials Provider Single Role verifies that the default client is used when no HTTP client is provided.
// TestUnitOOSIAMAssumeRoleCredentialsProvider_HTTPClientFallback verifies the fallback path for a nil client.
func TestUnitOOSIAMAssumeRoleCredentialsProvider_HTTPClientFallback(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.WriteString(w, `<AssumeRoleResponse><AssumeRoleResult><Credentials><AccessKeyId>assumed-access-key</AccessKeyId><SecretAccessKey>assumed-secret-key</SecretAccessKey><SessionToken>assumed-session-token</SessionToken></Credentials></AssumeRoleResult></AssumeRoleResponse>`); err != nil {
			t.Errorf("writing mock AssumeRole response: %v", err)
		}
	}))
	defer server.Close()

	source := aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{AccessKeyID: "source-access-key", SecretAccessKey: "source-secret-key"}, nil
	})
	provider := newOOSIAMAssumeRoleCredentialsProvider(source, server.URL, nil, []awsbase.AssumeRole{
		{RoleARN: "arn:ctyun:iam::32fefj64y54gc:role/AssumeRole", SessionName: "test-session"},
	}, "shanghai")

	got, err := provider.Retrieve(t.Context())
	if err != nil {
		t.Fatalf("retrieving assumed credentials with default HTTP client: %v", err)
	}
	if got.AccessKeyID != "assumed-access-key" || got.SecretAccessKey != "assumed-secret-key" || got.SessionToken != "assumed-session-token" {
		t.Fatalf("assumed credentials = %#v, want mock credentials", got)
	}
}

// TC-OOS-PROVIDER-28 AssumeRole Credentials Provider HTTP Error verifies a clear failure when the 2xx response body cannot be decoded.
// TestUnitOOSIAMAssumeRoleCredentialsProvider_ResponseDecodeError verifies wrapping of XML decode errors under a success status code.
func TestUnitOOSIAMAssumeRoleCredentialsProvider_ResponseDecodeError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.WriteString(w, "OK"); err != nil {
			t.Errorf("writing malformed AssumeRole response: %v", err)
		}
	}))
	defer server.Close()

	source := aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{AccessKeyID: "source-access-key", SecretAccessKey: "source-secret-key"}, nil
	})
	provider := newOOSIAMAssumeRoleCredentialsProvider(source, server.URL, server.Client(), []awsbase.AssumeRole{
		{RoleARN: "arn:ctyun:iam::32fefj64y54gc:role/AssumeRole", SessionName: "test-session"},
	}, "shanghai")

	got, err := provider.Retrieve(t.Context())
	if err == nil {
		t.Fatal("expected response decoding error")
	}
	if got != (aws.Credentials{}) {
		t.Fatalf("credentials = %#v, want zero value", got)
	}
	if !strings.Contains(err.Error(), "decoding OOS IAM AssumeRole response:") {
		t.Fatalf("error = %q, want response decoding context", err)
	}
}

// TC-OOS-PROVIDER-31 AssumeRole Credentials Provider Nil Source verifies that the root cause is preserved when source credential retrieval fails.
// TestUnitOOSIAMAssumeRoleCredentialsProvider_SourceRetrievalError verifies wrapping of Retrieve errors from the source provider.
func TestUnitOOSIAMAssumeRoleCredentialsProvider_SourceRetrievalError(t *testing.T) {
	t.Parallel()

	sourceError := errors.New("source credential retrieval failed")
	source := aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{}, sourceError
	})
	provider := newOOSIAMAssumeRoleCredentialsProvider(source, "https://unused.example.com", http.DefaultClient, []awsbase.AssumeRole{
		{RoleARN: "arn:ctyun:iam::32fefj64y54gc:role/AssumeRole", SessionName: "test-session"},
	}, "shanghai")

	got, err := provider.Retrieve(t.Context())
	if err == nil {
		t.Fatal("expected source credentials retrieval error")
	}
	if got != (aws.Credentials{}) {
		t.Fatalf("credentials = %#v, want zero value", got)
	}
	if !errors.Is(err, sourceError) {
		t.Fatalf("error = %v, want wrapped source retrieval error", err)
	}
	if got, want := err.Error(), "retrieving OOS IAM AssumeRole source credentials: source credential retrieval failed"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

// TC-OOS-PROVIDER-32 AssumeRole Credentials Provider Empty Endpoint verifies a clear failure when a malformed IAM endpoint is used to create a request.
// TestUnitOOSIAMAssumeRoleCredentialsProvider_InvalidEndpoint verifies error wrapping when the endpoint URL cannot be used to build an HTTP request.
func TestUnitOOSIAMAssumeRoleCredentialsProvider_InvalidEndpoint(t *testing.T) {
	t.Parallel()

	source := aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{AccessKeyID: "source-access-key", SecretAccessKey: "source-secret-key"}, nil
	})
	provider := newOOSIAMAssumeRoleCredentialsProvider(source, "://invalid", http.DefaultClient, []awsbase.AssumeRole{
		{RoleARN: "arn:ctyun:iam::32fefj64y54gc:role/AssumeRole", SessionName: "test-session"},
	}, "shanghai")

	got, err := provider.Retrieve(t.Context())
	if err == nil {
		t.Fatal("expected malformed endpoint error")
	}
	if got != (aws.Credentials{}) {
		t.Fatalf("credentials = %#v, want zero value", got)
	}
	if !strings.Contains(err.Error(), "creating OOS IAM AssumeRole request:") {
		t.Fatalf("error = %q, want request creation context", err)
	}
	if !strings.Contains(err.Error(), "missing protocol scheme") {
		t.Fatalf("error = %q, want malformed endpoint cause", err)
	}
}

// TC-OOS-PROVIDER-35 AssumeRole With All Optional Fields verifies that all optional fields are encoded into the OOS IAM request.
func TestUnitOOSIAMAssumeRoleCredentialsProvider_AllOptionalFields(t *testing.T) {
	t.Parallel()

	const (
		externalID     = "oos-test-external-id"
		sourceIdentity = "oos-test-source"
		policy         = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"oos:*","Resource":"*"}]}`
	)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("starting mock AssumeRole listener: %v", err)
	}
	server := &httptest.Server{Listener: listener, Config: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.FormValue("RoleSessionName"); got != "test-session" {
			t.Errorf("RoleSessionName = %q, want %q", got, "test-session")
		}
		if got := r.FormValue("DurationSeconds"); got != "3600" {
			t.Errorf("DurationSeconds = %q, want %q", got, "3600")
		}
		if got := r.FormValue("ExternalId"); got != externalID {
			t.Errorf("ExternalId = %q, want %q", got, externalID)
		}
		if got := r.FormValue("Policy"); got != policy {
			t.Errorf("Policy = %q, want %q", got, policy)
		}
		if got := r.FormValue("SourceIdentity"); got != sourceIdentity {
			t.Errorf("SourceIdentity = %q, want %q", got, sourceIdentity)
		}
		if _, err := io.WriteString(w, `<AssumeRoleResponse><AssumeRoleResult><Credentials><AccessKeyId>assumed-access-key</AccessKeyId><SecretAccessKey>assumed-secret-key</SecretAccessKey><SessionToken>assumed-session-token</SessionToken><Expiration>2026-08-11T12:34:56Z</Expiration></Credentials></AssumeRoleResult></AssumeRoleResponse>`); err != nil {
			t.Errorf("writing mock AssumeRole response: %v", err)
		}
	})}}
	server.Start()
	defer server.Close()

	source := aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{AccessKeyID: "source-access-key", SecretAccessKey: "source-secret-key"}, nil
	})
	provider := newOOSIAMAssumeRoleCredentialsProvider(source, server.URL, server.Client(), []awsbase.AssumeRole{
		{
			RoleARN:        "arn:ctyun:iam::32fefj64y54gc:role/AssumeRole",
			SessionName:    "test-session",
			Duration:       time.Hour,
			ExternalID:     externalID,
			Policy:         policy,
			SourceIdentity: sourceIdentity,
		},
	}, "shanghai")

	if _, err := provider.Retrieve(t.Context()); err != nil {
		t.Fatalf("retrieving assumed credentials: %v", err)
	}
}

// TC-OOS-PROVIDER-27 AssumeRole Credentials Provider Role Chain verifies that a subsequent role request is signed with the temporary credentials of the previous role.
func TestUnitOOSIAMAssumeRoleCredentialsProvider_RoleChain(t *testing.T) {
	t.Parallel()

	responses := []string{
		`<AssumeRoleResponse><AssumeRoleResult><Credentials><AccessKeyId>role-a-access-key</AccessKeyId><SecretAccessKey>role-a-secret-key</SecretAccessKey><SessionToken>role-a-session-token</SessionToken><Expiration>2026-08-11T12:34:56Z</Expiration></Credentials></AssumeRoleResult></AssumeRoleResponse>`,
		`<AssumeRoleResponse><AssumeRoleResult><Credentials><AccessKeyId>role-b-access-key</AccessKeyId><SecretAccessKey>role-b-secret-key</SecretAccessKey><SessionToken>role-b-session-token</SessionToken><Expiration>2026-08-11T13:34:56Z</Expiration></Credentials></AssumeRoleResult></AssumeRoleResponse>`,
	}
	expectedRoleARNs := []string{
		"arn:ctyun:iam::32fefj64y54gc:role/RoleA",
		"arn:ctyun:iam::32fefj64y54gc:role/RoleB",
	}
	var authorizations []string
	var securityTokens []string
	requestCount := 0

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("starting mock AssumeRole listener: %v", err)
	}
	server := &httptest.Server{Listener: listener, Config: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requestCount >= len(responses) {
			http.Error(w, "unexpected AssumeRole request", http.StatusInternalServerError)
			return
		}

		index := requestCount
		requestCount++
		if got := r.FormValue("RoleArn"); got != expectedRoleARNs[index] {
			t.Errorf("request %d RoleArn = %q, want %q", index+1, got, expectedRoleARNs[index])
		}
		authorizations = append(authorizations, r.Header.Get("Authorization"))
		securityTokens = append(securityTokens, r.Header.Get("X-Amz-Security-Token"))
		if _, err := io.WriteString(w, responses[index]); err != nil {
			t.Errorf("writing mock AssumeRole response: %v", err)
		}
	})}}
	server.Start()
	defer server.Close()

	source := aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{
			AccessKeyID:     "source-access-key",
			SecretAccessKey: "source-secret-key",
		}, nil
	})
	provider := newOOSIAMAssumeRoleCredentialsProvider(source, server.URL, server.Client(), []awsbase.AssumeRole{
		{RoleARN: expectedRoleARNs[0], SessionName: "role-a-session"},
		{RoleARN: expectedRoleARNs[1], SessionName: "role-b-session"},
	}, "shanghai")

	got, err := provider.Retrieve(t.Context())
	if err != nil {
		t.Fatalf("retrieving chained assumed credentials: %v", err)
	}

	wantExpiration, err := time.Parse(time.RFC3339, "2026-08-11T13:34:56Z")
	if err != nil {
		t.Fatal(err)
	}
	want := aws.Credentials{
		AccessKeyID:     "role-b-access-key",
		SecretAccessKey: "role-b-secret-key",
		SessionToken:    "role-b-session-token",
		Source:          "OOS IAM AssumeRole",
		CanExpire:       true,
		Expires:         wantExpiration,
	}
	if got != want {
		t.Fatalf("chained assumed credentials = %#v, want %#v", got, want)
	}
	if requestCount != 2 {
		t.Fatalf("AssumeRole request count = %d, want 2", requestCount)
	}
	if !strings.Contains(authorizations[0], "Credential=source-access-key/") {
		t.Fatalf("first Authorization = %q, want source access key", authorizations[0])
	}
	if !strings.Contains(authorizations[1], "Credential=role-a-access-key/") {
		t.Fatalf("second Authorization = %q, want role A access key", authorizations[1])
	}
	if got := securityTokens[1]; got != "role-a-session-token" {
		t.Fatalf("second X-Amz-Security-Token = %q, want role A session token", got)
	}
}

// TC-OOS-PROVIDER-28 AssumeRole Credentials Provider HTTP Error verifies that Retrieve preserves the HTTP error context and status code.
func TestUnitOOSIAMAssumeRoleCredentialsProvider_HTTPError(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		statusCode     int
		expectedStatus string
	}{
		{name: "forbidden", statusCode: http.StatusForbidden, expectedStatus: "HTTP 403"},
		{name: "internal server error", statusCode: http.StatusInternalServerError, expectedStatus: "HTTP 500"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			listener, err := net.Listen("tcp4", "127.0.0.1:0")
			if err != nil {
				t.Fatalf("starting mock AssumeRole listener: %v", err)
			}
			server := &httptest.Server{Listener: listener, Config: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.statusCode)
				if _, err := io.WriteString(w, `<Error><Code>AccessDenied</Code><Message>not authorized</Message></Error>`); err != nil {
					t.Errorf("writing mock AssumeRole error response: %v", err)
				}
			})}}
			server.Start()
			defer server.Close()

			source := aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
				return aws.Credentials{AccessKeyID: "source-access-key", SecretAccessKey: "source-secret-key"}, nil
			})
			provider := newOOSIAMAssumeRoleCredentialsProvider(source, server.URL, server.Client(), []awsbase.AssumeRole{
				{RoleARN: "arn:ctyun:iam::32fefj64y54gc:role/AssumeRole", SessionName: "test-session"},
			}, "shanghai")

			_, err = provider.Retrieve(t.Context())
			if err == nil {
				t.Fatal("expected Retrieve error")
			}
			if !strings.Contains(err.Error(), "calling OOS IAM AssumeRole: HTTP") {
				t.Fatalf("error = %q, want OOS IAM AssumeRole HTTP context", err)
			}
			if !strings.Contains(err.Error(), tc.expectedStatus) {
				t.Fatalf("error = %q, want status %q", err, tc.expectedStatus)
			}
		})
	}
}

// TC-OOS-PROVIDER-28 AssumeRole Credentials Provider HTTP Error verifies parsing of OOS IAM XML and non-XML error responses.
func TestUnitOOSIAMAssumeRole_Error(t *testing.T) {
	t.Parallel()

	t.Run("XML error request ID selection", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name          string
			header        http.Header
			responseBody  string
			wantRequestID string
		}{
			{
				name:          "response body takes precedence",
				header:        http.Header{"X-Amz-Request-Id": {"header-request-id"}},
				responseBody:  `<Error><Code>AccessDenied</Code><Message>not authorized</Message><RequestId>body-request-id</RequestId></Error>`,
				wantRequestID: "body-request-id",
			},
			{
				name:          "X-Amz-Request-Id fallback",
				header:        http.Header{"X-Amz-Request-Id": {"amz-header-request-id"}},
				responseBody:  `<Error><Code>AccessDenied</Code><Message>not authorized</Message></Error>`,
				wantRequestID: "amz-header-request-id",
			},
			{
				name:          "X-Request-Id fallback",
				header:        http.Header{"X-Request-Id": {"generic-header-request-id"}},
				responseBody:  `<Error><Code>AccessDenied</Code><Message>not authorized</Message></Error>`,
				wantRequestID: "generic-header-request-id",
			},
			{
				name:         "no request ID",
				header:       http.Header{},
				responseBody: `<Error><Code>AccessDenied</Code><Message>not authorized</Message></Error>`,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				resp := &http.Response{StatusCode: http.StatusForbidden, Header: tc.header}
				err := parseOOSIAMAssumeRoleError(resp, []byte(tc.responseBody))

				var apiErr smithy.APIError
				if !errors.As(err, &apiErr) {
					t.Fatalf("expected Smithy API error, got %T: %v", err, err)
				}
				if got, want := apiErr.ErrorCode(), "AccessDenied"; got != want {
					t.Errorf("ErrorCode = %q, want %q", got, want)
				}
				if got, want := apiErr.ErrorMessage(), "not authorized"; got != want {
					t.Errorf("ErrorMessage = %q, want %q", got, want)
				}

				if tc.wantRequestID == "" {
					if strings.Contains(err.Error(), "RequestID") {
						t.Errorf("error = %q, want no request ID", err)
					}
					return
				}

				if !strings.Contains(err.Error(), "RequestID "+tc.wantRequestID) {
					t.Errorf("error = %q, want request ID %q", err, tc.wantRequestID)
				}
			})
		}
	})

	t.Run("XML error code and message fallback", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name         string
			responseBody string
			wantCode     string
			wantMessage  string
		}{
			{
				name:         "missing code",
				responseBody: `<Error><Message>not authorized</Message></Error>`,
				wantCode:     "UnknownError",
				wantMessage:  "not authorized",
			},
			{
				name:         "missing message",
				responseBody: `<Error><Code>AccessDenied</Code></Error>`,
				wantCode:     "AccessDenied",
				wantMessage:  `<Error><Code>AccessDenied</Code></Error>`,
			},
			{
				name:         "unexpected XML root",
				responseBody: `<Other><Detail>unexpected response</Detail></Other>`,
				wantCode:     "UnknownError",
				wantMessage:  `<Other><Detail>unexpected response</Detail></Other>`,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				resp := &http.Response{StatusCode: http.StatusForbidden, Header: http.Header{}}
				err := parseOOSIAMAssumeRoleError(resp, []byte(tc.responseBody))

				var apiErr smithy.APIError
				if !errors.As(err, &apiErr) {
					t.Fatalf("expected Smithy API error, got %T: %v", err, err)
				}
				if got := apiErr.ErrorCode(); got != tc.wantCode {
					t.Errorf("ErrorCode = %q, want %q", got, tc.wantCode)
				}
				if got := apiErr.ErrorMessage(); got != tc.wantMessage {
					t.Errorf("ErrorMessage = %q, want %q", got, tc.wantMessage)
				}
			})
		}
	})

	t.Run("non XML response", func(t *testing.T) {
		t.Parallel()

		resp := &http.Response{StatusCode: http.StatusBadGateway, Header: http.Header{}}
		err := parseOOSIAMAssumeRoleError(resp, []byte("upstream unavailable"))

		if !strings.Contains(err.Error(), "unable to parse XML error response: upstream unavailable") {
			t.Errorf("error = %q, want diagnostic response body", err)
		}
	})
}

// TC-OOS-PROVIDER-29 AssumeRole Credentials Provider Missing Credentials verifies a clear failure when the returned credential fields are incomplete.
func TestUnitOOSIAMAssumeRoleCredentialsProvider_MissingCredentials(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name            string
		accessKeyID     string
		secretAccessKey string
		sessionToken    string
	}{
		{name: "missing access key ID", secretAccessKey: "secret-key", sessionToken: "session-token"},
		{name: "missing secret access key", accessKeyID: "access-key", sessionToken: "session-token"},
		{name: "missing session token", accessKeyID: "access-key", secretAccessKey: "secret-key"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			listener, err := net.Listen("tcp4", "127.0.0.1:0")
			if err != nil {
				t.Fatalf("starting mock AssumeRole listener: %v", err)
			}
			server := &httptest.Server{Listener: listener, Config: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				response := `<AssumeRoleResponse><AssumeRoleResult><Credentials><AccessKeyId>` + tc.accessKeyID + `</AccessKeyId><SecretAccessKey>` + tc.secretAccessKey + `</SecretAccessKey><SessionToken>` + tc.sessionToken + `</SessionToken><Expiration>2026-08-11T12:34:56Z</Expiration></Credentials></AssumeRoleResult></AssumeRoleResponse>`
				if _, err := io.WriteString(w, response); err != nil {
					t.Errorf("writing mock AssumeRole response: %v", err)
				}
			})}}
			server.Start()
			defer server.Close()

			source := aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
				return aws.Credentials{AccessKeyID: "source-access-key", SecretAccessKey: "source-secret-key"}, nil
			})
			provider := newOOSIAMAssumeRoleCredentialsProvider(source, server.URL, server.Client(), []awsbase.AssumeRole{
				{RoleARN: "arn:ctyun:iam::32fefj64y54gc:role/AssumeRole", SessionName: "test-session"},
			}, "shanghai")

			_, err = provider.Retrieve(t.Context())
			if err == nil {
				t.Fatal("expected missing credentials error")
			}
			const expectedError = "decoding OOS IAM AssumeRole response: missing credentials"
			if !strings.Contains(err.Error(), expectedError) {
				t.Fatalf("error = %q, want %q", err, expectedError)
			}
		})
	}
}

// TC-OOS-PROVIDER-30 AssumeRole Credentials Provider Expiration Parse verifies tolerant parsing of temporary credential expiration times.
func TestUnitOOSIAMAssumeRoleCredentialsProvider_ExpirationParse(t *testing.T) {
	t.Parallel()

	rfc3339Expiration := "2026-08-11T12:34:56Z"
	rfc3339NanoExpiration := "2026-08-11T12:34:56.123456789Z"
	rfc3339Time, err := time.Parse(time.RFC3339, rfc3339Expiration)
	if err != nil {
		t.Fatal(err)
	}
	rfc3339NanoTime, err := time.Parse(time.RFC3339Nano, rfc3339NanoExpiration)
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name           string
		expiration     string
		wantCanExpire  bool
		wantExpiration time.Time
		expectedError  string
	}{
		{name: "RFC3339", expiration: rfc3339Expiration, wantCanExpire: true, wantExpiration: rfc3339Time},
		{name: "RFC3339Nano", expiration: rfc3339NanoExpiration, wantCanExpire: true, wantExpiration: rfc3339NanoTime},
		{name: "empty", expiration: "", wantCanExpire: false},
		{name: "invalid", expiration: "not-a-timestamp", expectedError: "unsupported time format"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			listener, err := net.Listen("tcp4", "127.0.0.1:0")
			if err != nil {
				t.Fatalf("starting mock AssumeRole listener: %v", err)
			}
			server := &httptest.Server{Listener: listener, Config: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				response := `<AssumeRoleResponse><AssumeRoleResult><Credentials><AccessKeyId>access-key</AccessKeyId><SecretAccessKey>secret-key</SecretAccessKey><SessionToken>session-token</SessionToken><Expiration>` + tc.expiration + `</Expiration></Credentials></AssumeRoleResult></AssumeRoleResponse>`
				if _, err := io.WriteString(w, response); err != nil {
					t.Errorf("writing mock AssumeRole response: %v", err)
				}
			})}}
			server.Start()
			defer server.Close()

			source := aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
				return aws.Credentials{AccessKeyID: "source-access-key", SecretAccessKey: "source-secret-key"}, nil
			})
			provider := newOOSIAMAssumeRoleCredentialsProvider(source, server.URL, server.Client(), []awsbase.AssumeRole{
				{RoleARN: "arn:ctyun:iam::32fefj64y54gc:role/AssumeRole", SessionName: "test-session"},
			}, "shanghai")

			got, err := provider.Retrieve(t.Context())
			if tc.expectedError != "" {
				if err == nil {
					t.Fatal("expected expiration parse error")
				}
				if !strings.Contains(err.Error(), tc.expectedError) {
					t.Fatalf("error = %q, want %q", err, tc.expectedError)
				}
				return
			}
			if err != nil {
				t.Fatalf("retrieving assumed credentials: %v", err)
			}
			if got.CanExpire != tc.wantCanExpire {
				t.Fatalf("CanExpire = %t, want %t", got.CanExpire, tc.wantCanExpire)
			}
			if !got.Expires.Equal(tc.wantExpiration) {
				t.Fatalf("Expires = %s, want %s", got.Expires, tc.wantExpiration)
			}
		})
	}
}

// TC-OOS-PROVIDER-31 AssumeRole Credentials Provider Nil Source verifies that no AssumeRole request is made when source credentials are missing.
func TestUnitOOSIAMAssumeRoleCredentialsProvider_NilSource(t *testing.T) {
	t.Parallel()

	provider := newOOSIAMAssumeRoleCredentialsProvider(nil, "https://oos-shanghai-iam.ctyunapi.cn", nil, nil, "shanghai")
	_, err := provider.Retrieve(t.Context())
	if err == nil {
		t.Fatal("expected source credentials configuration error")
	}
	const expectedError = "OOS IAM AssumeRole source credentials are not configured"
	if got := err.Error(); got != expectedError {
		t.Fatalf("error = %q, want %q", got, expectedError)
	}
}

// TC-OOS-PROVIDER-32 AssumeRole Credentials Provider Empty Endpoint verifies that no AssumeRole request is made when the IAM endpoint is missing.
func TestUnitOOSIAMAssumeRoleCredentialsProvider_EmptyEndpoint(t *testing.T) {
	t.Parallel()

	source := aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{AccessKeyID: "source-access-key", SecretAccessKey: "source-secret-key"}, nil
	})
	provider := newOOSIAMAssumeRoleCredentialsProvider(source, "", nil, nil, "shanghai")
	_, err := provider.Retrieve(t.Context())
	if err == nil {
		t.Fatal("expected IAM endpoint configuration error")
	}
	const expectedError = "OOS IAM AssumeRole endpoint is not configured"
	if got := err.Error(); got != expectedError {
		t.Fatalf("error = %q, want %q", got, expectedError)
	}
}

// TC-OOS-PROVIDER-32 AssumeRole Credentials Provider Empty Signing Region verifies that no request is made when the signing region is missing.
func TestUnitOOSIAMAssumeRoleCredentialsProvider_EmptySigningRegion(t *testing.T) {
	t.Parallel()

	source := aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{AccessKeyID: "source-access-key", SecretAccessKey: "source-secret-key"}, nil
	})
	provider := newOOSIAMAssumeRoleCredentialsProvider(source, "https://oos-shanghai-iam.ctyunapi.cn", nil, []awsbase.AssumeRole{{
		RoleARN: "arn:ctyun:iam::32fefj64y54gc:role/AssumeRole",
	}}, "")
	_, err := provider.Retrieve(t.Context())
	if err == nil {
		t.Fatal("expected signing region configuration error")
	}
	const expectedError = "OOS IAM AssumeRole signing region is not configured"
	if got := err.Error(); got != expectedError {
		t.Fatalf("error = %q, want %q", got, expectedError)
	}
}

// TC-OOS-PROVIDER-33 ParseOOSIAMAssumeRoleExpiration verifies parsing of OOS IAM AssumeRole expiration times.
func TestUnitOOSIAMAssumeRole_Expiration(t *testing.T) {
	t.Parallel()

	rfc3339 := "2026-08-04T12:00:00Z"
	rfc3339Nano := "2026-08-04T12:00:00.123456789Z"
	rfc3339Time, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		t.Fatal(err)
	}
	rfc3339NanoTime, err := time.Parse(time.RFC3339Nano, rfc3339Nano)
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name      string
		input     string
		want      time.Time
		wantError bool
	}{
		{name: "empty", input: ""},
		{name: "RFC3339", input: rfc3339, want: rfc3339Time},
		{name: "RFC3339Nano", input: rfc3339Nano, want: rfc3339NanoTime},
		{name: "invalid", input: "2026/08/04 12:00", wantError: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseOOSIAMAssumeRoleExpiration(tc.input)
			if gotError := err != nil; gotError != tc.wantError {
				t.Fatalf("error = %v, want error = %t", err, tc.wantError)
			}
			if !got.Equal(tc.want) {
				t.Fatalf("expiration = %s, want %s", got, tc.want)
			}
		})
	}
}
