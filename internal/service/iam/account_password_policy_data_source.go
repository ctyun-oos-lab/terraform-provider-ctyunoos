// Copyright IBM Corp. 2014, 2026
// Modifications Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package iam

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	awstypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/sdkdiag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// @SDKDataSource("ctyun_iam_account_password_policy", name="Account Password Policy")
func dataSourceAccountPasswordPolicy() *schema.Resource {
	return &schema.Resource{
		ReadWithoutTimeout: dataSourceAccountPasswordPolicyRead,

		Schema: map[string]*schema.Schema{
			"allow_users_to_change_password": {
				Type:     schema.TypeBool,
				Computed: true,
			},
			"expire_passwords": {
				Type:     schema.TypeBool,
				Computed: true,
			},
			"hard_expiry": {
				Type:     schema.TypeBool,
				Computed: true,
			},
			"max_password_age": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"minimum_password_length": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"password_reuse_prevention": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"require_lowercase_characters": {
				Type:     schema.TypeBool,
				Computed: true,
			},
			"require_numbers": {
				Type:     schema.TypeBool,
				Computed: true,
			},
			"require_symbols": {
				Type:     schema.TypeBool,
				Computed: true,
			},
			"require_uppercase_characters": {
				Type:     schema.TypeBool,
				Computed: true,
			},
		},
	}
}

func dataSourceAccountPasswordPolicyRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	policy, err := findAccountPasswordPolicy(ctx, conn)
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading IAM Account Password Policy: %s", err)
	}

	d.SetId("iam-account-password-policy")

	for k, v := range flattenAccountPasswordPolicyForDataSource(policy) {
		d.Set(k, v)
	}

	return diags
}

func flattenAccountPasswordPolicyForDataSource(policy *awstypes.PasswordPolicy) map[string]any {
	if policy == nil {
		return map[string]any{}
	}

	return map[string]any{
		"allow_users_to_change_password": policy.AllowUsersToChangePassword,
		"expire_passwords":               policy.ExpirePasswords,
		"hard_expiry":                    aws.ToBool(policy.HardExpiry),
		"max_password_age":               int(aws.ToInt32(policy.MaxPasswordAge)),
		"minimum_password_length":        int(aws.ToInt32(policy.MinimumPasswordLength)),
		"password_reuse_prevention":      int(aws.ToInt32(policy.PasswordReusePrevention)),
		"require_lowercase_characters":   policy.RequireLowercaseCharacters,
		"require_numbers":                policy.RequireNumbers,
		"require_symbols":                policy.RequireSymbols,
		"require_uppercase_characters":   policy.RequireUppercaseCharacters,
	}
}
