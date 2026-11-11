// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package sdkv2_test

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	awstypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/envvar"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TC-OOS-PROVIDER-01 Static credentials with a minimal endpoint: provider HCL explicitly configures primary-account credentials and the IAM endpoint.
// Expected: the GetAccountSummary call succeeds.
func TestAccOOSProvider_staticCredentials(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)
	accessKey := os.Getenv("CTYUNOOS_ACCESS_KEY")
	secretKey := os.Getenv("CTYUNOOS_SECRET_KEY")
	// The static AK/SK test does not use a token, preventing a token left in the shell from being combined with explicit credentials.
	t.Setenv("CTYUNOOS_ACCESS_KEY", "invalid-env-access-key")
	t.Setenv("CTYUNOOS_SECRET_KEY", "invalid-env-secret-key")
	t.Setenv("CTYUNOOS_SESSION_TOKEN", "")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccOOSProviderConfigIAM(
					os.Getenv("CTYUNOOS_ENDPOINT_IAM"),
					accessKey,
					secretKey,
					"",
				),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.ctyun_iam_account_summary.test", names.AttrID, "iam_account_summary"),
				),
			},
		},
	})
}

// TC-OOS-PROVIDER-02 Temporary credentials: uses long-lived environment credentials to create a user and access key, then calls GetSessionToken to create temporary credentials.
// Expected: after configuring the returned AK/SK/security token in the provider, the user can be read and its name is correct.
func TestAccOOSProvider_temporaryCredentials(t *testing.T) {
	ctx := acctest.Context(t)
	// First use long-lived environment AK/SK values to create the test principal, then inject its matching temporary token.
	t.Setenv("CTYUNOOS_SESSION_TOKEN", "")
	acctest.PreCheck(ctx, t)
	fixture := testAccCreateOOSTemporaryCredentialsFixture(ctx, t)

	temporaryAccessKey, temporarySecretKey, temporarySessionToken := fixture.accessKeyID, fixture.secretAccessKey, fixture.sessionToken
	t.Setenv("CTYUNOOS_ACCESS_KEY", temporaryAccessKey)
	t.Setenv("CTYUNOOS_SECRET_KEY", temporarySecretKey)
	t.Setenv("CTYUNOOS_SESSION_TOKEN", temporarySessionToken)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccOOSProviderConfigIAMUser(
					os.Getenv("CTYUNOOS_ENDPOINT_IAM"),
					temporaryAccessKey,
					temporarySecretKey,
					temporarySessionToken,
					fixture.userName,
				),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.ctyun_iam_user.test", names.AttrUserName, fixture.userName),
					resource.TestCheckResourceAttr("data.ctyun_iam_user.test", "user_id", fixture.userID),
				),
			},
			{
				Config: testAccOOSProviderConfigIAMUserEnv(os.Getenv("CTYUNOOS_ENDPOINT_IAM"), fixture.userName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.ctyun_iam_user.test", names.AttrUserName, fixture.userName),
					resource.TestCheckResourceAttr("data.ctyun_iam_user.test", "user_id", fixture.userID),
				),
			},
		},
	})
}

// TC-OOS-PROVIDER-03 Credential environment fallback: provider HCL declares no credentials and uses only the current ACC environment variables.
// Expected: after CTYUNOOS_ACCESS_KEY/SECRET_KEY fallback, reading the account summary succeeds; when SESSION_TOKEN is configured, it falls back as a group with the two credentials.
func TestAccOOSProvider_credentialEnvFallback(t *testing.T) {
	ctx := acctest.Context(t)
	// This case verifies environment fallback for long-lived AK/SK values and avoids signature failures caused by a mismatched shell token.
	t.Setenv("CTYUNOOS_SESSION_TOKEN", "")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccOOSProviderConfigIAM(os.Getenv("CTYUNOOS_ENDPOINT_IAM"), "", "", ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.ctyun_iam_account_summary.test", names.AttrID, "iam_account_summary"),
				),
			},
		},
	})
}

// TC-OOS-PROVIDER-04 Configuration-block precedence: explicit temporary credentials must override credential values from the environment.
// Expected: when the environment token is invalid, explicitly configured temporary credentials can still read the test user.
func TestAccOOSProvider_credentialPrecedence(t *testing.T) {
	ctx := acctest.Context(t)
	// Use long-lived credentials to prepare the fixture so an invalid token does not affect fixture-creation requests.
	t.Setenv("CTYUNOOS_SESSION_TOKEN", "")
	acctest.PreCheck(ctx, t)
	fixture := testAccCreateOOSTemporaryCredentialsFixture(ctx, t)
	// If the provider incorrectly uses either environment credential, all subsequent IAM requests must fail.
	t.Setenv("CTYUNOOS_ACCESS_KEY", "invalid-env-access-key")
	t.Setenv("CTYUNOOS_SECRET_KEY", "invalid-env-secret-key")
	t.Setenv("CTYUNOOS_SESSION_TOKEN", "invalid-token-that-must-not-win")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccOOSProviderConfigIAMUser(
					os.Getenv("CTYUNOOS_ENDPOINT_IAM"),
					fixture.accessKeyID,
					fixture.secretAccessKey,
					fixture.sessionToken,
					fixture.userName,
				),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.ctyun_iam_user.test", names.AttrUserName, fixture.userName),
					resource.TestCheckResourceAttr("data.ctyun_iam_user.test", "user_id", fixture.userID),
				),
			},
		},
	})
}

// TC-OOS-PROVIDER-06 An endpoint block takes precedence over the environment variable of the same name: the explicit block uses an unreachable port.
// If the environment variable incorrectly overrides the block, S3 bucket creation would use the valid environment endpoint and unexpectedly succeed.
func TestAccOOSProvider_endpointBlockOverridesEnv(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	wrongEndpoint := testAccOOSUnreachableEndpoint(t, os.Getenv("CTYUNOOS_ENDPOINT"))
	rName := acctest.RandomWithPrefix(t, "tf-acc-test-provider-endpoint")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccOOSProviderConfigS3Bucket(wrongEndpoint, rName),
				ExpectError: regexp.MustCompile(`(?i)(creating OOS Bucket|connection refused|request send|operation error S3)`),
			},
		},
	})
}

// TC-OOS-PROVIDER-09 Invalid endpoint template: configures a private domain through the HCL endpoints block.
// Expected: the Terraform configuration path returns the relevant OOS endpoint-template mismatch diagnostic without accessing the cloud.
func TestAccOOSProvider_invalidEndpointTemplate(t *testing.T) {
	testCases := map[string]struct {
		service string
	}{
		"S3":         {service: names.S3},
		"IAM":        {service: names.IAM},
		"CloudTrail": {service: names.CloudTrail},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			resource.Test(t, resource.TestCase{
				ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config:      testAccOOSProviderConfigInvalidEndpointTemplate(tc.service),
						ExpectError: regexp.MustCompile(fmt.Sprintf(`(?i)the\s+%s\s+endpoint[\s\S]*does\s+not\s+match\s+the\s+expected\s+oos\s+endpoint\s+format`, regexp.QuoteMeta(tc.service))),
					},
				},
			})
		})
	}
}

// TC-OOS-PROVIDER-10 Region rejection: the OOS provider schema no longer includes region.
// Expected: Terraform rejects the configuration as an unsupported argument before issuing a provider API request.
func TestAccOOSProvider_regionRejected(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccOOSProviderConfigWithRegion(os.Getenv("CTYUNOOS_ENDPOINT_IAM")),
				ExpectError: regexp.MustCompile(`(?i)unsupported argument`),
			},
		},
	})
}

// TC-OOS-PROVIDER-11 Resource path with a missing IAM endpoint: a provider configured only with an S3 endpoint can initialize.
// Expected: IAM user creation triggers the IAM client resolver, returns "OOS requires a configured iam endpoint", and creates no cloud resource.
func TestAccOOSProvider_missingIAMEndpointOnResourcePath(t *testing.T) {
	ctx := acctest.Context(t)
	t.Setenv("CTYUNOOS_ENDPOINT_IAM", "")
	rName := acctest.RandomWithPrefix(t, "tf-acc-test-provider-missing-iam-endpoint")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccOOSProviderConfigS3OnlyIAMUser(os.Getenv("CTYUNOOS_ENDPOINT"), rName),
				ExpectError: regexp.MustCompile(`OOS requires a configured iam endpoint`),
			},
		},
	})
}

// TC-OOS-PROVIDER-11 AssumeRole configuration with a missing IAM endpoint: configures only an S3 endpoint and enables assume_role.
// Expected: during provider configuration, the AssumeRole chain is rejected for the missing IAM endpoint before any resource request is issued.
func TestAccOOSProvider_assumeRoleWithoutIAMEndpoint(t *testing.T) {
	// Prevent an IAM endpoint environment variable in the shell from populating the provider's undeclared endpoints.iam value.
	t.Setenv("CTYUNOOS_ENDPOINT_IAM", "")

	resource.Test(t, resource.TestCase{
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccOOSProviderConfigS3OnlyAssumeRole("https://oos-cn1.ctyunapi.cn"),
				ExpectError: regexp.MustCompile(`(?i)cannot assume IAM role:[\s\S]*OOS IAM AssumeRole endpoint is not configured`),
			},
		},
	})
}

// TC-OOS-PROVIDER-46 S3 Resource Region Override verifies the deferred error for a missing S3 endpoint when only a resource-level region is configured.
// Expected: the provider initializes using the IAM endpoint, but the S3 request returns an explicit endpoint-configuration error.
func TestAccOOSProvider_s3ResourceRegionWithoutEndpoint(t *testing.T) {
	ctx := acctest.Context(t)
	iamEndpoint := os.Getenv("CTYUNOOS_ENDPOINT_IAM")
	if iamEndpoint == "" {
		t.Skip("CTYUNOOS_ENDPOINT_IAM must be configured")
	}
	region := acctest.SkipIfEnvVarNotSet(t, envvar.OOSAlternateRegion)
	rName := acctest.RandomWithPrefix(t, "tf-acc-test-provider-s3-no-endpoint")
	// Prevent the shell's S3 endpoint environment variable from filling the
	// intentionally omitted provider endpoint.
	t.Setenv("CTYUNOOS_ENDPOINT", "")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccOOSProviderConfigS3BucketWithoutEndpoint(iamEndpoint, rName, region),
				ExpectError: regexp.MustCompile(`OOS requires a configured s3 endpoint`),
			},
		},
	})
}

type oosTemporaryCredentialsFixture struct {
	conn              *iam.Client
	userName          string
	userID            string
	policyARN         string
	sourceAccessKeyID string
	sourceSecretKey   string
	accessKeyID       string
	secretAccessKey   string
	sessionToken      string
	policyAttached    bool
}

func testAccCreateOOSTemporaryCredentialsFixture(ctx context.Context, t *testing.T) *oosTemporaryCredentialsFixture {
	t.Helper()
	fixture := &oosTemporaryCredentialsFixture{
		conn:     acctest.ProviderMeta(ctx, t).IAMClient(ctx),
		userName: acctest.RandomWithPrefix(t, "tf-acc-test-provider-session"),
	}
	t.Cleanup(func() {
		if err := testAccDestroyOOSTemporaryCredentialsFixture(context.Background(), fixture); err != nil {
			t.Errorf("destroying OOS temporary credentials fixture: %s", err)
		}
	})

	userOutput, err := fixture.conn.CreateUser(ctx, &iam.CreateUserInput{UserName: aws.String(fixture.userName)})
	if err != nil {
		t.Fatalf("creating IAM User (%s) for session credentials: %s", fixture.userName, err)
	}
	if userOutput == nil || userOutput.User == nil || aws.ToString(userOutput.User.UserId) == "" {
		t.Fatalf("creating IAM User (%s) for session credentials returned no user ID", fixture.userName)
	}
	fixture.userID = aws.ToString(userOutput.User.UserId)

	policyOutput, err := fixture.conn.CreatePolicy(ctx, &iam.CreatePolicyInput{
		Description:    aws.String("Allow GetUser for provider temporary credentials testing"),
		PolicyDocument: aws.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"iam:GetUser","Resource":"*"}]}`),
		PolicyName:     aws.String(acctest.RandomWithPrefix(t, "tf-acc-test-provider-session-policy")),
	})
	if err != nil {
		t.Fatalf("creating IAM Policy for session credentials: %s", err)
	}
	if policyOutput == nil || policyOutput.Policy == nil || aws.ToString(policyOutput.Policy.Arn) == "" {
		t.Fatal("creating IAM Policy for session credentials returned no ARN")
	}
	fixture.policyARN = aws.ToString(policyOutput.Policy.Arn)
	if _, err := fixture.conn.AttachUserPolicy(ctx, &iam.AttachUserPolicyInput{PolicyArn: aws.String(fixture.policyARN), UserName: aws.String(fixture.userName)}); err != nil {
		t.Fatalf("attaching IAM Policy for session credentials: %s", err)
	}
	fixture.policyAttached = true

	accessKeyOutput, err := fixture.conn.CreateAccessKey(ctx, &iam.CreateAccessKeyInput{UserName: aws.String(fixture.userName)})
	if err != nil || accessKeyOutput == nil || accessKeyOutput.AccessKey == nil {
		t.Fatalf("creating IAM Access Key for session credentials: %v", err)
	}
	fixture.sourceAccessKeyID = aws.ToString(accessKeyOutput.AccessKey.AccessKeyId)
	fixture.sourceSecretKey = aws.ToString(accessKeyOutput.AccessKey.SecretAccessKey)
	fixture.accessKeyID, fixture.secretAccessKey, fixture.sessionToken = testAccGetOOSSessionToken(ctx, t, fixture.sourceAccessKeyID, fixture.sourceSecretKey)
	return fixture
}

func testAccDestroyOOSTemporaryCredentialsFixture(ctx context.Context, fixture *oosTemporaryCredentialsFixture) error {
	if fixture == nil || fixture.conn == nil {
		return nil
	}
	if fixture.sourceAccessKeyID != "" {
		if _, err := fixture.conn.UpdateAccessKey(ctx, &iam.UpdateAccessKeyInput{AccessKeyId: aws.String(fixture.sourceAccessKeyID), Status: awstypes.StatusTypeInactive, UserName: aws.String(fixture.userName)}); err != nil {
			return fmt.Errorf("deactivating IAM Access Key (%s): %w", fixture.sourceAccessKeyID, err)
		}
		if _, err := fixture.conn.DeleteAccessKey(ctx, &iam.DeleteAccessKeyInput{AccessKeyId: aws.String(fixture.sourceAccessKeyID), UserName: aws.String(fixture.userName)}); err != nil {
			return fmt.Errorf("deleting IAM Access Key (%s): %w", fixture.sourceAccessKeyID, err)
		}
	}
	if fixture.policyAttached {
		if _, err := fixture.conn.DetachUserPolicy(ctx, &iam.DetachUserPolicyInput{PolicyArn: aws.String(fixture.policyARN), UserName: aws.String(fixture.userName)}); err != nil {
			return fmt.Errorf("detaching IAM Policy (%s): %w", fixture.policyARN, err)
		}
	}
	if fixture.policyARN != "" {
		if _, err := fixture.conn.DeletePolicy(ctx, &iam.DeletePolicyInput{PolicyArn: aws.String(fixture.policyARN)}); err != nil {
			return fmt.Errorf("deleting IAM Policy (%s): %w", fixture.policyARN, err)
		}
	}
	if fixture.userName != "" {
		if _, err := fixture.conn.DeleteUser(ctx, &iam.DeleteUserInput{UserName: aws.String(fixture.userName)}); err != nil {
			return fmt.Errorf("deleting IAM User (%s): %w", fixture.userName, err)
		}
	}
	return nil
}

type oosSessionTokenResponse struct {
	Result struct {
		Credentials struct {
			AccessKeyID     string `xml:"AccessKeyId"`
			SecretAccessKey string `xml:"SecretAccessKey"`
			SessionToken    string `xml:"SessionToken"`
		} `xml:"Credentials"`
	} `xml:"GetSessionTokenResult"`
}

func testAccGetOOSSessionToken(ctx context.Context, t *testing.T, accessKeyID, secretAccessKey string) (string, string, string) {
	t.Helper()
	endpoint := os.Getenv("CTYUNOOS_ENDPOINT_IAM")
	values := url.Values{"Action": {"GetSessionToken"}, "Version": {"2010-05-08"}, "DurationSeconds": {strconv.Itoa(900)}}
	body := values.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(body))
	if err != nil {
		t.Fatalf("creating GetSessionToken request: %s", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
	req.Header.Set("X-Amz-Content-Sha256", conns.OOSIAMUnsignedPayload)
	region := conns.ParseOOSIAMSigningRegion(endpoint)
	if err := (conns.OOSIAMHTTPSigner{SigningName: conns.OOSIAMSigningName, SigningRegion: region}).SignHTTP(ctx, aws.Credentials{AccessKeyID: accessKeyID, SecretAccessKey: secretAccessKey}, req, conns.OOSIAMUnsignedPayload, conns.OOSIAMSigningName, region, time.Now().UTC()); err != nil {
		t.Fatalf("signing GetSessionToken request: %s", err)
	}
	httpClient := acctest.ProviderMeta(ctx, t).HTTPClient(ctx)
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("calling GetSessionToken: %s", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading GetSessionToken response: %s", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("calling GetSessionToken: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	var output oosSessionTokenResponse
	if err := xml.Unmarshal(responseBody, &output); err != nil {
		t.Fatalf("decoding GetSessionToken response: %s", err)
	}
	if output.Result.Credentials.AccessKeyID == "" || output.Result.Credentials.SecretAccessKey == "" || output.Result.Credentials.SessionToken == "" {
		t.Fatal("GetSessionToken response did not contain complete credentials")
	}
	return output.Result.Credentials.AccessKeyID, output.Result.Credentials.SecretAccessKey, output.Result.Credentials.SessionToken
}

func testAccOOSProviderConfigIAM(endpoint, accessKey, secretKey, sessionToken string) string {
	var credentials strings.Builder
	if accessKey != "" {
		fmt.Fprintf(&credentials, "  access_key     = %q\n", accessKey)
		fmt.Fprintf(&credentials, "  secret_key     = %q\n", secretKey)
	}
	if sessionToken != "" {
		fmt.Fprintf(&credentials, "  security_token = %q\n", sessionToken)
	}

	return fmt.Sprintf(`
provider "ctyun" {
%s  endpoints {
    iam = %q
  }
}

data "ctyun_iam_account_summary" "test" {}
`, credentials.String(), endpoint)
}

func testAccOOSProviderConfigIAMUser(endpoint, accessKey, secretKey, sessionToken, userName string) string {
	var credentials strings.Builder
	fmt.Fprintf(&credentials, "  access_key     = %q\n", accessKey)
	fmt.Fprintf(&credentials, "  secret_key     = %q\n", secretKey)
	fmt.Fprintf(&credentials, "  security_token = %q\n", sessionToken)

	return fmt.Sprintf(`
provider "ctyun" {
%s  endpoints {
    iam = %q
  }
}

data "ctyun_iam_user" "test" {
  user_name = %q
}
`, credentials.String(), endpoint, userName)
}

func testAccOOSProviderConfigIAMUserEnv(endpoint, userName string) string {
	return fmt.Sprintf(`
provider "ctyun" {
  endpoints {
    iam = %q
  }
}

data "ctyun_iam_user" "test" {
  user_name = %q
}
`, endpoint, userName)
}

func testAccOOSProviderConfigWithRegion(endpoint string) string {
	return fmt.Sprintf(`
provider "ctyun" {
  region = "cn1"

  endpoints {
    iam = %q
  }
}

data "ctyun_iam_account_summary" "test" {}
`, endpoint)
}

func testAccOOSProviderConfigInvalidEndpointTemplate(service string) string {
	endpoints := map[string]string{
		names.S3:         "https://oos-cn1.ctyunapi.cn",
		names.IAM:        "https://oos-cn1-iam.ctyunapi.cn",
		names.CloudTrail: "https://oos-cn1-cloudtrail.ctyunapi.cn",
	}
	endpoints[service] = "https://my-private-oos.example.com"

	return fmt.Sprintf(`
provider "ctyun" {
  access_key = "invalid-access-key"
  secret_key = "invalid-secret-key"

  endpoints {
    s3         = %q
    iam        = %q
    cloudtrail = %q
  }
}

data "ctyun_iam_account_summary" "test" {}
`, endpoints[names.S3], endpoints[names.IAM], endpoints[names.CloudTrail])
}

func testAccOOSProviderConfigS3OnlyIAMUser(endpoint, userName string) string {
	return fmt.Sprintf(`
provider "ctyun" {
  endpoints {
    s3 = %q
  }
}

resource "ctyun_iam_user" "test" {
  name = %q
}
`, endpoint, userName)
}

func testAccOOSProviderConfigS3OnlyAssumeRole(s3Endpoint string) string {
	return fmt.Sprintf(`
provider "ctyun" {
  access_key = "test-access-key"
  secret_key = "test-secret-key"

  assume_role {
    role_arn     = "arn:ctyun:iam::32fefj64y54gc:role/AssumeRole"
    session_name = "test-session"
  }

  endpoints {
    s3 = %q
  }
}

data "ctyun_iam_account_summary" "test" {}
`, s3Endpoint)
}

func testAccOOSUnreachableEndpoint(t *testing.T, endpoint string) string {
	t.Helper()
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Hostname() == "" {
		t.Fatalf("parsing CTYUNOOS_ENDPOINT %q: %v", endpoint, err)
	}
	parsed.Host = net.JoinHostPort(parsed.Hostname(), "1")
	parsed.Path = ""
	parsed.RawPath = ""
	return parsed.String()
}

func testAccOOSProviderConfigS3Bucket(endpoint, bucketName string) string {
	return fmt.Sprintf(`
provider "ctyun" {
  max_retries = 1

  endpoints {
    s3 = %q
  }
}

resource "ctyun_oos_bucket" "test" {
  bucket = %q
}
`, endpoint, bucketName)
}

func testAccOOSProviderConfigS3BucketWithoutEndpoint(iamEndpoint, bucketName, region string) string {
	return fmt.Sprintf(`
provider "ctyun" {
  max_retries = 1

  endpoints {
    iam = %q
  }
}

resource "ctyun_oos_bucket" "test" {
  bucket = %q
  region = %q
}
`, iamEndpoint, bucketName, region)
}
