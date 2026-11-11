// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package acctest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest/jsoncmp"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/dns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/envvar"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/sdkdiag"
	tfsync "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/experimental/sync"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/provider"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/provider/sdkv2"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/retry"
	tfiam "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/iam"
	tfslices "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/slices"
	inttypes "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/aws-sdk-go-base/v2/endpoints"
	"github.com/hashicorp/aws-sdk-go-base/v2/tfawserr"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	sdkid "github.com/hashicorp/terraform-plugin-sdk/v2/helper/id"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/structure"
	terraformsdk "github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-plugin-testing/echoprovider"
	sdkacctest "github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/jmespath/go-jmespath"
	"github.com/mitchellh/mapstructure"
)

const (
	// Provider name for single configuration testing
	ProviderName = "aws"

	// ProviderNameOOS is the provider local name for OOS custom resources (ctyun_oos_*).
	// terraform-plugin-testing looks up the provider factory by the resource type prefix (the segment before the first
	// underscore); ctyun_oos_bucket corresponds to the local name "ctyun". The default factory map registers both aws and
	// ctyun, so that both the retained aws_* resource tests and the OOS custom resource tests hit the in-process provider.
	ProviderNameOOS = "ctyun"

	// Provider name for alternate configuration testing
	ProviderNameAlternate = "awsalternate"

	// Provider name for echo provider
	// used for testing ephemeral resources
	ProviderNameEcho = "echo"

	// Provider name for alternate account and alternate region configuration testing
	ProviderNameAlternateAccountAlternateRegion = "awsalternateaccountalternateregion"

	// Provider name for alternate account and same region configuration testing
	ProviderNameAlternateAccountSameRegion = "awsalternateaccountsameregion"

	// Provider name for same account and alternate region configuration testing
	ProviderNameSameAccountAlternateRegion = "awssameaccountalternateregion"

	// Provider name for third configuration testing
	ProviderNameThird = "awsthird"

	// Provider name for fourth configuration testing
	ProviderNameFourth = "awsfourth"

	ResourcePrefix = "tf-acc-test"

	CertificateIssueTimeout = 5 * time.Minute
)

const RFC3339RegexPattern = `^[0-9]{4}-(0[1-9]|1[012])-(0[1-9]|[12][0-9]|3[01])[Tt]([01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9](\.[0-9]+)?([Zz]|([+-]([01][0-9]|2[0-3]):[0-5][0-9]))$`
const accountIDRegexp = `(aws|aws-managed|partner-managed|\d{12})`

// Skip implements a wrapper for (*testing.T).Skip() to prevent unused linting reports
//
// Reference: https://github.com/dominikh/go-tools/issues/633#issuecomment-606560616
func Skip(t *testing.T, message string) {
	t.Helper()

	t.Skip(message)
}

// ProtoV5ProviderFactories is a static map containing only the main provider instance
//
// Use other ProviderFactories functions, such as FactoriesAlternate,
// for tests requiring special provider configurations.
var (
	ProtoV5ProviderFactories map[string]func() (tfprotov5.ProviderServer, error) = protoV5ProviderFactoriesInit(context.Background(), ProviderName, ProviderNameOOS)
)

// Provider is the "default" provider instance.
//
// This Provider can be used in testing code for API calls without requiring
// the use of saving and referencing specific ProviderFactories instances.
//
// PreCheck(t) must be called before using this provider instance.
var (
	Provider *schema.Provider = errs.Must(sdkv2.NewProvider(context.Background()))
)

type ProviderFunc func() *schema.Provider

var (
	DefaultProviderFunc ProviderFunc = func() *schema.Provider {
		return Provider
	}
)

// testAccProviderConfigure ensures Provider is only configured once
//
// The PreCheck(t) function is invoked for every test and this prevents
// extraneous reconfiguration to the same values each time. However, this does
// not prevent reconfiguration that may happen should the address of
// Provider be errantly reused in ProviderFactories.
var testAccProviderConfigure sync.Once

func protoV5ProviderFactoriesInit(ctx context.Context, providerNames ...string) map[string]func() (tfprotov5.ProviderServer, error) {
	factories := make(map[string]func() (tfprotov5.ProviderServer, error), len(providerNames))

	for _, name := range providerNames {
		factories[name] = func() (tfprotov5.ProviderServer, error) {
			providerServerFactory, _, err := provider.ProtoV5ProviderServerFactory(ctx)

			if err != nil {
				return nil, err
			}

			return providerServerFactory(), nil
		}
	}

	return factories
}

// ProtoV6ProviderFactories initializes v6 provider factories
// currently only initializes echo provider for testing ephemeral resources
func ProtoV6ProviderFactories(_ context.Context, providerNames ...string) map[string]func() (tfprotov6.ProviderServer, error) {
	factories := make(map[string]func() (tfprotov6.ProviderServer, error))

	for _, name := range providerNames {
		if name == ProviderNameEcho {
			factories[name] = echoprovider.NewProviderServer()
		}
	}

	return factories
}

func protoV5ProviderFactoriesNamedInit(ctx context.Context, t *testing.T, providers map[string]*schema.Provider, providerNames ...string) map[string]func() (tfprotov5.ProviderServer, error) {
	t.Helper()

	factories := make(map[string]func() (tfprotov5.ProviderServer, error), len(providerNames))

	for _, name := range providerNames {
		providerServerFactory, p, err := provider.ProtoV5ProviderServerFactory(ctx)

		if err != nil {
			t.Fatal(err)
		}

		factories[name] = func() (tfprotov5.ProviderServer, error) { //nolint:unparam
			return providerServerFactory(), nil
		}

		providers[name] = p
	}

	return factories
}

func protoV5ProviderFactoriesPlusProvidersInit(ctx context.Context, t *testing.T, providers *[]*schema.Provider, providerNames ...string) map[string]func() (tfprotov5.ProviderServer, error) {
	t.Helper()

	factories := make(map[string]func() (tfprotov5.ProviderServer, error), len(providerNames))

	for _, name := range providerNames {
		providerServerFactory, p, err := provider.ProtoV5ProviderServerFactory(ctx)

		if err != nil {
			t.Fatal(err)
		}

		factories[name] = func() (tfprotov5.ProviderServer, error) { //nolint:unparam
			return providerServerFactory(), nil
		}

		if providers != nil {
			*providers = append(*providers, p)
		}
	}

	return factories
}

// ProtoV5FactoriesPlusProvidersAlternate creates ProtoV5ProviderFactories for cross-account and cross-region configurations
// and also returns Providers suitable for use with AWS APIs.
//
// For cross-region testing: Typically paired with PreCheckMultipleRegion and ConfigAlternateRegionProvider.
//
// For cross-account testing: Typically paired with PreCheckAlternateAccount and ConfigAlternateAccountProvider.
func ProtoV5FactoriesPlusProvidersAlternate(ctx context.Context, t *testing.T, providers *[]*schema.Provider) map[string]func() (tfprotov5.ProviderServer, error) {
	t.Helper()

	return protoV5ProviderFactoriesPlusProvidersInit(ctx, t, providers, ProviderName, ProviderNameAlternate)
}

func ProtoV5FactoriesPlusProvidersThird(ctx context.Context, t *testing.T, providers *[]*schema.Provider) map[string]func() (tfprotov5.ProviderServer, error) {
	t.Helper()

	return protoV5ProviderFactoriesPlusProvidersInit(ctx, t, providers, ProviderName, ProviderNameAlternate, ProviderNameThird)
}

func ProtoV5FactoriesNamedAlternate(ctx context.Context, t *testing.T, providers map[string]*schema.Provider) map[string]func() (tfprotov5.ProviderServer, error) {
	t.Helper()

	return ProtoV5FactoriesNamed(ctx, t, providers, ProviderName, ProviderNameAlternate)
}

func ProtoV5FactoriesNamed(ctx context.Context, t *testing.T, providers map[string]*schema.Provider, providerNames ...string) map[string]func() (tfprotov5.ProviderServer, error) {
	t.Helper()

	return protoV5ProviderFactoriesNamedInit(ctx, t, providers, providerNames...)
}

func ProtoV5FactoriesAlternate(ctx context.Context, t *testing.T) map[string]func() (tfprotov5.ProviderServer, error) {
	t.Helper()

	return protoV5ProviderFactoriesInit(ctx, ProviderName, ProviderNameAlternate)
}

// ProtoV5FactoriesAlternateAccountAndAlternateRegion creates ProtoV5ProviderFactories for cross-account and cross-region configurations
//
// Usage typically paired with PreCheckMultipleRegion, PreCheckAlternateAccount,
// and ConfigAlternateAccountAndAlternateRegionProvider.
func ProtoV5FactoriesAlternateAccountAndAlternateRegion(ctx context.Context, t *testing.T) map[string]func() (tfprotov5.ProviderServer, error) {
	t.Helper()

	return protoV5ProviderFactoriesInit(
		ctx,
		ProviderName,
		ProviderNameAlternateAccountAlternateRegion,
		ProviderNameAlternateAccountSameRegion,
		ProviderNameSameAccountAlternateRegion,
	)
}

// ProtoV5FactoriesMultipleRegions creates ProtoV5ProviderFactories for the specified number of region configurations
//
// Usage typically paired with PreCheckMultipleRegion and ConfigMultipleRegionProvider.
func ProtoV5FactoriesMultipleRegions(ctx context.Context, t *testing.T, n int) map[string]func() (tfprotov5.ProviderServer, error) {
	t.Helper()

	switch n {
	case 2:
		return protoV5ProviderFactoriesInit(ctx, ProviderName, ProviderNameAlternate)
	case 3:
		return protoV5ProviderFactoriesInit(ctx, ProviderName, ProviderNameAlternate, ProviderNameThird)
	case 4:
		return protoV5ProviderFactoriesInit(ctx, ProviderName, ProviderNameAlternate, ProviderNameThird, ProviderNameFourth)
	default:
		t.Fatalf("invalid number of Region configurations: %d", n)
	}

	return nil
}

// PreCheck verifies and sets required provider testing configuration
//
// This PreCheck function should be present in every acceptance test. It allows
// test configurations to omit a provider configuration with region and ensures
// testing functions that attempt to call AWS APIs are previously configured.
//
// These verifications and configuration are preferred at this level to prevent
// provider developers from experiencing less clear errors for every test.
func PreCheck(ctx context.Context, t *testing.T) {
	t.Helper()

	// Since we are outside the scope of the Terraform configuration we must
	// call Configure() to properly initialize the provider configuration.
	testAccProviderConfigure.Do(func() {
		envvar.FailIfEmpty(t, "CTYUNOOS_ACCESS_KEY", "OOS access key for running acceptance testing")
		envvar.FailIfEmpty(t, "CTYUNOOS_SECRET_KEY", "OOS secret key for running acceptance testing")

		// Setting the AWS_DEFAULT_REGION environment variable here allows all tests to omit
		// a provider configuration with a region. This defaults to us-west-2 for provider
		// developer simplicity and has been in the codebase for a very long time.
		//
		// This handling must be preserved until either:
		//   * AWS_DEFAULT_REGION is required and checked above (should mention us-west-2 default)
		//   * Region is automatically handled via shared AWS configuration file and still verified
		region := Region()
		os.Setenv(envvar.DefaultRegion, region)

		Provider.TerraformVersion = "1.0.0"
		diags := Provider.Configure(ctx, terraformsdk.NewResourceConfigRaw(nil))
		if err := sdkdiag.DiagnosticsError(diags); err != nil {
			t.Fatalf("configuring provider: %s", err)
		}
	})
}

// ProviderAccountID returns the account ID of an AWS provider
func ProviderAccountID(ctx context.Context, provider *schema.Provider) string {
	if provider == nil {
		log.Print("[DEBUG] Unable to read account ID from test provider: empty provider")
		return ""
	}
	if provider.Meta() == nil {
		log.Print("[DEBUG] Unable to read account ID from test provider: unconfigured provider")
		return ""
	}
	client, ok := provider.Meta().(*conns.AWSClient)
	if !ok {
		log.Print("[DEBUG] Unable to read account ID from test provider: non-AWS or unconfigured AWS provider")
		return ""
	}
	return client.AccountID(ctx)
}

// CheckDestroyNoop is a TestCheckFunc to be used as a TestCase's CheckDestroy when no such check can be made.
func CheckDestroyNoop(*terraform.State) error {
	return nil
}

// CheckSleep returns a TestCheckFunc that pauses the current goroutine for at least the duration d.
func CheckSleep(t *testing.T, d time.Duration) resource.TestCheckFunc {
	t.Helper()

	return func(*terraform.State) error {
		time.Sleep(d)

		return nil
	}
}

// CheckResourceAttrAccountID ensures the Terraform state exactly matches the account ID
func CheckResourceAttrAccountID(ctx context.Context, resourceName, attributeName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		return resource.TestCheckResourceAttr(resourceName, attributeName, AccountID(ctx))(s)
	}
}

// CheckResourceAttrRegionalARN ensures the Terraform state exactly matches a formatted ARN with region
func CheckResourceAttrRegionalARN(ctx context.Context, resourceName, attributeName, arnService, arnResource string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		attributeValue := arn.ARN{
			AccountID: AccountID(ctx),
			Partition: Partition(),
			Region:    Region(),
			Resource:  arnResource,
			Service:   arnService,
		}.String()
		return resource.TestCheckResourceAttr(resourceName, attributeName, attributeValue)(s)
	}
}

// CheckResourceAttrRegionalARNNoAccount ensures the Terraform state exactly matches a formatted ARN with region but without account ID
func CheckResourceAttrRegionalARNNoAccount(resourceName, attributeName, arnService, arnResource string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		attributeValue := arn.ARN{
			Partition: Partition(),
			Region:    Region(),
			Resource:  arnResource,
			Service:   arnService,
		}.String()
		return resource.TestCheckResourceAttr(resourceName, attributeName, attributeValue)(s)
	}
}

// CheckResourceAttrRegionalARNAccountID ensures the Terraform state exactly matches a formatted ARN with region and specific account ID
func CheckResourceAttrRegionalARNAccountID(resourceName, attributeName, arnService, accountID, arnResource string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		attributeValue := arn.ARN{
			AccountID: accountID,
			Partition: Partition(),
			Region:    Region(),
			Resource:  arnResource,
			Service:   arnService,
		}.String()
		return resource.TestCheckResourceAttr(resourceName, attributeName, attributeValue)(s)
	}
}

// CheckResourceAttrRegionalHostnameService ensures the Terraform state exactly matches a service DNS hostname with region and partition DNS suffix
//
// For example: ec2.us-west-2.amazonaws.com
func CheckResourceAttrRegionalHostnameService(resourceName, attributeName, serviceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		hostname := fmt.Sprintf("%s.%s.%s", serviceName, Region(), PartitionDNSSuffix())

		return resource.TestCheckResourceAttr(resourceName, attributeName, hostname)(s)
	}
}

// CheckResourceAttrNameFromPrefix verifies that the state attribute value matches name generated from given prefix
func CheckResourceAttrNameFromPrefix(resourceName string, attributeName string, prefix string) resource.TestCheckFunc {
	return CheckResourceAttrNameWithSuffixFromPrefix(resourceName, attributeName, prefix, "")
}

// Regexp for "<start-of-string>terraform-<26 lowercase hex digits><additional suffix><end-of-string>".
func resourceUniqueIDPrefixPlusAdditionalSuffixRegexp(prefix, suffix string) *regexp.Regexp {
	return regexache.MustCompile(fmt.Sprintf("^%s[[:xdigit:]]{%d}%s$", prefix, sdkid.UniqueIDSuffixLength, suffix))
}

// CheckResourceAttrNameWithSuffixFromPrefix verifies that the state attribute value matches name with suffix generated from given prefix
func CheckResourceAttrNameWithSuffixFromPrefix(resourceName string, attributeName string, prefix string, suffix string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		attributeMatch := resourceUniqueIDPrefixPlusAdditionalSuffixRegexp(prefix, suffix)
		return resource.TestMatchResourceAttr(resourceName, attributeName, attributeMatch)(s)
	}
}

// CheckResourceAttrNameGenerated verifies that the state attribute value matches name automatically generated without prefix
func CheckResourceAttrNameGenerated(resourceName string, attributeName string) resource.TestCheckFunc {
	return CheckResourceAttrNameWithSuffixGenerated(resourceName, attributeName, "")
}

// CheckResourceAttrNameGeneratedWithPrefix verifies that the state attribute value matches name automatically generated with prefix
func CheckResourceAttrNameGeneratedWithPrefix(resourceName string, attributeName string, prefix string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		return resource.TestMatchResourceAttr(resourceName, attributeName, resourceUniqueIDPrefixPlusAdditionalSuffixRegexp(prefix, ""))(s)
	}
}

// CheckResourceAttrNameWithSuffixGenerated verifies that the state attribute value matches name with suffix automatically generated without prefix
func CheckResourceAttrNameWithSuffixGenerated(resourceName string, attributeName string, suffix string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		return resource.TestMatchResourceAttr(resourceName, attributeName, resourceUniqueIDPrefixPlusAdditionalSuffixRegexp(sdkid.UniqueIdPrefix, suffix))(s)
	}
}

// MatchResourceAttrAccountID ensures the Terraform state regexp matches an account ID
func MatchResourceAttrAccountID(resourceName, attributeName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		return resource.TestMatchResourceAttr(resourceName, attributeName, regexache.MustCompile(`^\d{12}$`))(s)
	}
}

// MatchResourceAttrRegionalARN ensures the Terraform state regexp matches a formatted ARN with region
func MatchResourceAttrRegionalARN(ctx context.Context, resourceName, attributeName, arnService string, arnResourceRegexp *regexp.Regexp) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		arnRegexp := arn.ARN{
			AccountID: AccountID(ctx),
			Partition: Partition(),
			Region:    Region(),
			Resource:  arnResourceRegexp.String(),
			Service:   arnService,
		}.String()

		attributeMatch, err := regexp.Compile(arnRegexp)

		if err != nil {
			return fmt.Errorf("unable to compile ARN regexp (%s): %w", arnRegexp, err)
		}

		return resource.TestMatchResourceAttr(resourceName, attributeName, attributeMatch)(s)
	}
}

// MatchResourceAttrRegionalARNRegion ensures the Terraform state regexp matches a formatted ARN with the specified region
func MatchResourceAttrRegionalARNRegion(ctx context.Context, resourceName, attributeName, arnService, region string, arnResourceRegexp *regexp.Regexp) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		arnRegexp := arn.ARN{
			AccountID: AccountID(ctx),
			Partition: Partition(),
			Region:    region,
			Resource:  arnResourceRegexp.String(),
			Service:   arnService,
		}.String()

		attributeMatch, err := regexp.Compile(arnRegexp)

		if err != nil {
			return fmt.Errorf("unable to compile ARN regexp (%s): %w", arnRegexp, err)
		}

		return resource.TestMatchResourceAttr(resourceName, attributeName, attributeMatch)(s)
	}
}

// MatchResourceAttrRegionalARNNoAccount ensures the Terraform state regexp matches a formatted ARN with region but without account ID
func MatchResourceAttrRegionalARNNoAccount(resourceName, attributeName, arnService string, arnResourceRegexp *regexp.Regexp) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		arnRegexp := arn.ARN{
			Partition: Partition(),
			Region:    Region(),
			Resource:  arnResourceRegexp.String(),
			Service:   arnService,
		}.String()

		attributeMatch, err := regexp.Compile(arnRegexp)

		if err != nil {
			return fmt.Errorf("unable to compile ARN regexp (%s): %w", arnRegexp, err)
		}

		return resource.TestMatchResourceAttr(resourceName, attributeName, attributeMatch)(s)
	}
}

// MatchResourceAttrRegionalARNAccountID ensures the Terraform state regexp matches a formatted ARN with region and specific account ID
func MatchResourceAttrRegionalARNAccountID(resourceName, attributeName, arnService, accountID string, arnResourceRegexp *regexp.Regexp) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		arnRegexp := arn.ARN{
			AccountID: accountID,
			Partition: Partition(),
			Region:    Region(),
			Resource:  arnResourceRegexp.String(),
			Service:   arnService,
		}.String()

		attributeMatch, err := regexp.Compile(arnRegexp)

		if err != nil {
			return fmt.Errorf("unable to compile ARN regexp (%s): %w", arnRegexp, err)
		}

		return resource.TestMatchResourceAttr(resourceName, attributeName, attributeMatch)(s)
	}
}

// MatchResourceAttrRegionalHostname ensures the Terraform state regexp matches a formatted DNS hostname with region and partition DNS suffix
func MatchResourceAttrRegionalHostname(resourceName, attributeName, serviceName string, hostnamePrefixRegexp *regexp.Regexp) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		hostnameRegexpPattern := fmt.Sprintf("%s\\.%s\\.%s\\.%s$", hostnamePrefixRegexp.String(), serviceName, Region(), PartitionDNSSuffix())

		hostnameRegexp, err := regexp.Compile(hostnameRegexpPattern)

		if err != nil {
			return fmt.Errorf("unable to compile hostname regexp (%s): %w", hostnameRegexp, err)
		}

		return resource.TestMatchResourceAttr(resourceName, attributeName, hostnameRegexp)(s)
	}
}

// MatchResourceAttrGlobalHostname ensures the Terraform state regexp matches a formatted DNS hostname with partition DNS suffix and without region
func MatchResourceAttrGlobalHostname(resourceName, attributeName, serviceName string, hostnamePrefixRegexp *regexp.Regexp) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		hostnameRegexpPattern := fmt.Sprintf("%s\\.%s\\.%s$", hostnamePrefixRegexp.String(), serviceName, PartitionDNSSuffix())

		hostnameRegexp, err := regexp.Compile(hostnameRegexpPattern)

		if err != nil {
			return fmt.Errorf("unable to compile hostname regexp (%s): %w", hostnameRegexp, err)
		}

		return resource.TestMatchResourceAttr(resourceName, attributeName, hostnameRegexp)(s)
	}
}

func globalARNValue(ctx context.Context, arnService, arnResource string) string {
	return arn.ARN{
		AccountID: AccountID(ctx),
		Partition: Partition(),
		Resource:  arnResource,
		Service:   arnService,
	}.String()
}

// CheckResourceAttrGlobalARN ensures the Terraform state exactly matches a formatted ARN without region
func CheckResourceAttrGlobalARN(ctx context.Context, resourceName, attributeName, arnService, arnResource string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		return resource.TestCheckResourceAttr(resourceName, attributeName, globalARNValue(ctx, arnService, arnResource))(s)
	}
}

// CheckResourceAttrGlobalARNNoAccount ensures the Terraform state exactly matches a formatted ARN without region or account ID
func CheckResourceAttrGlobalARNNoAccount(resourceName, attributeName, arnService, arnResource string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		attributeValue := arn.ARN{
			Partition: Partition(),
			Resource:  arnResource,
			Service:   arnService,
		}.String()
		return resource.TestCheckResourceAttr(resourceName, attributeName, attributeValue)(s)
	}
}

// CheckResourceAttrGlobalARNAccountID ensures the Terraform state exactly matches a formatted ARN without region and with specific account ID
func CheckResourceAttrGlobalARNAccountID(resourceName, attributeName, accountID, arnService, arnResource string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		attributeValue := arn.ARN{
			AccountID: accountID,
			Partition: Partition(),
			Resource:  arnResource,
			Service:   arnService,
		}.String()
		return resource.TestCheckResourceAttr(resourceName, attributeName, attributeValue)(s)
	}
}

// MatchResourceAttrGlobalARN ensures the Terraform state regexp matches a formatted ARN without region
func MatchResourceAttrGlobalARN(ctx context.Context, resourceName, attributeName, arnService string, arnResourceRegexp *regexp.Regexp) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		arnRegexp := arn.ARN{
			AccountID: AccountID(ctx),
			Partition: Partition(),
			Resource:  arnResourceRegexp.String(),
			Service:   arnService,
		}.String()

		attributeMatch, err := regexp.Compile(arnRegexp)

		if err != nil {
			return fmt.Errorf("unable to compile ARN regexp (%s): %w", arnRegexp, err)
		}

		return resource.TestMatchResourceAttr(resourceName, attributeName, attributeMatch)(s)
	}
}

// CheckResourceAttrRegionalARNIgnoreRegionAndAccount ensures the Terraform state exactly matches a formatted ARN with region without specifying the region or account
func CheckResourceAttrRegionalARNIgnoreRegionAndAccount(resourceName, attributeName, arnService, arnResource string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		arnRegexp := arn.ARN{
			AccountID: accountIDRegexp,
			Partition: Partition(),
			Region:    inttypes.CanonicalRegionPatternNoAnchors,
			Resource:  arnResource,
			Service:   arnService,
		}.String()

		attributeMatch, err := regexp.Compile(arnRegexp)

		if err != nil {
			return fmt.Errorf("unable to compile ARN regexp (%s): %w", arnRegexp, err)
		}

		return resource.TestMatchResourceAttr(resourceName, attributeName, attributeMatch)(s)
	}
}

// MatchResourceAttrGlobalARNNoAccount ensures the Terraform state regexp matches a formatted ARN without region or account ID
func MatchResourceAttrGlobalARNNoAccount(resourceName, attributeName, arnService string, arnResourceRegexp *regexp.Regexp) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		arnRegexp := arn.ARN{
			Partition: Partition(),
			Resource:  arnResourceRegexp.String(),
			Service:   arnService,
		}.String()

		attributeMatch, err := regexp.Compile(arnRegexp)

		if err != nil {
			return fmt.Errorf("unable to compile ARN regexp (%s): %w", arnRegexp, err)
		}

		return resource.TestMatchResourceAttr(resourceName, attributeName, attributeMatch)(s)
	}
}

// CheckResourceAttrRFC3339 ensures the Terraform state matches a RFC3339 value
// This TestCheckFunc will likely be moved to the Terraform Plugin SDK in the future.
func CheckResourceAttrRFC3339(resourceName, attributeName string) resource.TestCheckFunc {
	return resource.TestMatchResourceAttr(resourceName, attributeName, regexache.MustCompile(RFC3339RegexPattern))
}

// CheckResourceAttrEquivalentJSON is a TestCheckFunc that compares a JSON value with an expected value.
// Both JSON values are normalized before being compared.
func CheckResourceAttrEquivalentJSON(n, key, expectedJSON string) resource.TestCheckFunc {
	return resource.TestCheckResourceAttrWith(n, key, func(value string) error {
		vNormal, err := structure.NormalizeJsonString(value)
		if err != nil {
			return fmt.Errorf("%s: Error normalizing JSON in %q: %w", n, key, err)
		}

		expectedNormal, err := structure.NormalizeJsonString(expectedJSON)
		if err != nil {
			return fmt.Errorf("normalizing expected JSON: %w", err)
		}

		if vNormal != expectedNormal {
			return fmt.Errorf("%s: Attribute %q expected\n%s\ngot\n%s", n, key, expectedJSON, value)
		}
		return nil
	})
}

func CheckResourceAttrJSONNoDiff(n, key, expectedJSON string) resource.TestCheckFunc {
	return resource.TestCheckResourceAttrWith(n, key, func(value string) error {
		if diff := jsoncmp.Diff(value, expectedJSON); diff != "" {
			return fmt.Errorf("unexpected diff (+wanted, -got): %s", diff)
		}

		return nil
	})
}

func CheckResourceAttrJMES(name, key, jmesPath, value string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		is, err := PrimaryInstanceState(s, name)
		if err != nil {
			return err
		}

		attr, ok := is.Attributes[key]
		if !ok {
			return fmt.Errorf("%s: Attribute %q not set", name, key)
		}

		var jsonData any
		err = json.Unmarshal([]byte(attr), &jsonData)
		if err != nil {
			return fmt.Errorf("%s: Expected attribute %q to be JSON: %w", name, key, err)
		}

		result, err := jmespath.Search(jmesPath, jsonData)
		if err != nil {
			return fmt.Errorf("invalid JMESPath %q: %w", jmesPath, err)
		}

		var v string
		switch x := result.(type) {
		case string:
			v = x
		case float64:
			v = strconv.FormatFloat(x, 'f', -1, 64)
		case bool:
			v = strconv.FormatBool(x)
		default:
			return fmt.Errorf(`%[1]s: Attribute %[2]q, JMESPath %[3]q got "%#[4]v" (%[4]T)`, name, key, jmesPath, result)
		}

		if v != value {
			return fmt.Errorf("%s: Attribute %q, JMESPath %q expected %#v, got %#v", name, key, jmesPath, value, v)
		}

		return nil
	}
}

func CheckResourceAttrJMESPair(nameFirst, keyFirst, jmesPath, nameSecond, keySecond string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		first, err := PrimaryInstanceState(s, nameFirst)
		if err != nil {
			return err
		}

		second, err := PrimaryInstanceState(s, nameSecond)
		if err != nil {
			return err
		}

		vFirst, okFirst := first.Attributes[keyFirst]
		if !okFirst {
			return fmt.Errorf("%s: Attribute %q not set", nameFirst, keyFirst)
		}

		var jsonData any
		err = json.Unmarshal([]byte(vFirst), &jsonData)
		if err != nil {
			return fmt.Errorf("%s: Expected attribute %q to be JSON: %w", nameFirst, keyFirst, err)
		}

		result, err := jmespath.Search(jmesPath, jsonData)
		if err != nil {
			return fmt.Errorf("invalid JMESPath %q: %w", jmesPath, err)
		}

		var value string
		switch x := result.(type) {
		case string:
			value = x
		case float64:
			value = strconv.FormatFloat(x, 'f', -1, 64)
		case bool:
			value = strconv.FormatBool(x)
		default:
			return fmt.Errorf(`%[1]s: Attribute %[2]q, JMESPath %[3]q got "%#[4]v" (%[4]T)`, nameFirst, keyFirst, jmesPath, result)
		}

		vSecond, okSecond := second.Attributes[keySecond]
		if !okSecond {
			return fmt.Errorf("%s: Attribute %q, JMESPath %q is %q, but %q is not set in %s", nameFirst, keyFirst, jmesPath, value, keySecond, nameSecond)
		}

		if value != vSecond {
			return fmt.Errorf("%s: Attribute %q, JMESPath %q, expected %q, got %q", nameFirst, keyFirst, jmesPath, vSecond, value)
		}

		return nil
	}
}

func CheckResourceAttrJMESNotExists(name, key, jmesPath string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		is, err := PrimaryInstanceState(s, name)
		if err != nil {
			return err
		}

		attr, ok := is.Attributes[key]
		if !ok {
			return fmt.Errorf("%s: Attribute %q not set", name, key)
		}

		var jsonData any
		err = json.Unmarshal([]byte(attr), &jsonData)
		if err != nil {
			return fmt.Errorf("%s: Expected attribute %q to be JSON: %w", name, key, err)
		}

		result, err := jmespath.Search(jmesPath, jsonData)
		if err != nil {
			return fmt.Errorf("invalid JMESPath %q: %w", jmesPath, err)
		}

		var v string
		switch x := result.(type) {
		case nil:
			return nil
		case string:
			v = x
		case float64:
			v = strconv.FormatFloat(x, 'f', -1, 64)
		case bool:
			v = strconv.FormatBool(x)
		default:
			return fmt.Errorf(`%[1]s: Attribute %[2]q, JMESPath %[3]q got "%#[4]v" (%[4]T), expected no attribute`, name, key, jmesPath, result)
		}

		return fmt.Errorf("%s: Attribute %q, JMESPath %q expected no attribute, got %#v", name, key, jmesPath, v)
	}
}

// CheckResourceAttrContains ensures the Terraform state value contains the specified substr.
func CheckResourceAttrContains(name, key, substr string) resource.TestCheckFunc {
	return resource.TestCheckResourceAttrWith(name, key, func(value string) error {
		if strings.Contains(value, substr) {
			return nil
		}
		return fmt.Errorf("%s: Attribute '%s' expected contains %#v, got %#v", name, key, substr, value)
	})
}

// CheckResourceAttrHasPrefix ensures the Terraform state value has the specified prefix.
func CheckResourceAttrHasPrefix(name, key, prefix string) resource.TestCheckFunc {
	return resource.TestCheckResourceAttrWith(name, key, func(value string) error {
		if strings.HasPrefix(value, prefix) {
			return nil
		}
		return fmt.Errorf("%s: Attribute '%s' expected prefix %#v, got %#v", name, key, prefix, value)
	})
}

// CheckResourceAttrHasSuffix ensures the Terraform state value has the specified suffix.
func CheckResourceAttrHasSuffix(name, key, suffix string) resource.TestCheckFunc {
	return resource.TestCheckResourceAttrWith(name, key, func(value string) error {
		if strings.HasSuffix(value, suffix) {
			return nil
		}
		return fmt.Errorf("%s: Attribute '%s' expected suffix %#v, got %#v", name, key, suffix, value)
	})
}

// Copied and inlined from the SDK testing code
func PrimaryInstanceState(s *terraform.State, name string) (*terraform.InstanceState, error) {
	rs, ok := s.RootModule().Resources[name]
	if !ok {
		return nil, fmt.Errorf("not found: %s", name)
	}

	is := rs.Primary
	if is == nil {
		return nil, fmt.Errorf("no primary instance: %s", name)
	}

	return is, nil
}

// AccountID returns the account ID of Provider
// Must be used within a resource.TestCheckFunc
func AccountID(ctx context.Context) string {
	return ProviderAccountID(ctx, Provider)
}

func Region() string {
	if endpoint := os.Getenv("CTYUNOOS_ENDPOINT"); endpoint != "" {
		if region := names.RegionForEndpoint(names.S3, endpoint); region != "" {
			return region
		}
	}

	return envvar.GetWithDefault(envvar.DefaultRegion, endpoints.UsWest2RegionID)
}

func AlternateRegion() string {
	if region := os.Getenv(envvar.OOSAlternateRegion); region != "" {
		return region
	}

	return envvar.GetWithDefault(envvar.AlternateRegion, endpoints.UsEast1RegionID)
}

func OOSMetadataLocation() string {
	return envvar.GetWithDefault(envvar.OOSMetadataLocation, "ShangHai")
}

func ThirdRegion() string {
	return envvar.GetWithDefault(envvar.ThirdRegion, endpoints.UsEast2RegionID)
}

func FourthRegion() string {
	return envvar.GetWithDefault(envvar.FourthRegion, endpoints.UsWest1RegionID)
}

func Partition() string {
	return names.PartitionForRegion(Region()).ID()
}

func PartitionRegions() []string {
	return RegionsInPartition(Partition())
}

func PartitionDNSSuffix() string {
	return names.PartitionForRegion(Region()).DNSSuffix()
}

func PartitionReverseDNSPrefix() string {
	return dns.Reverse(PartitionDNSSuffix())
}

func alternateRegionPartition() string {
	return names.PartitionForRegion(AlternateRegion()).ID()
}

func thirdRegionPartition() string {
	return names.PartitionForRegion(ThirdRegion()).ID()
}

func fourthRegionPartition() string {
	return names.PartitionForRegion(FourthRegion()).ID()
}

func PreCheckAlternateAccount(t *testing.T) {
	t.Helper()

	envvar.SkipIfAllEmpty(t, []string{envvar.AlternateAccessKeyId}, "static credentials for running acceptance testing in alternate OOS account")

	if os.Getenv(envvar.AlternateAccessKeyId) != "" {
		envvar.SkipIfEmpty(t, envvar.AlternateSecretAccessKey, "static credentials value when using "+envvar.AlternateAccessKeyId)
	}
}

func PreCheckThirdAccount(t *testing.T) {
	t.Helper()

	envvar.SkipIfAllEmpty(t, []string{envvar.ThirdAccessKeyId}, "static credentials for running acceptance testing in third OOS account")

	if os.Getenv(envvar.ThirdAccessKeyId) != "" {
		envvar.SkipIfEmpty(t, envvar.ThirdSecretAccessKey, "static credentials value when using "+envvar.ThirdAccessKeyId)
	}
}

// add fourth region
func PreCheckFourthAccount(t *testing.T) {
	t.Helper()

	envvar.SkipIfAllEmpty(t, []string{envvar.FourthAccessKeyId}, "static credentials for running acceptance testing in fourth OOS account")

	if os.Getenv(envvar.FourthAccessKeyId) != "" {
		envvar.SkipIfEmpty(t, envvar.FourthSecretAccessKey, "static credentials value when using "+envvar.FourthAccessKeyId)
	}
}

func PreCheckPartitionHasService(t *testing.T, serviceID string) {
	t.Helper()

	if partition, ok := endpoints.PartitionForRegion(endpoints.DefaultPartitions(), Region()); ok {
		if _, ok := partition.Services()[serviceID]; !ok {
			t.Skipf("skipping tests; partition %s does not support %s service", partition.ID(), serviceID)
		}
	}
}

func PreCheckMultipleRegion(t *testing.T, regions int) {
	t.Helper()

	if len(PartitionRegions()) <= 1 {
		t.Skipf("Skipping multiple region test as 1 or fewer regions detected in partion (%s)", Partition())
	}

	if Region() == AlternateRegion() {
		t.Fatalf("%s and %s must be set to different values for acceptance tests", envvar.DefaultRegion, envvar.AlternateRegion)
	}

	if Partition() != alternateRegionPartition() {
		t.Fatalf("%s partition (%s) does not match %s partition (%s)", envvar.AlternateRegion, alternateRegionPartition(), envvar.DefaultRegion, Partition())
	}

	if regions >= 3 {
		if thirdRegionPartition() == endpoints.AwsUsGovPartitionID || Partition() == endpoints.AwsUsGovPartitionID {
			t.Skipf("wanted %d regions, partition (%s) only has 2 regions", regions, Partition())
		}

		if Region() == ThirdRegion() {
			t.Fatalf("%s and %s must be set to different values for acceptance tests", envvar.DefaultRegion, envvar.ThirdRegion)
		}

		if AlternateRegion() == ThirdRegion() {
			t.Fatalf("%s and %s must be set to different values for acceptance tests", envvar.AlternateRegion, envvar.ThirdRegion)
		}

		if Partition() != thirdRegionPartition() {
			t.Fatalf("%s partition (%s) does not match %s partition (%s)", envvar.ThirdRegion, thirdRegionPartition(), envvar.DefaultRegion, Partition())
		}

		if regions == 4 {
			if fourthRegionPartition() == endpoints.AwsUsGovPartitionID || Partition() == endpoints.AwsUsGovPartitionID {
				t.Skipf("wanted %d regions, partition (%s) only has 2 regions", regions, Partition())
			}

			if Region() == FourthRegion() {
				t.Fatalf("%s and %s must be set to different values for acceptance tests", envvar.DefaultRegion, envvar.FourthRegion)
			}

			if AlternateRegion() == FourthRegion() {
				t.Fatalf("%s and %s must be set to different values for acceptance tests", envvar.AlternateRegion, envvar.FourthRegion)
			}

			if ThirdRegion() == FourthRegion() {
				t.Fatalf("%s and %s must be set to different values for acceptance tests", envvar.ThirdRegion, envvar.FourthRegion)
			}

			if Partition() != fourthRegionPartition() {
				t.Fatalf("%s partition (%s) does not match %s partition (%s)", envvar.FourthRegion, fourthRegionPartition(), envvar.DefaultRegion, Partition())
			}
		}
	}

	if partition, ok := endpoints.PartitionForRegion(endpoints.DefaultPartitions(), Region()); ok {
		if len(partition.Regions()) < regions {
			t.Skipf("skipping tests; partition includes %d regions, %d expected", len(partition.Regions()), regions)
		}
	}
}

// PreCheckRegion checks that the test region is one of the specified AWS Regions.
func PreCheckRegion(t *testing.T, regions ...string) {
	t.Helper()

	if curr := Region(); !slices.Contains(regions, curr) {
		t.Skipf("skipping tests; %s (%s) not supported. Supported: [%s]", envvar.DefaultRegion, curr, strings.Join(regions, ", "))
	}
}

// PreCheckRegionNot checks that the test region is not one of the specified AWS Regions.
func PreCheckRegionNot(t *testing.T, regions ...string) {
	t.Helper()

	if curr := Region(); slices.Contains(regions, curr) {
		t.Skipf("skipping tests; %s (%s) not supported", envvar.DefaultRegion, curr)
	}
}

// PreCheckAlternateRegion checks that the alternate test region is one of the specified AWS Regions.
func PreCheckAlternateRegion(t *testing.T, regions ...string) {
	t.Helper()

	if curr := AlternateRegion(); !slices.Contains(regions, curr) {
		t.Skipf("skipping tests; %s (%s) not supported. Supported: [%s]", envvar.AlternateRegion, curr, strings.Join(regions, ", "))
	}
}

// PreCheckThirdRegion checks that the third test region is one of the specified AWS Regions.
func PreCheckThirdRegion(t *testing.T, regions ...string) {
	t.Helper()

	if curr := ThirdRegion(); !slices.Contains(regions, curr) {
		t.Skipf("skipping tests; %s (%s) not supported. Supported: [%s]", envvar.ThirdRegion, curr, strings.Join(regions, ", "))
	}
}

// PreCheckFourthRegion checks that the fourth test region is one of the specified AWS Regions.
func PreCheckFourthRegion(t *testing.T, regions ...string) {
	t.Helper()

	if curr := FourthRegion(); !slices.Contains(regions, curr) {
		t.Skipf("skipping tests; %s (%s) not supported. Supported: [%s]", envvar.FourthRegion, curr, strings.Join(regions, ", "))
	}
}

// PreCheckPartition checks that the test partition is the specified partition.
func PreCheckPartition(t *testing.T, partition string) {
	t.Helper()

	if curr := Partition(); curr != partition {
		t.Skipf("skipping tests; current partition (%s) does not equal %s", curr, partition)
	}
}

// PreCheckPartitionNot checks that the test partition is not one of the specified partitions.
func PreCheckPartitionNot(t *testing.T, partitions ...string) {
	t.Helper()

	if curr := Partition(); slices.Contains(partitions, curr) {
		t.Skipf("skipping tests; current partition (%s) not supported", curr)
	}
}

// PreCheckOrganizationsEnabledServicePrincipal is an AWS Organizations check
// that does not apply to OOS object storage. The underlying service package was
// removed during project slimming; this stub keeps dependent acceptance tests
// compiling while always skipping at runtime.
func PreCheckOrganizationsEnabledServicePrincipal(ctx context.Context, t *testing.T, servicePrincipalName string) {
	t.Helper()

	t.Skip("AWS Organizations is not supported by the OOS provider")
}

// PreCheckOrganizationManagementAccount is an AWS Organizations check that does
// not apply to OOS object storage. See PreCheckOrganizationsEnabledServicePrincipal.
func PreCheckOrganizationManagementAccount(ctx context.Context, t *testing.T) {
	t.Helper()

	t.Skip("AWS Organizations is not supported by the OOS provider")
}

// PreCheckRegionOptIn is an AWS Account region opt-in check that does not apply
// to OOS object storage. See PreCheckOrganizationsEnabledServicePrincipal.
func PreCheckRegionOptIn(ctx context.Context, t *testing.T, region string) {
	t.Helper()

	t.Skip("AWS Region opt-in is not supported by the OOS provider")
}

// PreCheckSSOAdminInstances is an AWS SSO Admin check that does not apply to OOS
// object storage. The underlying service package was removed during project
// slimming; this stub keeps dependent acceptance tests compiling while always
// skipping at runtime.
func PreCheckSSOAdminInstances(ctx context.Context, t *testing.T) {
	t.Helper()

	t.Skip("AWS SSO Admin is not supported by the OOS provider")
}

func PreCheckHasIAMRole(ctx context.Context, t *testing.T, roleName string) {
	t.Helper()

	_, err := tfiam.FindRoleByName(ctx, Provider.Meta().(*conns.AWSClient).IAMClient(ctx), roleName)

	if retry.NotFound(err) {
		t.Skipf("skipping acceptance test: required IAM role %q not found", roleName)
	}

	if PreCheckSkipError(err) {
		t.Skipf("skipping acceptance test: %s", err)
	}

	if err != nil {
		t.Fatalf("unexpected PreCheck error: %s", err)
	}
}

func PreCheckIAMServiceLinkedRole(ctx context.Context, t *testing.T, pathPrefix string) {
	t.Helper()

	PreCheckIAMServiceLinkedRoleWithProvider(ctx, t, DefaultProviderFunc, pathPrefix)
}

func PreCheckIAMServiceLinkedRoleWithProvider(ctx context.Context, t *testing.T, providerF ProviderFunc, pathPrefix string) {
	t.Helper()

	conn := providerF().Meta().(*conns.AWSClient).IAMClient(ctx)
	input := iam.ListRolesInput{
		PathPrefix: aws.String(pathPrefix),
	}
	var roleFound bool

	pages := iam.NewListRolesPaginator(conn, &input)
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if PreCheckSkipError(err) {
			t.Skipf("skipping tests: %s", err)
		}
		if err != nil {
			t.Fatalf("listing IAM roles: %s", err)
		}

		if len(page.Roles) > 0 {
			roleFound = true
			break
		}
	}

	if !roleFound {
		t.Skipf("skipping tests; missing IAM service-linked role %s. Please create the role and retry", pathPrefix)
	}
}

// PreCheckOutpostsOutposts is an AWS Outposts check that does not apply to OOS
// object storage. The underlying service package was removed during project
// slimming; this stub keeps dependent acceptance tests compiling while always
// skipping at runtime.
func PreCheckOutpostsOutposts(ctx context.Context, t *testing.T) {
	t.Helper()

	t.Skip("AWS Outposts is not supported by the OOS provider")
}

func RegionProviderFunc(ctx context.Context, region string, providers *[]*schema.Provider) ProviderFunc {
	return func() *schema.Provider {
		if region == "" {
			log.Println("[DEBUG] No region given")
			return nil
		}
		if providers == nil {
			log.Println("[DEBUG] No providers given")
			return nil
		}

		log.Printf("[DEBUG] Checking providers for AWS region: %s", region)
		for _, provo := range *providers {
			// Ignore if Meta is empty, this can happen for validation providers
			if provo == nil || provo.Meta() == nil {
				log.Printf("[DEBUG] Skipping empty provider")
				continue
			}

			// Ignore if Meta is not conns.AWSClient, this will happen for other providers
			client, ok := provo.Meta().(*conns.AWSClient)
			if !ok {
				log.Printf("[DEBUG] Skipping non-AWS provider")
				continue
			}

			clientRegion := client.Region(ctx)
			log.Printf("[DEBUG] Checking AWS provider region %q against %q", clientRegion, region)
			if clientRegion == region {
				log.Printf("[DEBUG] Found AWS provider with region: %s", region)
				return provo
			}
		}

		log.Printf("[DEBUG] No suitable provider found for %q in %d providers", region, len(*providers))
		return nil
	}
}

func NamedProviderFunc(name string, providers map[string]*schema.Provider) ProviderFunc {
	return func() *schema.Provider {
		return NamedProvider(name, providers)
	}
}

func NamedProvider(name string, providers map[string]*schema.Provider) *schema.Provider {
	if name == "" {
		log.Printf("[ERROR] No name passed")
	}

	p, ok := providers[name]
	if !ok {
		log.Printf("[ERROR] No provider named %q found", name)
		return nil
	}

	return p
}

// NewTestResourceContext bootstaps the testing context for a given resource type
func NewTestResourceContext(ctx context.Context, rType, region string) context.Context {
	return conns.NewResourceContext(ctx, "", "", rType, region)
}

func DeleteResource(ctx context.Context, resource *schema.Resource, d *schema.ResourceData, meta any) error {
	if resource.DeleteContext != nil || resource.DeleteWithoutTimeout != nil {
		var diags diag.Diagnostics

		if resource.DeleteContext != nil {
			diags = resource.DeleteContext(ctx, d, meta) // nosemgrep:ci.semgrep.migrate.direct-CRUD-calls
		} else {
			diags = resource.DeleteWithoutTimeout(ctx, d, meta) // nosemgrep:ci.semgrep.migrate.direct-CRUD-calls
		}

		for i := range diags {
			if diags[i].Severity == diag.Error {
				return fmt.Errorf("deleting resource: %s", diags[i].Summary)
			}
		}

		return nil
	}

	return resource.Delete(d, meta) // nosemgrep:ci.semgrep.migrate.direct-CRUD-calls
}

type providerMetaFunc func(ctx context.Context) *conns.AWSClient

func providerMeta(t *testing.T) providerMetaFunc {
	return func(ctx context.Context) *conns.AWSClient {
		return ProviderMeta(ctx, t)
	}
}

func CheckSDKResourceDisappears(ctx context.Context, t *testing.T, resource *schema.Resource, n string) resource.TestCheckFunc {
	return checkSDKResourceDisappears(ctx, providerMeta(t), resource, n)
}

func CheckSDKResourceDisappearsWithProvider(ctx context.Context, provider *schema.Provider, resource *schema.Resource, n string) resource.TestCheckFunc {
	return checkSDKResourceDisappears(ctx, func(context.Context) *conns.AWSClient { return provider.Meta().(*conns.AWSClient) }, resource, n)
}

func checkSDKResourceDisappears(ctx context.Context, providerMetaF providerMetaFunc, resource *schema.Resource, n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("resource not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("resource ID missing: %s", n)
		}

		// Bootstrap context for resource type
		ctx = NewTestResourceContext(ctx, rs.Type, rs.Primary.Attributes[names.AttrRegion])

		var state terraformsdk.InstanceState
		err := mapstructure.Decode(rs.Primary, &state)
		if err != nil {
			return err
		}

		return DeleteResource(ctx, resource, resource.Data(&state), providerMetaF(ctx))
	}
}

type TestCheckWithProviderFunc func(*terraform.State, *schema.Provider) error

func CheckWithProviders(f TestCheckWithProviderFunc, providers *[]*schema.Provider) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		numberOfProviders := len(*providers)
		for i, provo := range *providers {
			if provo.Meta() == nil {
				log.Printf("[DEBUG] Skipping empty provider %d (total: %d)", i, numberOfProviders)
				continue
			}
			log.Printf("[DEBUG] Calling check with provider %d (total: %d)", i, numberOfProviders)
			if err := f(s, provo); err != nil {
				return err
			}
		}
		return nil
	}
}

func CheckWithNamedProviders(f TestCheckWithProviderFunc, providers map[string]*schema.Provider) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		numberOfProviders := len(providers)
		for k, provo := range providers {
			if provo.Meta() == nil {
				log.Printf("[DEBUG] Skipping empty provider %q (total: %d)", k, numberOfProviders)
				continue
			}
			log.Printf("[DEBUG] Calling check with provider %q (total: %d)", k, numberOfProviders)
			if err := f(s, provo); err != nil {
				return err
			}
		}
		return nil
	}
}

type TestCheckWithRegionFunc func(*terraform.State, string) error

func CheckWithRegions(f TestCheckWithRegionFunc, regions ...string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for _, region := range regions {
			if err := f(s, region); err != nil {
				return err
			}
		}
		return nil
	}
}

func ErrorCheckSequence(funcs ...resource.ErrorCheckFunc) resource.ErrorCheckFunc {
	return func(err error) error {
		for _, f := range funcs {
			err = f(err)
		}
		return err
	}
}

// ErrorCheckSkipMessagesContaining skips tests based on error messages that contain one of the specified needles.
func ErrorCheckSkipMessagesContaining(t *testing.T, needles ...string) resource.ErrorCheckFunc {
	t.Helper()

	return func(err error) error {
		if err == nil {
			return nil
		}

		for _, needle := range needles {
			errorMessage := err.Error()
			if strings.Contains(errorMessage, needle) {
				t.Skipf("skipping test for %s/%s: %s", Partition(), Region(), errorMessage)
			}
		}

		return err
	}
}

// ErrorCheckSkipMessagesMatches skips tests based on error messages that match one of the specified regular expressions.
func ErrorCheckSkipMessagesMatches(t *testing.T, rs ...*regexp.Regexp) resource.ErrorCheckFunc {
	t.Helper()

	return func(err error) error {
		if err == nil {
			return nil
		}

		for _, r := range rs {
			errorMessage := err.Error()
			if r.MatchString(errorMessage) {
				t.Skipf("skipping test for %s/%s: %s", Partition(), Region(), errorMessage)
			}
		}

		return err
	}
}

type ServiceErrorCheckFunc func(*testing.T) resource.ErrorCheckFunc

var serviceErrorCheckFuncs map[string]ServiceErrorCheckFunc

func RegisterServiceErrorCheckFunc(serviceID string, f ServiceErrorCheckFunc) {
	if serviceErrorCheckFuncs == nil {
		serviceErrorCheckFuncs = make(map[string]ServiceErrorCheckFunc)
	}

	if _, ok := serviceErrorCheckFuncs[serviceID]; ok {
		// already registered
		panic(fmt.Sprintf("Cannot re-register a service! ServiceErrorCheckFunc exists for %s", serviceID)) //lintignore:R009
	}

	serviceErrorCheckFuncs[serviceID] = f
}

func ErrorCheck(t *testing.T, serviceIDs ...string) resource.ErrorCheckFunc {
	t.Helper()

	return func(err error) error {
		if err == nil {
			return nil
		}

		for _, serviceID := range serviceIDs {
			if f, ok := serviceErrorCheckFuncs[serviceID]; ok {
				ef := f(t)
				err = ef(err)
			}

			if err == nil {
				break
			}
		}

		if errorCheckCommon(err) {
			t.Skipf("skipping test for %s/%s: %s", Partition(), Region(), err.Error())
		}

		return err
	}
}

// NOTE: This function cannot use the standard tfawserr helpers
// as it is receiving error strings from the SDK testing framework,
// not actual error types from the resource logic.
func errorCheckCommon(err error) bool {
	if strings.Contains(err.Error(), "is not supported in this") {
		return true
	}

	if strings.Contains(err.Error(), "is currently not supported") {
		return true
	}

	if strings.Contains(err.Error(), "InvalidAction") {
		return true
	}

	if strings.Contains(err.Error(), "Unknown operation") {
		return true
	}

	if strings.Contains(err.Error(), "UnknownOperationException") {
		return true
	}

	if strings.Contains(err.Error(), "UnsupportedOperation") {
		return true
	}

	return false
}

// Check service API call error for reasons to skip acceptance testing
// These include missing API endpoints and unsupported API calls
func PreCheckSkipError(err error) bool {
	// GovCloud has endpoints that respond with (no message provided after the error code):
	// AccessDeniedException:
	// Ignore these API endpoints that exist but are not officially enabled
	if tfawserr.ErrCodeEquals(err, "AccessDeniedException") {
		return true
	}
	// Ignore missing API endpoints
	if tfawserr.ErrMessageContains(err, "RequestError", "send request failed") {
		return true
	}
	// Ignore unsupported API calls
	if tfawserr.ErrCodeEquals(err, "UnknownOperationException") {
		return true
	}
	if tfawserr.ErrCodeEquals(err, "UnsupportedOperation") {
		return true
	}
	if tfawserr.ErrMessageContains(err, "InvalidInputException", "Unknown operation") {
		return true
	}
	if tfawserr.ErrMessageContains(err, "InvalidAction", "is not valid") {
		return true
	}
	if tfawserr.ErrMessageContains(err, "InvalidAction", "Unavailable Operation") {
		return true
	}
	// ignore when not authorized to call API from account
	if tfawserr.ErrCodeEquals(err, "ForbiddenException") {
		return true
	}
	// sts
	if tfawserr.ErrCodeEquals(err, "OutboundWebIdentityFederationDisabledException") {
		return true
	}
	// Ignore missing API endpoints
	if errs.IsA[*net.DNSError](err) {
		return true
	}
	return false
}

func PreCheckAssumeRoleARN(t *testing.T) {
	t.Helper()

	envvar.SkipIfEmpty(t, envvar.AccAssumeRoleARN, "Amazon Resource Name (ARN) of existing IAM Role to assume for testing restricted permissions")
}

type domainName string

// The top level domain ".test" is reserved by IANA for testing purposes:
// https://datatracker.ietf.org/doc/html/rfc6761
const domainNameTestTopLevelDomain domainName = "test"

// RandomSubdomain creates a random three-level domain name in the form
// "<random>.<random>.test"
// The top level domain ".test" is reserved by IANA for testing purposes:
// https://datatracker.ietf.org/doc/html/rfc6761
func RandomSubdomain() string {
	return string(RandomDomain().RandomSubdomain())
}

// RandomDomainName creates a random two-level domain name in the form
// "<random>.test"
// The top level domain ".test" is reserved by IANA for testing purposes:
// https://datatracker.ietf.org/doc/html/rfc6761
func RandomDomainName() string {
	return string(RandomDomain())
}

// RandomFQDomainName creates a random fully-qualified two-level domain name in the form
// "<random>.test."
// The top level domain ".test" is reserved by IANA for testing purposes:
// https://datatracker.ietf.org/doc/html/rfc6761
func RandomFQDomainName() string {
	return string(RandomDomain().FQDN())
}

func (d domainName) Subdomain(name string) domainName {
	return domainName(fmt.Sprintf("%s.%s", name, d))
}

func (d domainName) RandomSubdomain() domainName {
	return d.Subdomain(sdkacctest.RandString(8)) //nolint:mnd // standard length of 8
}

func (d domainName) FQDN() domainName {
	return domainName(fmt.Sprintf("%s.", d))
}

func (d domainName) String() string {
	return string(d)
}

func RandomDomain() domainName {
	return domainNameTestTopLevelDomain.RandomSubdomain()
}

// DefaultEmailAddress is the default email address to set as a
// resource or data source parameter for acceptance tests.
const DefaultEmailAddress = "no-reply@hashicorp.com"

// RandomEmailAddress generates a random email address in the form
// "tf-acc-test-<random>@<domain>"
func RandomEmailAddress(domainName string) string {
	return fmt.Sprintf("%s@%s", sdkacctest.RandomWithPrefix(ResourcePrefix), domainName)
}

const (
	// ACM domain names cannot be longer than 64 characters
	// Other resources, e.g. Cognito User Pool Domains, limit this to 63
	acmCertificateDomainMaxLen = 63

	acmRandomSubDomainPrefix    = "tf-acc-"
	acmRandomSubDomainPrefixLen = len(acmRandomSubDomainPrefix)

	// Max length (63)
	// Subtract "tf-acc-" prefix (7)
	// Subtract "." between prefix and root domain (1)
	acmRandomSubDomainRemainderLen = acmCertificateDomainMaxLen - acmRandomSubDomainPrefixLen - 1
)

func ACMCertificateDomainFromEnv(t *testing.T) string {
	t.Helper()

	rootDomain := os.Getenv("ACM_CERTIFICATE_ROOT_DOMAIN")

	if rootDomain == "" {
		t.Skip(
			"Environment variable ACM_CERTIFICATE_ROOT_DOMAIN is not set. " +
				"For DNS validation requests, this domain must be publicly " +
				"accessible and configurable via Route53 during the testing. " +
				"For email validation requests, you must have access to one of " +
				"the five standard email addresses used (admin|administrator|" +
				"hostmaster|postmaster|webmaster)@domain or one of the WHOIS " +
				"contact addresses.")
	}

	if len(rootDomain) > acmRandomSubDomainRemainderLen {
		t.Skipf(
			"Environment variable ACM_CERTIFICATE_ROOT_DOMAIN is too long. "+
				"The domain must be %d characters or shorter to allow for "+
				"subdomain randomization in the testing.", acmRandomSubDomainRemainderLen)
	}

	return rootDomain
}

// ACM domain names cannot be longer than 64 characters
// Other resources, e.g. Cognito User Pool Domains, limit this to 63
func ACMCertificateRandomSubDomain(rootDomain string) string {
	return fmt.Sprintf(
		acmRandomSubDomainPrefix+"%s.%s",
		sdkacctest.RandString(acmRandomSubDomainRemainderLen-len(rootDomain)),
		rootDomain)
}

// PreCheckAPIGatewayTypeEDGE checks if endpoint config type EDGE can be used in a test and skips test if not (i.e., not in standard partition).
func PreCheckAPIGatewayTypeEDGE(t *testing.T) {
	t.Helper()

	if Partition() != endpoints.AwsPartitionID {
		t.Skipf("skipping test; Endpoint Configuration type EDGE is not supported in this partition (%s)", Partition())
	}
}

func CheckCallerIdentityAccountID(ctx context.Context, n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("can't find AccountID resource: %s", n)
		}

		expected := Provider.Meta().(*conns.AWSClient).AccountID(ctx)
		if rs.Primary.Attributes["account_id"] != expected {
			return fmt.Errorf("incorrect Account ID: expected %q, got %q", expected, rs.Primary.Attributes["account_id"])
		}

		if rs.Primary.Attributes["user_id"] == "" {
			return fmt.Errorf("user_id expected to not be nil")
		}

		if rs.Primary.Attributes["arn"] == "" {
			return fmt.Errorf("attribute ARN expected to not be nil")
		}

		return nil
	}
}

func CheckResourceAttrGreaterThanValue(n, key string, val int) resource.TestCheckFunc {
	return resource.TestCheckResourceAttrWith(n, key, func(value string) error {
		v, err := strconv.Atoi(value)

		if err != nil {
			return err
		}

		if v <= val {
			return fmt.Errorf("got %d, want > %d", v, val)
		}

		return nil
	})
}

func CheckResourceAttrGreaterThanOrEqualValue(n, key string, val int) resource.TestCheckFunc {
	return resource.TestCheckResourceAttrWith(n, key, func(value string) error {
		v, err := strconv.Atoi(value)

		if err != nil {
			return err
		}

		if v < val {
			return fmt.Errorf("got %d, want >= %d", v, val)
		}

		return nil
	})
}

func CheckResourceAttrIsJSONString(n, key string) resource.TestCheckFunc {
	return resource.TestCheckResourceAttrWith(n, key, func(value string) error {
		var m map[string]*json.RawMessage

		if err := json.Unmarshal([]byte(value), &m); err != nil {
			return err
		}

		if len(m) == 0 {
			return errors.New(`empty JSON string`)
		}

		return nil
	})
}

// SkipIfEnvVarNotSet skips the current test if the specified environment variable is not set.
// The variable's value is returned.
func SkipIfEnvVarNotSet(t *testing.T, key string) string {
	t.Helper()
	return envvar.SkipIfEmpty(t, key, "")
}

// SkipIfExeNotOnPath skips the current test if the specified executable is not found in the directories named by the PATH environment variable.
// The absolute path to the executable is returned.
func SkipIfExeNotOnPath(t *testing.T, file string) string {
	t.Helper()

	v, err := exec.LookPath(file)
	if err != nil {
		t.Skipf("File %s not found on PATH, skipping test: %s", v, err)
	}
	return v
}

// SkipIfNotRunningAcceptanceTests skips the current test if it's not an acceptance test (TF_ACC is set in the environment).
func SkipIfNotRunningAcceptanceTests(t *testing.T) {
	t.Helper()
	SkipIfEnvVarNotSet(t, resource.EnvTfAcc)
}

// RunSerialTests1Level runs test cases in parallel, optionally sleeping between each.
func RunSerialTests1Level(t *testing.T, testCases map[string]func(*testing.T), d time.Duration) {
	t.Helper()
	SkipIfNotRunningAcceptanceTests(t)

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tc(t)
			time.Sleep(d)
		})
	}
}

// RunSerialTests2Levels runs test cases in parallel, optionally sleeping between each.
func RunSerialTests2Levels(t *testing.T, testCases map[string]map[string]func(*testing.T), d time.Duration) {
	t.Helper()

	for group, m := range testCases {
		t.Run(group, func(t *testing.T) {
			RunSerialTests1Level(t, m, d)
		})
	}
}

// RunLimitedConcurrencyTests2Levels runs test cases with concurrency limited via `semaphore`.
func RunLimitedConcurrencyTests2Levels(t *testing.T, semaphore tfsync.Semaphore, testCases map[string]map[string]func(*testing.T, tfsync.Semaphore)) {
	t.Helper()

	for group, m := range testCases {
		for name, tc := range m {
			t.Run(fmt.Sprintf("%s_%s", group, name), func(t *testing.T) {
				t.Cleanup(func() {
					if os.Getenv(resource.EnvTfAcc) != "" {
						semaphore.Notify()
					}
				})
				tc(t, semaphore)
			})
		}
	}
}

func ExpectErrorAttrAtLeastOneOf(attrs ...string) *regexp.Regexp {
	return regexache.MustCompile(fmt.Sprintf("one of\\s+`%s`\\s+must be specified", strings.Join(attrs, ",")))
}

func ExpectErrorAttrMinItems(attr string, expected, actual int) *regexp.Regexp {
	return regexache.MustCompile(fmt.Sprintf(`Attribute %s requires %d\s+item minimum, but config has only %d declared`, attr, expected, actual))
}

func ListOfStrings[E ~string](s ...E) string {
	return strings.Join(tfslices.ApplyToAll(s, func(e E) string {
		return strconv.Quote(string(e))
	}), ", ")
}
