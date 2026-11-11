// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package sdkv2

import (
	"context"
	"errors"
	"io"
	"maps"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	awscloudtrail "github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	awsiam "github.com/aws/aws-sdk-go-v2/service/iam"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	smithymiddleware "github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/sdkdiag"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/provider/sdkv2/internal/attribute"
	ooscloudtrail "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/cloudtrail"
	oosiam "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/iam"
	ooss3 "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/s3"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/version"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/aws-sdk-go-base/v2/servicemocks"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	terraformsdk "github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// TC-OOS-PROVIDER-01 Static Credentials Minimal Endpoint configures the provider with paired static credentials and a valid endpoint.
// Expected: provider initialization succeeds without region and can create an S3 client.
func TestUnitOOSProviderConfig_ValidConfiguration(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	p, err := NewProvider(ctx)
	if err != nil {
		t.Fatal(err)
	}
	p.TerraformVersion = "1.0.0"

	diags := p.Configure(ctx, terraformsdk.NewResourceConfigRaw(oosBaseConfig()))
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	meta := p.Meta().(*conns.AWSClient)

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("S3Client() panicked: %v", r)
			}
		}()
		_ = meta.S3Client(ctx)
	}()
}

// TC-OOS-PROVIDER-08/09 Endpoint Contract And Invalid Endpoint Template verifies the OOS endpoint configuration contract.
// Expected: any single-service endpoint can initialize; all endpoints missing or an endpoint that does not match its template returns an explicit diagnostic.
func TestUnitOOSProviderConfig_EndpointContract(t *testing.T) {
	// Isolate endpoint environment variables so this test verifies only the endpoints block configuration contract.
	t.Setenv("CTYUNOOS_ENDPOINT", "")
	t.Setenv("CTYUNOOS_ENDPOINT_IAM", "")
	t.Setenv("CTYUNOOS_ENDPOINT_TRAIL", "")

	ctx := t.Context()

	testCases := map[string]struct {
		mutate             func(map[string]any)
		expectError        bool
		expectedService    string
		expectedDiagnostic string
	}{
		"valid endpoints only": {
			mutate:      func(map[string]any) {},
			expectError: false,
		},
		// The provider requires only one endpoint; an unconfigured service fails only when it is requested.
		"missing s3 endpoint": {
			mutate: func(c map[string]any) {
				c["endpoints"].([]any)[0].(map[string]any)["s3"] = ""
			},
			expectError: false,
		},
		"missing iam endpoint": {
			mutate: func(c map[string]any) {
				c["endpoints"].([]any)[0].(map[string]any)["iam"] = ""
			},
			expectError: false,
		},
		"missing cloudtrail endpoint": {
			mutate: func(c map[string]any) {
				c["endpoints"].([]any)[0].(map[string]any)["cloudtrail"] = ""
			},
			expectError: false,
		},
		"only S3 endpoint": {
			mutate: func(c map[string]any) {
				endpoints := c["endpoints"].([]any)[0].(map[string]any)
				endpoints["iam"] = ""
				endpoints["cloudtrail"] = ""
			},
			expectError: false,
		},
		"only IAM endpoint": {
			mutate: func(c map[string]any) {
				endpoints := c["endpoints"].([]any)[0].(map[string]any)
				endpoints["s3"] = ""
				endpoints["cloudtrail"] = ""
			},
			expectError: false,
		},
		"only CloudTrail endpoint": {
			mutate: func(c map[string]any) {
				endpoints := c["endpoints"].([]any)[0].(map[string]any)
				endpoints["s3"] = ""
				endpoints["iam"] = ""
			},
			expectError: false,
		},
		// Omitting all three endpoints violates the provider contract requiring at least one endpoint.
		"missing all endpoints": {
			mutate: func(c map[string]any) {
				ep := c["endpoints"].([]any)[0].(map[string]any)
				ep["s3"] = ""
				ep["iam"] = ""
				ep["cloudtrail"] = ""
			},
			expectError:        true,
			expectedDiagnostic: "Missing required provider configuration",
		},
		"s3 endpoint not matching template": {
			mutate: func(c map[string]any) {
				c["endpoints"].([]any)[0].(map[string]any)["s3"] = "https://my-private-oos.example.com"
			},
			expectError:        true,
			expectedService:    "The s3 endpoint",
			expectedDiagnostic: "does not match the expected OOS endpoint format",
		},
		"s3 endpoint missing scheme": {
			mutate: func(c map[string]any) {
				c["endpoints"].([]any)[0].(map[string]any)["s3"] = "oos-cn1.ctyunapi.cn"
			},
			expectError:        true,
			expectedDiagnostic: "must be an absolute URL with an http or https scheme",
		},
		"s3 endpoint unsupported scheme": {
			mutate: func(c map[string]any) {
				c["endpoints"].([]any)[0].(map[string]any)["s3"] = "ftp://oos-cn1.ctyunapi.cn"
			},
			expectError:        true,
			expectedDiagnostic: "must be an absolute URL with an http or https scheme",
		},
		"s3 endpoint missing host": {
			mutate: func(c map[string]any) {
				c["endpoints"].([]any)[0].(map[string]any)["s3"] = "https:///oos-cn1.ctyunapi.cn"
			},
			expectError:        true,
			expectedDiagnostic: "must be an absolute URL with an http or https scheme",
		},
		"s3 endpoint malformed URL": {
			mutate: func(c map[string]any) {
				c["endpoints"].([]any)[0].(map[string]any)["s3"] = "https://%gh"
			},
			expectError:        true,
			expectedDiagnostic: "must be an absolute URL with an http or https scheme",
		},
		"s3 http endpoint accepted": {
			mutate: func(c map[string]any) {
				c["endpoints"].([]any)[0].(map[string]any)["s3"] = "http://oos-cn1.ctyunapi.cn"
			},
			expectError: false,
		},
		"IAM endpoint not matching template": {
			mutate: func(c map[string]any) {
				c["endpoints"].([]any)[0].(map[string]any)["iam"] = "https://my-private-oos.example.com"
			},
			expectError:        true,
			expectedService:    "The iam endpoint",
			expectedDiagnostic: "does not match the expected OOS endpoint format",
		},
		"CloudTrail endpoint not matching template": {
			mutate: func(c map[string]any) {
				c["endpoints"].([]any)[0].(map[string]any)["cloudtrail"] = "https://my-private-oos.example.com"
			},
			expectError:        true,
			expectedService:    "The cloudtrail endpoint",
			expectedDiagnostic: "does not match the expected OOS endpoint format",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			config := oosBaseConfig()
			tc.mutate(config)

			p, err := NewProvider(ctx)
			if err != nil {
				t.Fatal(err)
			}
			p.TerraformVersion = "1.0.0"

			diags := p.Configure(ctx, terraformsdk.NewResourceConfigRaw(config))
			if got := diags.HasError(); got != tc.expectError {
				t.Fatalf("expected error=%t, got error=%t (diags: %v)", tc.expectError, got, diags)
			}
			if tc.expectedService != "" || tc.expectedDiagnostic != "" {
				var diagnosticText strings.Builder
				for _, diagnostic := range diags {
					diagnosticText.WriteString(diagnostic.Summary)
					diagnosticText.WriteString(diagnostic.Detail)
				}
				for _, expected := range []string{tc.expectedService, tc.expectedDiagnostic} {
					if expected != "" && !strings.Contains(diagnosticText.String(), expected) {
						t.Fatalf("expected diagnostic containing %q, got: %v", expected, diags)
					}
				}
			}
		})
	}
}

// TC-OOS-PROVIDER-09 Invalid Endpoint Template verifies that OOS endpoint environment variables must also include a URL scheme.
func TestUnitOOSProviderConfig_EndpointSchemeValidationEnv(t *testing.T) {
	t.Setenv("CTYUNOOS_ENDPOINT", "")
	t.Setenv("CTYUNOOS_ENDPOINT_IAM", "")
	t.Setenv("CTYUNOOS_ENDPOINT_TRAIL", "")

	testCases := map[string]string{
		"CTYUNOOS_ENDPOINT":       "oos-cn1.ctyunapi.cn",
		"CTYUNOOS_ENDPOINT_IAM":   "oos-cn1-iam.ctyunapi.cn",
		"CTYUNOOS_ENDPOINT_TRAIL": "oos-cn1-cloudtrail.ctyunapi.cn",
	}

	for envName, endpoint := range testCases {
		t.Run(envName, func(t *testing.T) {
			t.Setenv(envName, endpoint)

			config := oosBaseConfig()
			endpoints := config["endpoints"].([]any)[0].(map[string]any)
			endpoints["s3"] = ""
			endpoints["iam"] = ""
			endpoints["cloudtrail"] = ""

			p, err := NewProvider(t.Context())
			if err != nil {
				t.Fatal(err)
			}

			diags := p.Configure(t.Context(), terraformsdk.NewResourceConfigRaw(config))
			if !diags.HasError() {
				t.Fatal("expected an invalid endpoint diagnostic")
			}

			var diagnosticText strings.Builder
			for _, diagnostic := range diags {
				diagnosticText.WriteString(diagnostic.Summary)
				diagnosticText.WriteString(diagnostic.Detail)
			}
			if !strings.Contains(diagnosticText.String(), "must be an absolute URL with an http or https scheme") {
				t.Fatalf("expected scheme diagnostic, got: %s", diagnosticText.String())
			}
		})
	}
}

// TC-OOS-PROVIDER-03 Credential Env Fallback verifies that OOS custom credential environment variables populate unset credential fields.
func TestUnitOOSProviderConfig_CredentialsEnvFallback(t *testing.T) {
	testcases := map[string]struct {
		config         conns.Config
		envvars        map[string]string
		expectedAccess string
		expectedSecret string
		expectedToken  string
	}{
		"envvars fill empty config": {
			config: conns.Config{},
			envvars: map[string]string{
				"CTYUNOOS_ACCESS_KEY":    "ak-env",
				"CTYUNOOS_SECRET_KEY":    "sk-env",
				"CTYUNOOS_SESSION_TOKEN": "token-env",
			},
			expectedAccess: "ak-env",
			expectedSecret: "sk-env",
			expectedToken:  "token-env",
		},
		"all empty stays empty": {
			config:         conns.Config{},
			envvars:        map[string]string{},
			expectedAccess: "",
			expectedSecret: "",
			expectedToken:  "",
		},
		"configured long-lived keys do not inherit environment token": {
			config: conns.Config{
				AccessKey: "ak-config",
				SecretKey: "sk-config",
			},
			envvars: map[string]string{
				"CTYUNOOS_ACCESS_KEY":    "ak-env",
				"CTYUNOOS_SECRET_KEY":    "sk-env",
				"CTYUNOOS_SESSION_TOKEN": "token-env",
			},
			expectedAccess: "ak-config",
			expectedSecret: "sk-config",
			expectedToken:  "",
		},
		"partial configured credentials do not combine with environment": {
			config: conns.Config{
				AccessKey: "ak-config",
			},
			envvars: map[string]string{
				"CTYUNOOS_ACCESS_KEY":    "ak-env",
				"CTYUNOOS_SECRET_KEY":    "sk-env",
				"CTYUNOOS_SESSION_TOKEN": "token-env",
			},
			expectedAccess: "ak-config",
			expectedSecret: "",
			expectedToken:  "",
		},
	}

	for name, testcase := range testcases {
		t.Run(name, func(t *testing.T) {
			oldEnv := stashEnv()
			defer popEnv(oldEnv)

			for k, v := range testcase.envvars {
				os.Setenv(k, v) //nolint:usetesting // stashEnv & popEnv require os.Setenv
			}

			config := testcase.config
			expandCredentialsCustomEnvVars(&config)

			if config.AccessKey != testcase.expectedAccess {
				t.Errorf("AccessKey: expected %q, got %q", testcase.expectedAccess, config.AccessKey)
			}
			if config.SecretKey != testcase.expectedSecret {
				t.Errorf("SecretKey: expected %q, got %q", testcase.expectedSecret, config.SecretKey)
			}
			if config.Token != testcase.expectedToken {
				t.Errorf("Token: expected %q, got %q", testcase.expectedToken, config.Token)
			}
		})
	}
}

// TC-OOS-PROVIDER-04 Credential Precedence verifies that explicit AK/SK/token values are not overridden by environment variables.
func TestUnitOOSProviderConfig_CredentialsConfigPrecedence(t *testing.T) {
	t.Setenv("CTYUNOOS_ACCESS_KEY", "ak-env")
	t.Setenv("CTYUNOOS_SECRET_KEY", "sk-env")
	t.Setenv("CTYUNOOS_SESSION_TOKEN", "token-env")

	config := conns.Config{
		AccessKey: "ak-config",
		SecretKey: "sk-config",
		Token:     "token-config",
	}
	expandCredentialsCustomEnvVars(&config)

	if got, want := config.AccessKey, "ak-config"; got != want {
		t.Errorf("AccessKey: got %q, want %q", got, want)
	}
	if got, want := config.SecretKey, "sk-config"; got != want {
		t.Errorf("SecretKey: got %q, want %q", got, want)
	}
	if got, want := config.Token, "token-config"; got != want {
		t.Errorf("Token: got %q, want %q", got, want)
	}
}

func stashEnv() []string {
	env := os.Environ()
	os.Clearenv()
	return env
}

func popEnv(env []string) {
	os.Clearenv()

	for _, e := range env {
		k, v, _ := strings.Cut(e, "=")
		os.Setenv(k, v)
	}
}

// TC-OOS-PROVIDER-01 Static Credentials Minimal Endpoint verifies that OOS does not fall back to the AWS default credential chain.
// Expected: without complete CTYUNOOS static credentials, the provider returns Missing OOS Credentials.
func TestUnitOOSProviderConfig_AuthenticationRequiresStaticCredentials(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "aws-access-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "aws-secret-key")
	t.Setenv("CTYUNOOS_ACCESS_KEY", "")
	t.Setenv("CTYUNOOS_SECRET_KEY", "")
	t.Setenv("CTYUNOOS_SESSION_TOKEN", "")

	testCases := map[string]map[string]any{
		"AWS default chain only": {},
		"access key only":        {"access_key": "oos-access-key"},
		"secret key only":        {"secret_key": "oos-secret-key"},
	}

	for name, config := range testCases {
		t.Run(name, func(t *testing.T) {
			config["endpoints"] = []any{map[string]any{"s3": "https://oos-cn1.ctyunapi.cn"}}
			rc := terraformsdk.NewResourceConfigRaw(config)

			p, err := NewProvider(t.Context())
			if err != nil {
				t.Fatal(err)
			}

			diags := p.Configure(t.Context(), rc)
			if !diags.HasError() {
				t.Fatal("expected static OOS credential validation error")
			}
			if got := sdkdiag.DiagnosticsString(diags); !strings.Contains(got, "Missing OOS Credentials") {
				t.Fatalf("expected Missing OOS Credentials diagnostic, got: %s", got)
			}
		})
	}
}

// TC-OOS-PROVIDER-38 Multiple Endpoint Blocks verifies compatibility behavior for multiple endpoints blocks.
// Expected: multiple blocks produce only a warning; endpoints for distinct services merge, while duplicate services retain the first non-empty input value.
func TestUnitOOSProviderConfig_MultipleEndpointBlocks(t *testing.T) {
	// Isolate endpoint environment variables so only endpoints block expansion is observed.
	t.Setenv("CTYUNOOS_ENDPOINT", "")
	t.Setenv("CTYUNOOS_ENDPOINT_IAM", "")
	t.Setenv("CTYUNOOS_ENDPOINT_TRAIL", "")

	ctx := t.Context()

	t.Run("different services merge through provider configuration", func(t *testing.T) {
		p, err := NewProvider(ctx)
		if err != nil {
			t.Fatal(err)
		}

		config := terraformsdk.NewResourceConfigRaw(map[string]any{
			"access_key": "oos-access-key",
			"secret_key": "oos-secret-key",
			"endpoints": []any{
				map[string]any{"s3": "https://oos-cn1.ctyunapi.cn"},
				map[string]any{"iam": "https://oos-cn1-iam.ctyunapi.cn"},
			},
		})

		diags := p.Configure(ctx, config)
		if diags.HasError() {
			t.Fatalf("unexpected provider configuration diagnostics: %v", diags)
		}
		if got, want := len(diags), 1; got != want {
			t.Fatalf("diagnostic count = %d, want %d: %v", got, want, diags)
		}
		if got, want := diags[0].Severity, diag.Warning; got != want {
			t.Errorf("diagnostic severity = %v, want %v", got, want)
		}
		if got := diags[0].Detail; !strings.Contains(got, "should have at most 1 element, got 2") {
			t.Errorf("warning detail = %q, want multiple endpoints block warning", got)
		}

		client, ok := p.Meta().(*conns.AWSClient)
		if !ok || client == nil {
			t.Fatal("provider metadata does not contain an AWS client")
		}
		if client.S3Client(ctx) == nil {
			t.Error("S3 client is nil after endpoints block merge")
		}
		if client.IAMClient(ctx) == nil {
			t.Error("IAM client is nil after endpoints block merge")
		}
	})

	t.Run("duplicate service keeps first non-empty endpoint", func(t *testing.T) {
		endpoints, diags := expandEndpoints(ctx, []any{
			map[string]any{"s3": ""},
			map[string]any{"s3": "https://oos-cn1.ctyunapi.cn"},
			map[string]any{
				"s3":  "https://oos-cn2.ctyunapi.cn",
				"iam": "https://oos-cn1-iam.ctyunapi.cn",
			},
		})
		if diags.HasError() {
			t.Fatalf("unexpected endpoint expansion diagnostics: %v", diags)
		}
		if got, want := len(diags), 1; got != want {
			t.Fatalf("diagnostic count = %d, want %d: %v", got, want, diags)
		}
		if got, want := diags[0].Severity, diag.Warning; got != want {
			t.Errorf("diagnostic severity = %v, want %v", got, want)
		}

		wantEndpoints := map[string]string{
			"s3":  "https://oos-cn1.ctyunapi.cn",
			"iam": "https://oos-cn1-iam.ctyunapi.cn",
		}
		if !maps.Equal(endpoints, wantEndpoints) {
			t.Errorf("endpoints = %#v, want %#v", endpoints, wantEndpoints)
		}
	})
}

// TC-OOS-PROVIDER-02 Temporary Credentials verifies that security_token is passed into the SDK static-credentials object.
func TestUnitOOSProviderConfig_SecurityTokenPropagation(t *testing.T) {
	p, err := NewProvider(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	config := terraformsdk.NewResourceConfigRaw(map[string]any{
		"access_key":     "oos-access-key",
		"secret_key":     "oos-secret-key",
		"security_token": "oos-session-token",
		"endpoints": []any{
			map[string]any{"iam": "https://oos-cn1-iam.ctyunapi.cn"},
		},
	})

	diags := p.Configure(t.Context(), config)
	if diags.HasError() {
		t.Fatalf("unexpected provider configuration diagnostics: %v", diags)
	}

	client, ok := p.Meta().(*conns.AWSClient)
	if !ok || client == nil {
		t.Fatal("provider metadata does not contain an AWS client")
	}

	credentials, err := client.CredentialsProvider(t.Context()).Retrieve(t.Context())
	if err != nil {
		t.Fatalf("retrieving configured credentials: %s", err)
	}

	if got, want := credentials.AccessKeyID, "oos-access-key"; got != want {
		t.Errorf("access key ID = %q, want %q", got, want)
	}
	if got, want := credentials.SecretAccessKey, "oos-secret-key"; got != want {
		t.Errorf("secret access key = %q, want %q", got, want)
	}
	if got, want := credentials.SessionToken, "oos-session-token"; got != want {
		t.Errorf("session token = %q, want %q", got, want)
	}
}

// TC-OOS-PROVIDER-40 Max Retries Propagation verifies propagation of max_retries to the AWS SDK.
// Expected: omission and explicit zero use the provider default of 5; positive values are passed through as configured.
func TestUnitOOSProviderConfig_MaxRetriesPropagation(t *testing.T) {
	testCases := map[string]struct {
		configuredValue  any
		expectedAttempts int
	}{
		"omitted uses provider default": {
			expectedAttempts: 5,
		},
		"one attempt": {
			configuredValue:  1,
			expectedAttempts: 1,
		},
		"three attempts": {
			configuredValue:  3,
			expectedAttempts: 3,
		},
		"explicit zero keeps provider default": {
			configuredValue:  0,
			expectedAttempts: 5,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			config := map[string]any{
				"access_key": "oos-access-key",
				"secret_key": "oos-secret-key",
				"endpoints": []any{
					map[string]any{"s3": "https://oos-cn1.ctyunapi.cn"},
				},
			}
			if tc.configuredValue != nil {
				config["max_retries"] = tc.configuredValue
			}

			p, err := NewProvider(t.Context())
			if err != nil {
				t.Fatal(err)
			}

			diags := p.Configure(t.Context(), terraformsdk.NewResourceConfigRaw(config))
			if diags.HasError() {
				t.Fatalf("unexpected provider configuration diagnostics: %v", diags)
			}

			client, ok := p.Meta().(*conns.AWSClient)
			if !ok || client == nil {
				t.Fatal("provider metadata does not contain an AWS client")
			}

			if got := client.AwsConfig(t.Context()).RetryMaxAttempts; got != tc.expectedAttempts {
				t.Errorf("RetryMaxAttempts = %d, want %d", got, tc.expectedAttempts)
			}
		})
	}
}

// TC-OOS-PROVIDER-41 Retry Mode Propagation verifies retry_mode configuration, environment variables, and precedence.
// Expected: standard and adaptive are passed to the SDK, invalid values fail during configuration, and explicit configuration overrides AWS_RETRY_MODE.
func TestUnitOOSProviderConfig_RetryModePropagation(t *testing.T) {
	testCases := map[string]struct {
		configuredMode  string
		environmentMode string
		expectedMode    aws.RetryMode
		expectedError   string
	}{
		"explicit standard": {
			configuredMode: "standard",
			expectedMode:   aws.RetryModeStandard,
		},
		"explicit adaptive": {
			configuredMode: "adaptive",
			expectedMode:   aws.RetryModeAdaptive,
		},
		"environment fallback": {
			environmentMode: "adaptive",
			expectedMode:    aws.RetryModeAdaptive,
		},
		"explicit configuration overrides environment": {
			configuredMode:  "standard",
			environmentMode: "adaptive",
			expectedMode:    aws.RetryModeStandard,
		},
		"invalid mode": {
			configuredMode: "invalid",
			expectedError:  "unknown RetryMode, invalid",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Setenv("AWS_RETRY_MODE", tc.environmentMode)

			config := map[string]any{
				"access_key": "oos-access-key",
				"secret_key": "oos-secret-key",
				"endpoints": []any{
					map[string]any{"s3": "https://oos-cn1.ctyunapi.cn"},
				},
			}
			if tc.configuredMode != "" {
				config["retry_mode"] = tc.configuredMode
			}

			p, err := NewProvider(t.Context())
			if err != nil {
				t.Fatal(err)
			}

			diags := p.Configure(t.Context(), terraformsdk.NewResourceConfigRaw(config))
			if tc.expectedError != "" {
				if !diags.HasError() {
					t.Fatalf("expected provider configuration error containing %q, got: %v", tc.expectedError, diags)
				}
				if got := sdkdiag.DiagnosticsString(diags); !strings.Contains(got, tc.expectedError) {
					t.Errorf("diagnostics = %q, want substring %q", got, tc.expectedError)
				}
				return
			}
			if diags.HasError() {
				t.Fatalf("unexpected provider configuration diagnostics: %v", diags)
			}

			client, ok := p.Meta().(*conns.AWSClient)
			if !ok || client == nil {
				t.Fatal("provider metadata does not contain an AWS client")
			}

			awsConfig := client.AwsConfig(t.Context())
			if got := awsConfig.RetryMode; got != tc.expectedMode {
				t.Errorf("RetryMode = %q, want %q", got, tc.expectedMode)
			}
			if retryer := awsConfig.Retryer(); retryer == nil {
				t.Error("Retryer() returned nil")
			}
		})
	}
}

// TC-OOS-PROVIDER-42 Token Bucket Rate Limiter Capacity verifies propagation of retry token-bucket capacity.
// Expected: unset or zero disables rate limiting; capacity 100 permits 20 failed retries under both standard and adaptive modes, and the 21st fails.
func TestUnitOOSProviderConfig_TokenBucketRateLimiterCapacity(t *testing.T) {
	testCases := map[string]struct {
		capacity        int
		setCapacity     bool
		retryMode       string
		expectedMode    aws.RetryMode
		expectExhausted bool
	}{
		"standard omitted capacity disables limiter": {
			retryMode:    "standard",
			expectedMode: aws.RetryModeStandard,
		},
		"standard zero capacity disables limiter": {
			capacity:     0,
			setCapacity:  true,
			retryMode:    "standard",
			expectedMode: aws.RetryModeStandard,
		},
		"standard capacity one hundred limits retries": {
			capacity:        100,
			setCapacity:     true,
			retryMode:       "standard",
			expectedMode:    aws.RetryModeStandard,
			expectExhausted: true,
		},
		"adaptive omitted capacity disables limiter": {
			retryMode:    "adaptive",
			expectedMode: aws.RetryModeAdaptive,
		},
		"adaptive zero capacity disables limiter": {
			capacity:     0,
			setCapacity:  true,
			retryMode:    "adaptive",
			expectedMode: aws.RetryModeAdaptive,
		},
		"adaptive capacity one hundred limits retries": {
			capacity:        100,
			setCapacity:     true,
			retryMode:       "adaptive",
			expectedMode:    aws.RetryModeAdaptive,
			expectExhausted: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			config := map[string]any{
				"access_key": "oos-access-key",
				"secret_key": "oos-secret-key",
				"retry_mode": tc.retryMode,
				"endpoints": []any{
					map[string]any{"s3": "https://oos-cn1.ctyunapi.cn"},
				},
			}
			if tc.setCapacity {
				config["token_bucket_rate_limiter_capacity"] = tc.capacity
			}

			p, err := NewProvider(t.Context())
			if err != nil {
				t.Fatal(err)
			}

			diags := p.Configure(t.Context(), terraformsdk.NewResourceConfigRaw(config))
			if diags.HasError() {
				t.Fatalf("unexpected provider configuration diagnostics: %v", diags)
			}

			client, ok := p.Meta().(*conns.AWSClient)
			if !ok || client == nil {
				t.Fatal("provider metadata does not contain an AWS client")
			}
			awsConfig := client.AwsConfig(t.Context())
			if got := awsConfig.RetryMode; got != tc.expectedMode {
				t.Errorf("RetryMode = %q, want %q", got, tc.expectedMode)
			}

			retryer := awsConfig.Retryer()
			if retryer == nil {
				t.Fatal("Retryer() returned nil")
			}
			retryError := &smithy.GenericAPIError{Code: "RequestTimeout", Message: "retry failed"}
			if !retryer.IsErrorRetryable(retryError) {
				t.Fatalf("retryer does not consider %T retryable", retryError)
			}

			// The default RetryCost is 5. Simulating 20 failed retries consumes capacity 100 without releasing tokens.
			for attempt := 1; attempt <= 20; attempt++ {
				release, err := retryer.GetRetryToken(t.Context(), retryError)
				if err != nil {
					t.Fatalf("retry token attempt %d: %s", attempt, err)
				}
				if err := release(retryError); err != nil {
					t.Fatalf("releasing retry token %d: %s", attempt, err)
				}
			}

			_, err = retryer.GetRetryToken(t.Context(), retryError)
			if tc.expectExhausted {
				if err == nil {
					t.Fatal("expected retry quota exhaustion after 20 failed retries")
				}
				if got := err.Error(); !strings.Contains(got, "retry quota exceeded") {
					t.Errorf("retry quota error = %q, want retry quota exceeded", got)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected retry quota error with disabled limiter: %s", err)
			}
		})
	}
}

// TC-OOS-PROVIDER-43 Provider Meta User Agent verifies the final request header for provider- and module-level User-Agent values.
// Expected: an explicit provider value replaces the default identifier, provider_meta is appended, and S3, IAM, and CloudTrail preserve the same order.
func TestUnitOOSProviderConfig_ProviderMetaUserAgent(t *testing.T) {
	const (
		providerUserAgent = "provider-test/1.0"
		moduleUserAgent   = "module-test/2.0"
	)
	defaultProduct := "terraform-provider-ctyunoos/" + version.ProviderVersion

	testCases := map[string]struct {
		providerUserAgent []any
		moduleUserAgent   []string
		expectDefault     bool
		expectProducts    []string
	}{
		"default": {
			expectDefault: true,
		},
		"provider override": {
			providerUserAgent: []any{providerUserAgent},
			expectProducts:    []string{providerUserAgent},
		},
		"provider meta append": {
			moduleUserAgent: []string{moduleUserAgent},
			expectDefault:   true,
			expectProducts:  []string{moduleUserAgent},
		},
		"provider and provider meta": {
			providerUserAgent: []any{providerUserAgent},
			moduleUserAgent:   []string{moduleUserAgent},
			expectProducts:    []string{providerUserAgent, moduleUserAgent},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			servicemocks.InitSessionTestEnv(t)
			t.Setenv("TF_APPEND_USER_AGENT", "")

			ctx := t.Context()
			config := map[string]any{
				"access_key": "StaticAccessKey",
				"secret_key": servicemocks.MockStaticSecretKey,
				"endpoints": []any{
					map[string]any{"s3": "https://oos-cn1.ctyunapi.cn"},
				},
			}
			if len(tc.providerUserAgent) > 0 {
				config["user_agent"] = tc.providerUserAgent
			}

			p, err := NewProvider(ctx)
			if err != nil {
				t.Fatal(err)
			}
			diags := p.Configure(ctx, terraformsdk.NewResourceConfigRaw(config))
			if diags.HasError() {
				t.Fatalf("unexpected provider configuration diagnostics: %v", diags)
			}

			ctx, err = contextWithProviderMeta(ctx, func(raw any) error {
				metadata := raw.(*providerMeta)
				metadata.UserAgent = tc.moduleUserAgent
				return nil
			})
			if err != nil {
				t.Fatalf("adding provider_meta to request context: %s", err)
			}

			client := p.Meta().(*conns.AWSClient)
			for _, serviceName := range []string{names.S3, names.IAM, names.CloudTrail} {
				header := testOOSServiceUserAgentHeader(ctx, t, client, serviceName)
				assertOOSUserAgentProducts(t, serviceName, header, defaultProduct, tc.expectDefault, tc.expectProducts)
			}
		})
	}
}

// TC-OOS-PROVIDER-44 S3 Path Style Propagation verifies that S3 address-style configuration reaches the final request.
// Expected: with explicit true or no setting, the bucket is in the URL path; with explicit false, it is in the URL host.
func TestUnitOOSProviderConfig_S3UsePathStyle(t *testing.T) {
	testCases := map[string]struct {
		configure       func(map[string]any)
		expectPathStyle bool
	}{
		"default": {
			expectPathStyle: true,
		},
		"explicit true": {
			configure: func(config map[string]any) {
				config["s3_use_path_style"] = true
			},
			expectPathStyle: true,
		},
		"explicit false": {
			configure: func(config map[string]any) {
				config["s3_use_path_style"] = false
			},
			expectPathStyle: false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			servicemocks.InitSessionTestEnv(t)

			config := oosBaseConfig()
			if tc.configure != nil {
				tc.configure(config)
			}

			p, err := NewProvider(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if diags := p.Configure(t.Context(), terraformsdk.NewResourceConfigRaw(config)); diags.HasError() {
				t.Fatalf("configuring provider: %s", sdkdiag.DiagnosticsString(diags))
			}

			client := p.Meta().(*conns.AWSClient)
			transport := &userAgentCaptureRoundTripper{}
			awsConfig := client.AwsConfig(t.Context())
			awsConfig.HTTPClient = &http.Client{Transport: transport}
			clientConfig := map[string]any{
				"aws_sdkv2_config":  &awsConfig,
				names.AttrEndpoint:  "https://oos-cn1.ctyunapi.cn",
				names.AttrRegion:    awsConfig.Region,
				"s3_use_path_style": client.S3UsePathStyle(t.Context()),
				"original_region":   "cn1",
			}
			factory, ok := ooss3.ServicePackage(t.Context()).(interface {
				NewClient(context.Context, map[string]any) (*awss3.Client, error)
			})
			if !ok {
				t.Fatal("S3 service package does not expose an SDK client factory")
			}
			s3Client, err := factory.NewClient(t.Context(), clientConfig)
			if err != nil {
				t.Fatalf("creating S3 client: %s", err)
			}
			_, _ = s3Client.HeadBucket(t.Context(), &awss3.HeadBucketInput{Bucket: aws.String("path-style-test")})

			if transport.request == nil {
				t.Fatal("S3 client did not send a request")
			}
			requestURL := transport.request.URL
			if tc.expectPathStyle {
				if requestURL.Host == "path-style-test.oos-cn1.ctyunapi.cn" || !strings.Contains(requestURL.Path, "/path-style-test") {
					t.Fatalf("expected path-style S3 URL, got %s", requestURL)
				}
			} else {
				if requestURL.Host != "path-style-test.oos-cn1.ctyunapi.cn" || requestURL.Path == "/path-style-test" {
					t.Fatalf("expected virtual-hosted-style S3 URL, got %s", requestURL)
				}
			}
		})
	}
}

// TC-OOS-PROVIDER-14/15/16 AssumeRole Empty And Multiple Empty Error verifies AssumeRole block compatibility and required-field diagnostics.
// A single empty block retains the historical compatibility warning; empty first or last role_arn values in multiple blocks return the relevant RequiredAttributeError.
func TestUnitOOSProviderConfig_AssumeRole(t *testing.T) {
	const (
		mockOOSAssumeRoleARN  = "arn:ctyun:iam::32fefj64y54gc:role/AssumeRole"
		mockOOSAssumeRoleARN2 = "arn:ctyun:iam::10rc2arpn6306:role/AssumeRole2"
	)

	testCases := map[string]struct {
		Config        map[string]any
		ExpectedDiags diag.Diagnostics
	}{
		// For historical reasons, this is valid
		"config empty": {
			Config: map[string]any{
				"assume_role": []any{
					map[string]any{},
				},
			},
			ExpectedDiags: diag.Diagnostics{
				errs.NewAttributeRequiredWillBeError(cty.GetAttrPath("assume_role").IndexInt(0), "role_arn"),
			},
		},

		// For historical reasons, this is valid
		"config null string": {
			Config: map[string]any{
				"assume_role": []any{
					map[string]any{
						"role_arn": nil,
					},
				},
			},
			ExpectedDiags: diag.Diagnostics{
				errs.NewAttributeRequiredWillBeError(cty.GetAttrPath("assume_role").IndexInt(0), "role_arn"),
			},
		},

		// For historical reasons, this is valid
		"config empty string": {
			Config: map[string]any{
				"assume_role": []any{
					map[string]any{
						"role_arn": "",
					},
				},
			},
			ExpectedDiags: diag.Diagnostics{
				errs.NewAttributeRequiredWillBeError(cty.GetAttrPath("assume_role").IndexInt(0), "role_arn"),
			},
		},

		"config multiple first empty": {
			Config: map[string]any{
				"assume_role": []any{
					map[string]any{},
					map[string]any{
						"role_arn":     mockOOSAssumeRoleARN2,
						"session_name": servicemocks.MockStsAssumeRoleSessionName2,
					},
				},
			},
			ExpectedDiags: diag.Diagnostics{
				errs.NewAttributeRequiredError(cty.GetAttrPath("assume_role").IndexInt(0), "role_arn"),
			},
		},

		"config multiple first null string": {
			Config: map[string]any{
				"assume_role": []any{
					map[string]any{
						"role_arn": nil,
					},
					map[string]any{
						"role_arn":     mockOOSAssumeRoleARN2,
						"session_name": servicemocks.MockStsAssumeRoleSessionName2,
					},
				},
			},
			ExpectedDiags: diag.Diagnostics{
				errs.NewAttributeRequiredError(cty.GetAttrPath("assume_role").IndexInt(0), "role_arn"),
			},
		},

		"config multiple first empty string": {
			Config: map[string]any{
				"assume_role": []any{
					map[string]any{
						"role_arn": "",
					},
					map[string]any{
						"role_arn":     mockOOSAssumeRoleARN2,
						"session_name": servicemocks.MockStsAssumeRoleSessionName2,
					},
				},
			},
			ExpectedDiags: diag.Diagnostics{
				errs.NewAttributeRequiredError(cty.GetAttrPath("assume_role").IndexInt(0), "role_arn"),
			},
		},

		"config multiple last empty": {
			Config: map[string]any{
				"assume_role": []any{
					map[string]any{
						"role_arn":     mockOOSAssumeRoleARN,
						"session_name": servicemocks.MockStsAssumeRoleSessionName,
					},
					map[string]any{},
				},
			},
			ExpectedDiags: diag.Diagnostics{
				errs.NewAttributeRequiredError(cty.GetAttrPath("assume_role").IndexInt(1), "role_arn"),
			},
		},

		"config multiple last null string": {
			Config: map[string]any{
				"assume_role": []any{
					map[string]any{
						"role_arn":     mockOOSAssumeRoleARN,
						"session_name": servicemocks.MockStsAssumeRoleSessionName,
					},
					map[string]any{
						"role_arn": nil,
					},
				},
			},
			ExpectedDiags: diag.Diagnostics{
				errs.NewAttributeRequiredError(cty.GetAttrPath("assume_role").IndexInt(1), "role_arn"),
			},
		},

		"config multiple last empty string": {
			Config: map[string]any{
				"assume_role": []any{
					map[string]any{
						"role_arn":     mockOOSAssumeRoleARN,
						"session_name": servicemocks.MockStsAssumeRoleSessionName,
					},
					map[string]any{
						"role_arn": "",
					},
				},
			},
			ExpectedDiags: diag.Diagnostics{
				errs.NewAttributeRequiredError(cty.GetAttrPath("assume_role").IndexInt(1), "role_arn"),
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctx := t.Context()

			servicemocks.InitSessionTestEnv(t)

			config := map[string]any{
				"access_key": servicemocks.MockStaticAccessKey,
				"secret_key": servicemocks.MockStaticSecretKey,
				"endpoints": []any{
					map[string]any{
						"s3": "https://oos-cn1.ctyunapi.cn",
					},
				},
			}

			maps.Copy(config, tc.Config)

			rc := terraformsdk.NewResourceConfigRaw(config)

			p, err := NewProvider(ctx)
			if err != nil {
				t.Fatal(err)
			}

			p.TerraformVersion = "1.0.0"

			var diags diag.Diagnostics
			diags = append(diags, p.Validate(rc)...)
			if diags.HasError() {
				t.Fatalf("validating: %s", sdkdiag.DiagnosticsString(diags))
			}

			diags = append(diags, p.Configure(ctx, rc)...)

			expectedDiags := tc.ExpectedDiags

			if diff := cmp.Diff(diags, expectedDiags, cmp.Comparer(sdkdiag.Comparer)); diff != "" {
				t.Errorf("unexpected diagnostics difference: %s", diff)
			}
		})
	}
}

// TC-OOS-PROVIDER-46 S3 Resource Region Override verifies that imported state uses the actual region parsed from the endpoint.
// Expected: the dummy region used to initialize the OOS AWS SDK is not written to the imported Terraform state.
func TestUnitOOSProviderConfig_ImportRegionUsesEffectiveRegion(t *testing.T) {
	t.Parallel()

	const (
		importID   = "tf-test-bucket-import"
		realRegion = "shanghai"
	)

	resourceSchema := map[string]*schema.Schema{
		names.AttrBucket: {
			Type:     schema.TypeString,
			Required: true,
		},
		names.AttrRegion: attribute.Region(),
	}
	d := schema.TestResourceDataRaw(t, resourceSchema, nil)
	d.SetId(importID)

	// mockClient panics on AwsConfig(), so this also prevents a regression to
	// the dummy AWS SDK region instead of EffectiveRealRegion.
	opts := importInterceptorOptions{
		c:    mockClient{accountID: "123456789012", region: realRegion},
		d:    d,
		when: Before,
		why:  Import,
	}
	if err := importRegion().run(t.Context(), opts); err != nil {
		t.Fatalf("importing region: %s", err)
	}

	if got := d.Id(); got != importID {
		t.Errorf("id = %q, expected %q", got, importID)
	}
	if got := d.Get(names.AttrRegion).(string); got != realRegion {
		t.Errorf("region = %q, expected %q", got, realRegion)
	}
}

// TC-OOS-PROVIDER-46 S3 Resource Region Override verifies that both plan defaults and refreshed state use the actual region from the endpoint.
func TestUnitOOSProviderConfig_RegionStateUsesEffectiveRegion(t *testing.T) {
	t.Parallel()

	const realRegion = "shanghai"
	resourceSchema := map[string]*schema.Schema{
		names.AttrBucket: {
			Type:     schema.TypeString,
			Required: true,
		},
		names.AttrRegion: attribute.Region(),
	}
	client := mockClient{accountID: "123456789012", region: realRegion}

	t.Run("plan defaults omitted region", func(t *testing.T) {
		resource := &schema.Resource{
			Schema: resourceSchema,
			CustomizeDiff: func(ctx context.Context, d *schema.ResourceDiff, meta any) error {
				return defaultRegion().run(ctx, customizeDiffInterceptorOptions{
					c:    meta.(awsClient),
					d:    d,
					when: Before,
					why:  CustomizeDiff,
				})
			},
		}
		diff, err := resource.Diff(t.Context(), nil, terraformsdk.NewResourceConfigRaw(map[string]any{
			names.AttrBucket: "tf-test-bucket",
		}), client)
		if err != nil {
			t.Fatalf("planning resource: %s", err)
		}
		regionDiff, ok := diff.Attributes[names.AttrRegion]
		if !ok {
			t.Fatalf("planned diff did not contain %q", names.AttrRegion)
		}
		if got := regionDiff.New; got != realRegion {
			t.Errorf("planned region = %q, expected %q", got, realRegion)
		}
	})

	t.Run("refresh existing resource", func(t *testing.T) {
		d := schema.TestResourceDataRaw(t, resourceSchema, map[string]any{
			names.AttrBucket: "tf-test-bucket",
		})
		d.SetId("tf-test-bucket")
		diags := setRegionInState().run(t.Context(), crudInterceptorOptions{
			c:    client,
			d:    d,
			when: After,
			why:  Read,
		})
		if diags.HasError() {
			t.Fatalf("setting region in refresh state: %v", diags)
		}
		if got := d.Get(names.AttrRegion).(string); got != realRegion {
			t.Errorf("refreshed region = %q, expected %q", got, realRegion)
		}
	})

	t.Run("refresh disappeared resource", func(t *testing.T) {
		d := schema.TestResourceDataRaw(t, resourceSchema, map[string]any{
			names.AttrBucket: "tf-test-bucket",
			names.AttrRegion: "configured-region",
		})
		diags := setRegionInState().run(t.Context(), crudInterceptorOptions{
			c:    client,
			d:    d,
			when: After,
			why:  Read,
		})
		if diags.HasError() {
			t.Fatalf("setting region for disappeared resource: %v", diags)
		}
		if got := d.Get(names.AttrRegion).(string); got != "configured-region" {
			t.Errorf("region = %q, expected unchanged value", got)
		}
	})
}

// TC-OOS-PROVIDER-45 Custom CA Bundle And Insecure TLS verifies the CA file, environment-variable precedence, and TLS security switch.
// Expected: a valid PEM is loaded into the HTTP transport, invalid or missing files return diagnostics, and explicit configuration takes precedence over AWS_CA_BUNDLE.
func TestUnitOOSProviderConfig_CustomCABundleAndInsecure(t *testing.T) {
	testCases := map[string]struct {
		configure      func(t *testing.T, config map[string]any)
		configureEnv   func(t *testing.T)
		expectError    string
		expectRootCAs  bool
		expectInsecure bool
	}{
		"valid PEM": {
			configure: func(t *testing.T, config map[string]any) {
				pemFile, err := servicemocks.TempPEMFile()
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Remove(pemFile) })
				config["custom_ca_bundle"] = pemFile
			},
			expectRootCAs: true,
		},
		"invalid PEM": {
			configure: func(t *testing.T, config map[string]any) {
				file, err := os.CreateTemp(t.TempDir(), "invalid-ca-*.pem")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := file.WriteString("not a PEM certificate"); err != nil {
					t.Fatal(err)
				}
				_ = file.Close()
				config["custom_ca_bundle"] = file.Name()
			},
			expectError: "failed to load custom CA bundle PEM file",
		},
		"missing file": {
			configure: func(_ *testing.T, config map[string]any) {
				config["custom_ca_bundle"] = "/path/does/not/exist/oos-ca.pem"
			},
			expectError: "reading custom CA bundle",
		},
		"AWS_CA_BUNDLE environment": {
			configureEnv: func(t *testing.T) {
				pemFile, err := servicemocks.TempPEMFile()
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Remove(pemFile) })
				t.Setenv("AWS_CA_BUNDLE", pemFile)
			},
			expectRootCAs: true,
		},
		"explicit config overrides AWS_CA_BUNDLE": {
			configure: func(t *testing.T, config map[string]any) {
				pemFile, err := servicemocks.TempPEMFile()
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Remove(pemFile) })
				config["custom_ca_bundle"] = pemFile
			},
			configureEnv: func(t *testing.T) {
				t.Setenv("AWS_CA_BUNDLE", "/path/does/not/exist/env-ca.pem")
			},
			expectRootCAs: true,
		},
		"insecure true": {
			configure: func(_ *testing.T, config map[string]any) {
				config["insecure"] = true
			},
			expectInsecure: true,
		},
		"insecure false": {
			expectInsecure: false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			servicemocks.InitSessionTestEnv(t)
			if tc.configureEnv != nil {
				tc.configureEnv(t)
			}

			config := oosBaseConfig()
			if tc.configure != nil {
				tc.configure(t, config)
			}

			p, err := NewProvider(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			diags := p.Configure(t.Context(), terraformsdk.NewResourceConfigRaw(config))
			if tc.expectError != "" {
				if !diags.HasError() {
					t.Fatalf("expected configuration error containing %q, got none", tc.expectError)
				}
				if diagnosticText := sdkdiag.DiagnosticsString(diags); !strings.Contains(diagnosticText, tc.expectError) {
					t.Fatalf("expected diagnostic containing %q, got %s", tc.expectError, diagnosticText)
				}
				return
			}
			if diags.HasError() {
				t.Fatalf("configuring provider: %s", sdkdiag.DiagnosticsString(diags))
			}

			meta := p.Meta().(*conns.AWSClient)
			buildable, ok := meta.AwsConfig(t.Context()).HTTPClient.(*awshttp.BuildableClient)
			if !ok {
				t.Fatalf("expected awshttp.BuildableClient, got %T", meta.AwsConfig(t.Context()).HTTPClient)
			}
			transport := buildable.GetTransport()
			if got := transport.TLSClientConfig.RootCAs != nil; got != tc.expectRootCAs {
				t.Errorf("expected RootCAs configured = %t, got %t", tc.expectRootCAs, got)
			}
			if got := transport.TLSClientConfig.InsecureSkipVerify; got != tc.expectInsecure {
				t.Errorf("expected InsecureSkipVerify = %t, got %t", tc.expectInsecure, got)
			}
		})
	}
}

// TC-OOS-PROVIDER-46 S3 Resource Region Override verifies endpoint and signing overrides for an S3 resource-level region.
// Expected: an unset or matching region retains the original endpoint; a distinct region replaces only the region in the host and signs with the override.
func TestUnitOOSProviderConfig_S3ResourceRegionOverride(t *testing.T) {
	testCases := map[string]struct {
		overrideRegion string
		expectEndpoint string
		expectSigning  string
	}{
		"endpoint region default": {
			expectEndpoint: "https://oos-cn1.ctyunapi.cn/path-style-test",
			expectSigning:  "cn1",
		},
		"same region override": {
			overrideRegion: "cn1",
			expectEndpoint: "https://oos-cn1.ctyunapi.cn/path-style-test",
			expectSigning:  "cn1",
		},
		"different region override": {
			overrideRegion: "cn4",
			expectEndpoint: "https://oos-cn4.ctyunapi.cn/path-style-test",
			expectSigning:  "cn4",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			servicemocks.InitSessionTestEnv(t)
			ctx := t.Context()
			p, err := NewProvider(ctx)
			if err != nil {
				t.Fatal(err)
			}
			config := oosBaseConfig()
			if diags := p.Configure(ctx, terraformsdk.NewResourceConfigRaw(config)); diags.HasError() {
				t.Fatalf("configuring provider: %s", sdkdiag.DiagnosticsString(diags))
			}
			if tc.overrideRegion != "" {
				ctx = conns.NewResourceContext(ctx, names.S3, "", "", tc.overrideRegion)
			} else {
				ctx = conns.NewResourceContext(ctx, names.S3, "", "", "")
			}

			client := p.Meta().(*conns.AWSClient).S3Client(ctx)
			var endpoint, signingRegion string
			_, err = client.HeadBucket(ctx, &awss3.HeadBucketInput{Bucket: aws.String("path-style-test")}, func(options *awss3.Options) {
				options.APIOptions = append(options.APIOptions, func(stack *smithymiddleware.Stack) error {
					return stack.Finalize.Add(smithymiddleware.FinalizeMiddlewareFunc("TestCaptureS3EndpointAndSigningRegion", func(ctx context.Context, input smithymiddleware.FinalizeInput, next smithymiddleware.FinalizeHandler) (smithymiddleware.FinalizeOutput, smithymiddleware.Metadata, error) {
						request, ok := input.Request.(*smithyhttp.Request)
						if !ok {
							t.Fatalf("expected smithy HTTP request, got %T", input.Request)
						}
						endpoint = request.URL.String()
						signingRegion = awsmiddleware.GetSigningRegion(ctx) //nolint:staticcheck // SA1019: test captures the signing region from context; the replacement requires signer-property plumbing
						return smithymiddleware.FinalizeOutput{}, smithymiddleware.Metadata{}, errors.New("cancel test request")
					}), smithymiddleware.After)
				})
			})
			if err == nil || !strings.Contains(err.Error(), "cancel test request") {
				t.Fatalf("expected canceled request, got %v", err)
			}
			if endpoint != tc.expectEndpoint {
				t.Errorf("endpoint = %q, want %q", endpoint, tc.expectEndpoint)
			}
			if signingRegion != tc.expectSigning {
				t.Errorf("signing region = %q, want %q", signingRegion, tc.expectSigning)
			}
		})
	}
}

// oosEndpoints returns provider endpoint configuration containing valid template endpoints for the three OOS services.
func oosEndpoints() []any {
	return []any{
		map[string]any{
			"s3":         "https://oos-cn1.ctyunapi.cn",
			"iam":        "https://oos-cn1-iam.ctyunapi.cn",
			"cloudtrail": "https://oos-cn1-cloudtrail.ctyunapi.cn",
		},
	}
}

func oosBaseConfig() map[string]any {
	return map[string]any{
		"access_key": "StaticAccessKey",
		"secret_key": servicemocks.MockStaticSecretKey,
		"endpoints":  oosEndpoints(),
	}
}

type userAgentCaptureRoundTripper struct {
	request *http.Request
}

func (t *userAgentCaptureRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	t.request = request.Clone(request.Context())

	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("")),
		Request:    request,
	}, nil
}

func testOOSServiceUserAgentHeader(ctx context.Context, t *testing.T, client *conns.AWSClient, serviceName string) string {
	t.Helper()

	transport := &userAgentCaptureRoundTripper{}
	awsConfig := client.AwsConfig(ctx)
	awsConfig.HTTPClient = &http.Client{Transport: transport}
	clientConfig := map[string]any{
		"aws_sdkv2_config":  &awsConfig,
		names.AttrEndpoint:  "https://oos-cn1.ctyunapi.cn",
		names.AttrRegion:    awsConfig.Region,
		"s3_use_path_style": true,
		"original_region":   "cn1",
	}

	switch serviceName {
	case names.S3:
		factory, ok := ooss3.ServicePackage(ctx).(interface {
			NewClient(context.Context, map[string]any) (*awss3.Client, error)
		})
		if !ok {
			t.Fatal("S3 service package does not expose an SDK client factory")
		}
		s3Client, err := factory.NewClient(ctx, clientConfig)
		if err != nil {
			t.Fatalf("creating S3 client: %s", err)
		}
		_, _ = s3Client.ListBuckets(ctx, &awss3.ListBucketsInput{})
	case names.IAM:
		clientConfig[names.AttrEndpoint] = "https://oos-cn1-iam.ctyunapi.cn"
		factory, ok := oosiam.ServicePackage(ctx).(interface {
			NewClient(context.Context, map[string]any) (*awsiam.Client, error)
		})
		if !ok {
			t.Fatal("IAM service package does not expose an SDK client factory")
		}
		iamClient, err := factory.NewClient(ctx, clientConfig)
		if err != nil {
			t.Fatalf("creating IAM client: %s", err)
		}
		_, _ = iamClient.GetAccountSummary(ctx, &awsiam.GetAccountSummaryInput{})
	case names.CloudTrail:
		clientConfig[names.AttrEndpoint] = "https://oos-cn1-cloudtrail.ctyunapi.cn"
		factory, ok := ooscloudtrail.ServicePackage(ctx).(interface {
			NewClient(context.Context, map[string]any) (*awscloudtrail.Client, error)
		})
		if !ok {
			t.Fatal("CloudTrail service package does not expose an SDK client factory")
		}
		cloudTrailClient, err := factory.NewClient(ctx, clientConfig)
		if err != nil {
			t.Fatalf("creating CloudTrail client: %s", err)
		}
		_, _ = cloudTrailClient.DescribeTrails(ctx, &awscloudtrail.DescribeTrailsInput{})
	default:
		t.Fatalf("unsupported service %q", serviceName)
	}

	if transport.request == nil {
		t.Fatalf("%s client did not send a request", serviceName)
	}
	return transport.request.Header.Get("User-Agent")
}

func assertOOSUserAgentProducts(t *testing.T, serviceName, header, defaultProduct string, expectDefault bool, products []string) {
	t.Helper()

	if got := strings.Contains(header, defaultProduct); got != expectDefault {
		t.Errorf("%s User-Agent %q contains default product = %t, want %t", serviceName, header, got, expectDefault)
	}

	lastIndex := -1
	for _, product := range products {
		index := strings.Index(header, product)
		if index == -1 {
			t.Errorf("%s User-Agent %q does not contain %q", serviceName, header, product)
			continue
		}
		if index <= lastIndex {
			t.Errorf("%s User-Agent %q does not preserve product order %q", serviceName, header, products)
		}
		lastIndex = index
	}
}
