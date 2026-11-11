// Copyright IBM Corp. 2014, 2026
// Modifications Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package iam

import (
	"context"
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

// @SDKDataSource("ctyun_iam_policies", name="Policies")
func dataSourcePolicies() *schema.Resource {
	return &schema.Resource{
		ReadWithoutTimeout: dataSourcePoliciesRead,

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
			"policies": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						names.AttrARN: {
							Type:     schema.TypeString,
							Computed: true,
						},
						"attachment_count": {
							Type:     schema.TypeInt,
							Computed: true,
						},
						"create_date": {
							Type:     schema.TypeString,
							Computed: true,
						},
						names.AttrDescription: {
							Type:     schema.TypeString,
							Computed: true,
						},
						names.AttrName: {
							Type:     schema.TypeString,
							Computed: true,
						},
						"policy_id": {
							Type:     schema.TypeString,
							Computed: true,
						},
						names.AttrScope: {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
		},
	}
}

func dataSourcePoliciesRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	policies, scopes, err := findPolicies(ctx, conn, &iam.ListPoliciesInput{}, withOOSIAMListPoliciesResponse(""))
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading IAM Policies: %s", err)
	}

	if v, ok := d.GetOk("name_regex"); ok {
		nameRegex := v.(string)
		r, err := regexp.Compile(nameRegex)
		if err != nil {
			return sdkdiag.AppendErrorf(diags, "compiling name_regex (%s): %s", nameRegex, err)
		}

		policies = filterPoliciesByNameRegex(policies, r)
	}

	d.SetId(meta.(*conns.AWSClient).EffectiveRealRegion(ctx))

	tfPolicies, arns, policyNames := flattenPolicies(policies, scopes)
	d.Set(names.AttrARNs, arns)
	d.Set(names.AttrNames, policyNames)
	if err := d.Set("policies", tfPolicies); err != nil {
		return sdkdiag.AppendErrorf(diags, "setting policies: %s", err)
	}

	return diags
}

func findPolicies(ctx context.Context, conn *iam.Client, input *iam.ListPoliciesInput, optFns ...func(*iam.Options)) ([]awstypes.Policy, map[string]string, error) {
	var output []awstypes.Policy
	scopes := map[string]string{}

	paginator := iam.NewListPoliciesPaginator(conn, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx, optFns...)
		if err != nil {
			return nil, nil, err
		}

		if pageScopes, ok := page.ResultMetadata.Get(oosIAMPolicyScopesMetadataKey).(map[string]string); ok {
			for policyID, scope := range pageScopes {
				scopes[policyID] = scope
			}
		}

		output = append(output, page.Policies...)
	}

	return output, scopes, nil
}

func filterPoliciesByNameRegex(policies []awstypes.Policy, r *regexp.Regexp) []awstypes.Policy {
	var output []awstypes.Policy
	for _, policy := range policies {
		if r.MatchString(aws.ToString(policy.PolicyName)) {
			output = append(output, policy)
		}
	}
	return output
}

func flattenPolicies(policies []awstypes.Policy, scopes map[string]string) ([]map[string]any, []string, []string) {
	if len(policies) == 0 {
		return []map[string]any{}, nil, nil
	}

	output := make([]map[string]any, 0, len(policies))
	arns := make([]string, 0, len(policies))
	policyNames := make([]string, 0, len(policies))
	for _, policy := range policies {
		policyARN := aws.ToString(policy.Arn)
		policyName := aws.ToString(policy.PolicyName)
		policyID := aws.ToString(policy.PolicyId)

		arns = append(arns, policyARN)
		policyNames = append(policyNames, policyName)
		output = append(output, map[string]any{
			names.AttrARN:         policyARN,
			"attachment_count":    int(aws.ToInt32(policy.AttachmentCount)),
			"create_date":         policyTimeString(policy.CreateDate),
			names.AttrDescription: aws.ToString(policy.Description),
			names.AttrName:        policyName,
			"policy_id":           policyID,
			names.AttrScope:       scopes[policyID],
		})
	}

	return output, arns, policyNames
}

func policyTimeString(v *time.Time) string {
	if v == nil {
		return ""
	}

	return v.Format(time.RFC3339)
}
