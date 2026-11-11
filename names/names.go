// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

// Package names provides constants for OOS service names that are used as keys
// for the endpoints slice in internal/conns/conns.go. The package also exposes
// access to data found in the data/names_data.hcl file, which provides additional
// service-related name information.
//
// Consumers of the names package include the conns package
// (internal/conn/conns.go), the provider package
// (internal/provider/provider.go), and the provider generators.
//
// It is very important that information in the data/names_data.hcl be exactly
// correct because the CTYUN OOS Provider relies on the information to
// function correctly.
package names

import (
	"fmt"
	"log"
	"net"
	"regexp"
	"slices"
	"strings"

	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names/data"
	"github.com/hashicorp/aws-sdk-go-base/v2/endpoints"
)

// PartitionForRegion returns the partition for the given Region.
// Returns the empty partition if the Region is empty.
// Returns the standard partition if no known partition includes the Region.
func PartitionForRegion(region string) endpoints.Partition {
	if region == "" {
		return endpoints.Partition{}
	}

	if partition, ok := endpoints.PartitionForRegion(endpoints.DefaultPartitions(), region); ok {
		return partition
	}

	return PartitionForRegion(endpoints.UsEast1RegionID)
}

// Type ServiceDatum corresponds closely to attributes and blocks in `data/names_data.hcl` and are
// described in detail in README.md.
type serviceDatum struct {
	aliases           []string
	brand             string
	humanFriendly     string
	providerNameUpper string
	endpointTemplate  string
}

// serviceData key is the provider service package
var serviceData map[string]serviceDatum

func init() {
	serviceData = make(map[string]serviceDatum)

	// Data from names_data.hcl
	if err := readHCLIntoServiceData(); err != nil {
		log.Fatalf("reading HCL into service data: %s", err)
	}
}

func readHCLIntoServiceData() error {
	// names_data.hcl is dynamically embedded so changes, additions should be made
	// there also

	d, err := data.ReadAllServiceData()
	if err != nil {
		return fmt.Errorf("reading HCL into service data: %w", err)
	}

	for _, l := range d {
		if l.Exclude() {
			continue
		}

		if l.NotImplemented() && !l.EndpointOnly() {
			continue
		}

		p := l.ProviderPackage()

		sd := serviceDatum{
			brand:             l.Brand(),
			humanFriendly:     l.HumanFriendly(),
			providerNameUpper: l.ProviderNameUpper(),
			endpointTemplate:  l.EndpointTemplate(),
		}

		a := []string{p}

		if len(l.Aliases()) > 0 {
			a = append(a, l.Aliases()...)
		}

		sd.aliases = a

		serviceData[p] = sd
	}

	return nil
}

func ProviderPackageForAlias(serviceAlias string) (string, error) {
	for k, v := range serviceData {
		if slices.Contains(v.aliases, serviceAlias) {
			return k, nil
		}
	}

	return "", fmt.Errorf("unable to find service for service alias %s", serviceAlias)
}

func ProviderPackages() []string {
	keys := make([]string, len(serviceData))

	i := 0
	for k := range serviceData {
		keys[i] = k
		i++
	}

	return keys
}

func Aliases() []string {
	keys := make([]string, 0)

	for _, v := range serviceData {
		keys = append(keys, v.aliases...)
	}

	return keys
}

func ProviderNameUpper(service string) (string, error) {
	if v, ok := serviceData[service]; ok {
		return v.providerNameUpper, nil
	}

	return "", fmt.Errorf("no service data found for %s", service)
}

// EndpointForService generates a host by substituting the effective region into the service's endpoint template.
// It returns an empty string when the service has no template or the region is empty; the caller should fall back to other endpoint sources.
func EndpointForService(service, region string) string {
	if region == "" {
		return ""
	}
	if v, ok := serviceData[service]; ok && v.endpointTemplate != "" {
		return strings.ReplaceAll(v.endpointTemplate, "{region}", region)
	}
	return ""
}

// RequiresConfiguredEndpoint reports whether the service is an OOS service for which the user must explicitly configure
// an endpoint. The determination is based on whether the service defines an endpoint template (endpoint_template): OOS
// services all carry a template, while the others do not. OOS is a private cloud with no connectivity to the AWS public
// network, so when no endpoint is configured, it must not fall back to the SDK default resolver (which would resolve to a
// real AWS public host); instead, an error should be reported when the request is made.
func RequiresConfiguredEndpoint(service string) bool {
	if v, ok := serviceData[service]; ok {
		return v.endpointTemplate != ""
	}
	return false
}

// RegionForEndpoint reverse-derives the region from a user-configured endpoint according to the service template: the
// {region} placeholder in the template is treated as a capture group, the rest is matched literally, and differences in
// scheme (http/https) and trailing slashes are tolerated. OOS does not configure a region on the provider; the signing
// region is always derived this way.
// It returns the region only when the endpoint matches the service template; otherwise it returns an empty string (the caller reports an error accordingly).
func RegionForEndpoint(service, endpoint string) string {
	if endpoint == "" {
		return ""
	}
	v, ok := serviceData[service]
	if !ok || v.endpointTemplate == "" {
		return ""
	}

	const placeholder = "{region}"
	idx := strings.Index(v.endpointTemplate, placeholder)
	if idx < 0 {
		return ""
	}
	prefix := v.endpointTemplate[:idx]
	suffix := v.endpointTemplate[idx+len(placeholder):]

	// Tolerate the scheme: the template and the input may use http/https, so strip the scheme uniformly before matching.
	stripScheme := func(s string) string {
		if _, after, ok := strings.Cut(s, "://"); ok {
			return after
		}
		return s
	}
	// Strip the port: the template has no port, but the input may carry one (e.g. :8443), so ignore the port before matching.
	stripPort := func(host string) string {
		if h, _, err := net.SplitHostPort(host); err == nil {
			return h
		}
		return host
	}
	prefix = stripScheme(prefix)
	suffix = strings.TrimRight(suffix, "/")
	host := stripPort(strings.TrimRight(stripScheme(endpoint), "/"))

	// The region capture group is restricted to the region-name character set (lowercase letters, digits, hyphens; no dots,
	// because a dot is a host segment separator), matched non-greedily and anchored at the right boundary by the fixed suffix.
	pattern := "^" + regexp.QuoteMeta(prefix) + "([0-9a-z-]+?)" + regexp.QuoteMeta(suffix) + "$"
	re, err := regexp.Compile(pattern)
	if err != nil {
		return ""
	}
	if m := re.FindStringSubmatch(host); len(m) == 2 {
		return m[1]
	}
	return ""
}

// RebaseEndpointRegion replaces the region segment in a user-configured endpoint with targetRegion,
// for S3 per-resource region overrides: it swaps the region on top of the provider-configured endpoint
// instead of generating one from the template out of thin air, so the scheme (http/https), port (e.g. :8080), and the rest
// of the host are all preserved as-is. The region segment is located using the service template's prefix/suffix: only the
// part of the host sandwiched between the prefix and the suffix is replaced. It returns an empty string when the endpoint
// does not match the template, when the region segment cannot be located, or when targetRegion is empty; the caller falls back accordingly.
func RebaseEndpointRegion(service, endpoint, targetRegion string) string {
	if endpoint == "" || targetRegion == "" {
		return ""
	}
	v, ok := serviceData[service]
	if !ok || v.endpointTemplate == "" {
		return ""
	}

	const placeholder = "{region}"
	idx := strings.Index(v.endpointTemplate, placeholder)
	if idx < 0 {
		return ""
	}
	// The template prefix/suffix do not include the scheme, so strip the scheme uniformly and take the literals of the host part.
	stripScheme := func(s string) string {
		if _, after, ok := strings.Cut(s, "://"); ok {
			return after
		}
		return s
	}
	prefix := stripScheme(v.endpointTemplate[:idx])
	suffix := strings.TrimRight(v.endpointTemplate[idx+len(placeholder):], "/")

	// Split the endpoint into the scheme and the remainder, perform the replacement only on the host(+port+path), and then
	// reassemble, preserving the scheme.
	scheme := ""
	rest := endpoint
	if before, after, ok := strings.Cut(endpoint, "://"); ok {
		scheme = before + "://"
		rest = after
	}

	// rest has the form host[:port][/path]. Split out hostport and path, and locate the region only on hostport;
	// the port and path are preserved as-is.
	hostport := rest
	path := ""
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		hostport = rest[:i]
		path = rest[i:]
	}
	host := hostport
	port := ""
	if h, p, err := net.SplitHostPort(hostport); err == nil {
		host = h
		port = ":" + p
	}

	// The host must be exactly prefix + <region> + suffix; the region is restricted to the OOS region character set
	// (lowercase letters, digits, hyphens; no dots) and is anchored at the right boundary by the fixed suffix.
	pattern := "^" + regexp.QuoteMeta(prefix) + "([0-9a-z-]+?)" + regexp.QuoteMeta(suffix) + "$"
	re, err := regexp.Compile(pattern)
	if err != nil {
		return ""
	}
	if !re.MatchString(host) {
		return ""
	}
	newHost := prefix + targetRegion + suffix

	return scheme + newHost + port + path
}

func FullHumanFriendly(service string) (string, error) {
	if v, ok := serviceData[service]; ok {
		if v.brand == "" {
			return v.humanFriendly, nil
		}

		return fmt.Sprintf("%s %s", v.brand, v.humanFriendly), nil
	}

	if s, err := ProviderPackageForAlias(service); err == nil {
		return FullHumanFriendly(s)
	}

	return "", fmt.Errorf("no service data found for %s", service)
}

func HumanFriendly(service string) (string, error) {
	if v, ok := serviceData[service]; ok {
		return v.humanFriendly, nil
	}

	if s, err := ProviderPackageForAlias(service); err == nil {
		return HumanFriendly(s)
	}

	return "", fmt.Errorf("no service data found for %s", service)
}

const (
	ResourceTopLevelRegionAttributeDescription     = `Region where this resource will be [managed](https://docs.aws.amazon.com/general/latest/gr/rande.html#regional-endpoints). ` + topLevelRegionDefaultDescription
	ListResourceTopLevelRegionAttributeDescription = `Region to [query](https://docs.aws.amazon.com/general/latest/gr/rande.html#regional-endpoints) for resources of this type. ` + topLevelRegionDefaultDescription
	ActionTopLevelRegionAttributeDescription       = `Region where this action will be [executed](https://docs.aws.amazon.com/general/latest/gr/rande.html#regional-endpoints). ` + topLevelRegionDefaultDescription

	topLevelRegionDefaultDescription = `Defaults to the Region set in the [provider configuration](https://registry.terraform.io/providers/ctyun-oos-lab/ctyunoos/latest/docs#provider-configuration).`
)
