// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package iam

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/YakDriver/regexache"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func validResourceName(max int) schema.SchemaValidateFunc {
	return validation.All(
		validation.StringLenBetween(1, max),
		validation.StringMatch(regexache.MustCompile(`^[\w+=,.@-]*$`), "must match [\\w+=,.@-]"),
	)
}

var validAccountAlias = validation.All(
	validation.StringLenBetween(3, 63),
	validation.StringMatch(regexache.MustCompile(`^[0-9a-z][0-9a-z-]+$`), "must start with an alphanumeric character and only contain lowercase alphanumeric characters and hyphens"),
	func(v any, k string) (ws []string, es []error) {
		val := v.(string)
		if strings.Contains(val, "--") {
			es = append(es, fmt.Errorf("%q must not contain consecutive hyphens", k))
		}
		if strings.HasSuffix(val, "-") {
			es = append(es, fmt.Errorf("%q must not end in a hyphen", k))
		}
		return
	},
)

var validOpenIDURL = validation.All(
	validation.IsURLWithHTTPS,
	func(v any, k string) (ws []string, es []error) {
		value := v.(string)
		u, err := url.Parse(value)
		if err != nil {
			// validation.IsURLWithHTTPS will already have returned an error for an invalid URL
			return
		}
		if len(u.Query()) > 0 {
			es = append(es, fmt.Errorf("%q cannot contain query parameters per the OIDC standard", k))
		}
		return
	},
)

var validPolicyPath = validation.AllDiag(
	validation.ToDiagFunc(validation.StringLenBetween(1, 512)),
	func(i any, path cty.Path) diag.Diagnostics {
		val := i.(string)
		if !strings.HasPrefix(val, "/") || !strings.HasSuffix(val, "/") {
			return diag.Diagnostics{
				errs.NewInvalidValueAttributeError(
					path,
					fmt.Sprintf("Attribute %q must begin and end with a slash (/), got %q", errs.PathString(path), val),
				),
			}
		}
		if strings.Contains(val, "//") {
			return diag.Diagnostics{
				errs.NewInvalidValueAttributeError(
					path,
					fmt.Sprintf("Attribute %q must not contain consecutive slashes (//), got %q", errs.PathString(path), val),
				),
			}
		}
		return nil
	},
	validation.ToDiagFunc(validation.StringMatch(regexache.MustCompile(`^[A-Za-z0-9\.,\+@=_/-]*$`), "must contain uppercase or lowercase alphanumeric characters or any of the following: / , . + @ = _ -")),
)

func validRolePolicyDocumentCharacters(v any, k string) (ws []string, errors []error) {
	for _, r := range v.(string) {
		if r == '\t' || r == '\n' || r == '\r' || (r >= 0x20 && r <= 0xff) {
			continue
		}

		errors = append(errors, fmt.Errorf("%q must contain only printable ASCII, Basic Latin, Latin-1 Supplement, tab, newline, or carriage return characters", k))
		break
	}

	return
}

func validUserTags(v any, path cty.Path) diag.Diagnostics {
	var diags diag.Diagnostics
	m := v.(map[string]any)

	if len(m) > 10 {
		diags = append(diags, diag.Diagnostic{
			Severity:      diag.Error,
			Summary:       "Bad map size",
			Detail:        fmt.Sprintf("Map must contain at most 10 elements: length=%d", len(m)),
			AttributePath: path,
		})
	}

	keyValidators := []schema.SchemaValidateDiagFunc{
		validation.ToDiagFunc(validation.StringLenBetween(1, 128)),
		validation.ToDiagFunc(validation.StringMatch(
			regexache.MustCompile(`^[\p{L}\p{Z}\p{N}_.:/=+\-@]+$`),
			"must match [\\p{L}\\p{Z}\\p{N}_.:/=+\\-@]+",
		)),
	}
	valueValidators := []schema.SchemaValidateDiagFunc{
		validation.ToDiagFunc(validation.StringLenBetween(0, 256)),
		validation.ToDiagFunc(validation.StringMatch(
			regexache.MustCompile(`^[\p{L}\p{Z}\p{N}_.:/=+\-@]*$`),
			"must match [\\p{L}\\p{Z}\\p{N}_.:/=+\\-@]*",
		)),
	}

	for k, v := range m {
		for _, validator := range keyValidators {
			diags = append(diags, validator(k, path.IndexString(k))...)
		}

		value, ok := v.(string)
		if !ok {
			diags = append(diags, diag.Diagnostic{
				Severity:      diag.Error,
				Summary:       "Invalid value type",
				Detail:        fmt.Sprintf("Tag value for key %q must be a string", k),
				AttributePath: path.IndexString(k),
			})
			continue
		}

		for _, validator := range valueValidators {
			diags = append(diags, validator(value, path.IndexString(k))...)
		}
	}

	return diags
}
