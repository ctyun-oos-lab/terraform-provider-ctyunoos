// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

//go:build generate

package main

import (
	_ "embed"
	"path/filepath"
	"strings"

	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/generate/common"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names/data"
	namesgen "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names/generate"
	"github.com/hashicorp/aws-sdk-go-base/v2/endpoints"
)

const (
	relativePath = `../../service`
	filename     = `service_endpoints_gen_test.go`
)

func main() {
	g := common.NewGenerator()

	services, err := data.ReadAllServiceData()

	if err != nil {
		g.Fatalf("error reading service data: %s", err)
	}

	for _, l := range services {
		packageName := l.ProviderPackage()

		switch packageName {
		case "arcregionswitch", // Resolver modifies URL
			"cloudfrontkeyvaluestore", // Endpoint includes account ID
			"codecatalyst",            // Bearer auth token needs special handling
			"location",                // Resolver modifies URL
			"mwaa",                    // Resolver modifies URL
			"neptunegraph",            // EndpointParameters has an additional parameter, ApiType
			"paymentcryptography",     // Resolver modifies URL
			"route53profiles",         // Resolver modifies URL
			"simpledb",                // AWS SDK for Go v1
			"timestreamwrite":         // Uses endpoint discovery
			continue
		}

		if l.Exclude() {
			continue
		}

		if l.NotImplemented() && !l.EndpointOnly() {
			continue
		}

		g.Infof("Generating internal/service/%s/%s", packageName, filename)

		serviceEndpointKey := l.AWSConfigParameter()
		td := TemplateData{
			HumanFriendly:      l.HumanFriendly(),
			PackageName:        packageName,
			GoPackage:          l.GoPackageName(),
			ProviderNameUpper:  l.ProviderNameUpper(),
			Region:             "us-west-2",
			APICall:            l.EndpointAPICall(),
			APICallParams:      l.EndpointAPIParams(),
			AwsEnvVar:          l.AWSServiceEnvVar(),
			ConfigParameter:    namesgen.ConstOrQuote(serviceEndpointKey),
			ServiceEndpointKey: serviceEndpointKey,
			CustomEnvVar:       l.CustomEnvVar(),
			DeprecatedEnvVar:   l.DeprecatedEnvVar(),
			TFAWSEnvVar:        l.TFAWSEnvVar(),
			Aliases:            l.Aliases(),
			OverrideRegion:     l.EndpointRegionOverrides()[endpoints.AwsPartitionID],
			DefaultEndpoint:    l.DefaultEndpoint(),
			CheckSigningRegion: packageName != names.CloudTrail,
			OOSTestRegion:      "cn1",
			OOSTestEndpoint:    strings.ReplaceAll(l.EndpointTemplate(), "{region}", "cn1") + "/",
			OOSEnvRegion:       "cn2",
			OOSEnvEndpoint:     strings.ReplaceAll(l.EndpointTemplate(), "{region}", "cn2") + "/",
		}
		if strings.Contains(td.APICallParams, "awstypes") {
			td.ImportAwsTypes = true
		}

		if td.OverrideRegion == "us-west-2" {
			td.Region = "us-east-1"
		}

		switch packageName {
		// TODO: This case should be handled in service data
		case "costoptimizationhub", "cur", "globalaccelerator", "mpa", "notifications", "notificationscontacts", "route53domains", "route53recoverycontrolconfig", "route53recoveryreadiness", "uxc":
			td.OverrideRegionRegionalEndpoint = true

		case "chatbot":
			// chatbot is available in `us-east-2`, `us-west-2`, `eu-west-1`, and `ap-southeast-1`
			// If the service is called from any other region, it defaults to `us-west-2`
			td.Region = "us-east-1"
			td.OverrideRegion = "us-west-2"
			td.OverrideRegionRegionalEndpoint = true
		}

		if packageName == names.IAM {
			td.ParseSigningRegionFromEndpoint = true
		}

		if td.APICall == "" {
			g.Fatalf("error generating service endpoint tests: package %q missing APICall", packageName)
		}

		d := g.NewGoFileDestination(filepath.Join(relativePath, packageName, filename))

		if err := d.BufferTemplate("serviceendpointtests", tmpl, td); err != nil {
			g.Fatalf("error generating service endpoint tests: %s", err)
		}

		if err := d.Write(); err != nil {
			g.Fatalf("generating file (internal/service/%s/%s): %s", packageName, filename, err)
		}
	}
}

type TemplateData struct {
	HumanFriendly                     string
	PackageName                       string
	GoPackage                         string
	ProviderNameUpper                 string
	Region                            string
	APICall                           string
	APICallParams                     string
	AwsEnvVar                         string
	ConfigParameter                   string
	ServiceEndpointKey                string
	CustomEnvVar                      string
	DeprecatedEnvVar                  string
	TFAWSEnvVar                       string
	V1NameResolverNeedsUnknownService bool
	Aliases                           []string
	ImportAwsTypes                    bool
	OverrideRegion                    string
	// DefaultEndpoint is the endpoint injected by the service resolver when the
	// user does not configure one (used by S3-compatible storage such as OOS).
	DefaultEndpoint string
	// CloudTrail uses an OOS-specific signer that does not publish its signing
	// region through the AWS SDK middleware context.
	CheckSigningRegion bool
	OOSTestRegion      string
	OOSTestEndpoint    string
	OOSEnvRegion       string
	OOSEnvEndpoint     string
	// The provider switches to the required region, but the service has a regional endpoint
	OverrideRegionRegionalEndpoint bool
	ParseSigningRegionFromEndpoint bool
}

//go:embed file.gtpl
var tmpl string
