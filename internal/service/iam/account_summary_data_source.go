// Copyright IBM Corp. 2014, 2026
// Modifications Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package iam

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	awstypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/sdkdiag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// @SDKDataSource("ctyun_iam_account_summary", name="Account Summary")
func dataSourceAccountSummary() *schema.Resource {
	return &schema.Resource{
		ReadWithoutTimeout: dataSourceAccountSummaryRead,

		Schema: map[string]*schema.Schema{
			"accesskeys_per_user_quota": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"account_accesskeys_present": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"attached_policies_per_group_quota": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"attached_policies_per_role_quota": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"attached_policies_per_user_quota": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"groups": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"groups_per_user_quota": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"groups_quota": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"mfa_devices": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"mfa_devices_inuse": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"policies": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"policies_quota": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"providers": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"providers_quota": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"roles": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"roles_quota": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"users": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"users_quota": {
				Type:     schema.TypeInt,
				Computed: true,
			},
		},
	}
}

func dataSourceAccountSummaryRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	output, err := conn.GetAccountSummary(ctx, &iam.GetAccountSummaryInput{})
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading IAM Account Summary: %s", err)
	}
	if output == nil || output.SummaryMap == nil {
		return sdkdiag.AppendErrorf(diags, "reading IAM Account Summary: empty result")
	}

	d.SetId("iam_account_summary")

	for attrName, summaryKey := range accountSummaryAttributeKeys() {
		d.Set(attrName, accountSummaryValue(output.SummaryMap, summaryKey))
	}

	return diags
}

func accountSummaryAttributeKeys() map[string]string {
	return map[string]string{
		"accesskeys_per_user_quota":         string(awstypes.SummaryKeyTypeAccessKeysPerUserQuota),
		"account_accesskeys_present":        string(awstypes.SummaryKeyTypeAccountAccessKeysPresent),
		"attached_policies_per_group_quota": string(awstypes.SummaryKeyTypeAttachedPoliciesPerGroupQuota),
		"attached_policies_per_role_quota":  string(awstypes.SummaryKeyTypeAttachedPoliciesPerRoleQuota),
		"attached_policies_per_user_quota":  string(awstypes.SummaryKeyTypeAttachedPoliciesPerUserQuota),
		"groups":                            string(awstypes.SummaryKeyTypeGroups),
		"groups_per_user_quota":             string(awstypes.SummaryKeyTypeGroupsPerUserQuota),
		"groups_quota":                      string(awstypes.SummaryKeyTypeGroupsQuota),
		"mfa_devices":                       string(awstypes.SummaryKeyTypeMFADevices),
		"mfa_devices_inuse":                 string(awstypes.SummaryKeyTypeMFADevicesInUse),
		"policies":                          string(awstypes.SummaryKeyTypePolicies),
		"policies_quota":                    string(awstypes.SummaryKeyTypePoliciesQuota),
		"providers":                         string(awstypes.SummaryKeyTypeProviders),
		"providers_quota":                   "ProvidersQuota",
		"roles":                             string(awstypes.SummaryKeyTypeRoles),
		"roles_quota":                       string(awstypes.SummaryKeyTypeRolesQuota),
		"users":                             string(awstypes.SummaryKeyTypeUsers),
		"users_quota":                       string(awstypes.SummaryKeyTypeUsersQuota),
	}
}

func accountSummaryValue(summaryMap map[string]int32, key string) int {
	return int(summaryMap[key])
}
