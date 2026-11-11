// Copyright IBM Corp. 2014, 2026
// Modifications Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package iam

import (
	"context"
	"net/url"
	"regexp"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	awstypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/sdkdiag"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// @SDKDataSource("ctyun_iam_saml_providers", name="SAML Providers")
func dataSourceSAMLProviders() *schema.Resource {
	return &schema.Resource{
		ReadWithoutTimeout: dataSourceSAMLProvidersRead,

		Schema: map[string]*schema.Schema{
			names.AttrARNs: {
				Type:     schema.TypeSet,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			names.AttrNames: {
				Type:     schema.TypeSet,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"name_regex": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringIsValidRegExp,
			},
			"providers": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						names.AttrARN: {
							Type:     schema.TypeString,
							Computed: true,
						},
						"create_date": {
							Type:     schema.TypeString,
							Computed: true,
						},
						names.AttrName: {
							Type:     schema.TypeString,
							Computed: true,
						},
						"saml_metadata_document": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"saml_provider_uuid": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"valid_until": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
		},
	}
}

func dataSourceSAMLProvidersRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	providerList, err := findSAMLProviders(ctx, conn)
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading IAM SAML Providers: %s", err)
	}

	if v, ok := d.GetOk("name_regex"); ok {
		nameRegex := v.(string)
		r, err := regexp.Compile(nameRegex)
		if err != nil {
			return sdkdiag.AppendErrorf(diags, "compiling name_regex (%s): %s", nameRegex, err)
		}

		providerList = filterSAMLProvidersByNameRegex(providerList, r)
	}

	d.SetId(meta.(*conns.AWSClient).EffectiveRealRegion(ctx))

	var arns, nms []string
	providers := make([]map[string]any, 0, len(providerList))

	for _, provider := range providerList {
		tfMap, err := flattenSAMLProviderListEntry(ctx, conn, provider)
		if err != nil {
			return sdkdiag.AppendErrorf(diags, "reading IAM SAML Provider (%s): %s", aws.ToString(provider.Arn), err)
		}

		arns = append(arns, tfMap[names.AttrARN].(string))
		nms = append(nms, tfMap[names.AttrName].(string))
		providers = append(providers, tfMap)
	}

	d.Set(names.AttrARNs, arns)
	d.Set(names.AttrNames, nms)
	if err := d.Set("providers", providers); err != nil {
		return sdkdiag.AppendErrorf(diags, "setting providers: %s", err)
	}

	return diags
}

// findSAMLProviders aggregates all pages of OOS ListSAMLProviders (default MaxItems=100).
// The AWS SDK's ListSAMLProviders model has no Marker/IsTruncated, so a Marker is injected into the Query and the paging fields are parsed from the response body.
func findSAMLProviders(ctx context.Context, conn *iam.Client) ([]awstypes.SAMLProviderListEntry, error) {
	var results []awstypes.SAMLProviderListEntry
	marker := ""

	for {
		page, nextMarker, truncated, err := listSAMLProvidersPage(ctx, conn, marker)
		if err != nil {
			return nil, err
		}
		results = append(results, page...)
		if !truncated || nextMarker == "" {
			break
		}
		marker = nextMarker
	}

	return results, nil
}

// listSAMLProvidersPage fetches a single page of ListSAMLProviders; an empty marker starts from the first page.
func listSAMLProvidersPage(ctx context.Context, conn *iam.Client, marker string) ([]awstypes.SAMLProviderListEntry, string, bool, error) {
	values := url.Values{}
	if marker != "" {
		values.Set("Marker", marker)
	}

	var pageMeta oosListSAMLProvidersPageMeta
	output, err := conn.ListSAMLProviders(ctx, &iam.ListSAMLProvidersInput{},
		withOOSIAMQueryBody("ListSAMLProviders", values),
		withOOSIAMListSAMLProvidersPageMeta(&pageMeta),
	)
	if err != nil {
		return nil, "", false, err
	}
	if output == nil {
		return nil, "", false, nil
	}

	return output.SAMLProviderList, pageMeta.Marker, pageMeta.IsTruncated, nil
}

func filterSAMLProvidersByNameRegex(providers []awstypes.SAMLProviderListEntry, r *regexp.Regexp) []awstypes.SAMLProviderListEntry {
	var output []awstypes.SAMLProviderListEntry
	for _, provider := range providers {
		name, err := nameFromSAMLProviderARN(aws.ToString(provider.Arn))
		if err != nil {
			continue
		}

		if r.MatchString(name) {
			output = append(output, provider)
		}
	}
	return output
}

func flattenSAMLProviderListEntry(ctx context.Context, conn *iam.Client, provider awstypes.SAMLProviderListEntry) (map[string]any, error) {
	providerARN := aws.ToString(provider.Arn)
	name, err := nameFromSAMLProviderARN(providerARN)
	if err != nil {
		return nil, err
	}

	tfMap := map[string]any{
		names.AttrARN:  providerARN,
		"create_date":  samlProviderTimeString(provider.CreateDate),
		names.AttrName: name,
		"valid_until":  samlProviderTimeString(provider.ValidUntil),
	}

	detail, err := findSAMLProviderByARN(ctx, conn, providerARN)
	if err != nil {
		return nil, err
	}

	tfMap["saml_metadata_document"] = aws.ToString(detail.SAMLMetadataDocument)
	tfMap["saml_provider_uuid"] = aws.ToString(detail.SAMLProviderUUID)
	if detail.ValidUntil != nil {
		tfMap["valid_until"] = samlProviderTimeString(detail.ValidUntil)
	}

	return tfMap, nil
}

func samlProviderTimeString(v *time.Time) string {
	if v == nil {
		return ""
	}

	return v.Format(time.RFC3339)
}
