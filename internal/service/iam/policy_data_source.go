// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package iam

import (
	"context"
	"fmt"

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

// @SDKDataSource("ctyun_iam_policy", name="Policy")
func dataSourcePolicy() *schema.Resource {
	return &schema.Resource{
		ReadWithoutTimeout: dataSourcePolicyRead,

		Schema: map[string]*schema.Schema{
			names.AttrARN: {
				Type:         schema.TypeString,
				Optional:     true,
				Computed:     true,
				ExactlyOneOf: []string{names.AttrARN, names.AttrName},
				ValidateFunc: validation.StringLenBetween(20, 2048),
			},
			"attachment_count": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			names.AttrDescription: {
				Type:     schema.TypeString,
				Computed: true,
			},
			"create_date": {
				Type:     schema.TypeString,
				Computed: true,
			},
			names.AttrName: {
				Type:         schema.TypeString,
				Optional:     true,
				Computed:     true,
				ExactlyOneOf: []string{names.AttrARN, names.AttrName},
			},
			names.AttrPolicy: {
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
	}
}

func dataSourcePolicyRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	arn := d.Get(names.AttrARN).(string)
	if arn == "" {
		var err error
		arn, err = findPolicyARNByName(ctx, conn, d.Get(names.AttrName).(string))
		if err != nil {
			return sdkdiag.AppendErrorf(diags, "reading IAM Policy by name (%s): %s", d.Get(names.AttrName).(string), err)
		}
	}

	// We need to make a call to `iam.GetPolicy` because `iam.ListPolicies` doesn't return all values.
	// OOS also returns the policy document in GetPolicy, so do not call GetPolicyVersion.
	getPolicyOutput, err := getPolicyByARN(ctx, conn, arn)

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading IAM Policy (%s): %s", arn, err)
	}

	policy := getPolicyOutput.Policy
	arn = aws.ToString(policy.Arn)

	d.SetId(arn)
	resourcePolicyFlatten(policy, d)
	d.Set("create_date", policyTimeString(policy.CreateDate))

	scope, _ := getPolicyOutput.ResultMetadata.Get(oosIAMPolicyScopeMetadataKey).(string)
	if scope == "" {
		return sdkdiag.AppendFromErr(diags, fmt.Errorf("GetPolicy response did not include scope"))
	}
	err = d.Set(names.AttrScope, scope)
	if err != nil {
		return sdkdiag.AppendFromErr(diags, fmt.Errorf("Get scope error"))
	}

	policyDocument, _ := getPolicyOutput.ResultMetadata.Get(oosIAMPolicyDocumentMetadataKey).(string)
	if policyDocument == "" {
		return sdkdiag.AppendFromErr(diags, fmt.Errorf("GetPolicy response did not include policy document"))
	}

	if err := resourcePolicyFlattenPolicyDocument(policyDocument, d); err != nil {
		return sdkdiag.AppendFromErr(diags, err)
	}

	return diags
}

func findPolicyARNByName(ctx context.Context, conn *iam.Client, name string) (string, error) {
	policies, _, err := findPolicies(ctx, conn, &iam.ListPoliciesInput{}, withOOSIAMListPoliciesResponse(name))
	if err != nil {
		return "", err
	}

	return findPolicyARNByNameFromList(policies, name)
}

// findPolicyARNByNameFromList exactly matches a unique ARN by PolicyName among the listed policies.
// Zero matches return "no IAM policy found"; multiple matches return "multiple IAM policies found".
func findPolicyARNByNameFromList(policies []awstypes.Policy, name string) (string, error) {
	var matchedPolicies []awstypes.Policy
	for _, policy := range policies {
		if aws.ToString(policy.PolicyName) == name {
			matchedPolicies = append(matchedPolicies, policy)
		}
	}

	if len(matchedPolicies) == 0 {
		return "", fmt.Errorf("no IAM policy found")
	}

	if len(matchedPolicies) > 1 {
		return "", fmt.Errorf("multiple IAM policies found")
	}

	return aws.ToString(matchedPolicies[0].Arn), nil
}
