// Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package iam

import (
	"context"

	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/sdkdiag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// @SDKDataSource("ctyun_iam_account_login_security_policy", name="Account Login Security Policy")
func dataSourceAccountLoginSecurityPolicy() *schema.Resource {
	return &schema.Resource{
		ReadWithoutTimeout: dataSourceAccountLoginSecurityPolicyRead,

		Schema: map[string]*schema.Schema{
			"allow_single_users_simultaneous_login": {
				Type:     schema.TypeBool,
				Computed: true,
			},
			"lockout_duration": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"login_failed_times": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"period_with_login_failures": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"session_duration": {
				Type:     schema.TypeInt,
				Computed: true,
			},
		},
	}
}

func dataSourceAccountLoginSecurityPolicyRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	policy, err := findAccountLoginSecurityPolicy(ctx, conn)
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading IAM Account Login Security Policy: %s", err)
	}

	d.SetId(accountLoginSecurityPolicyID)

	for k, v := range flattenAccountLoginSecurityPolicyForDataSource(policy) {
		d.Set(k, v)
	}

	return diags
}

func flattenAccountLoginSecurityPolicyForDataSource(policy *accountLoginSecurityPolicyInput) map[string]any {
	if policy == nil {
		return map[string]any{}
	}

	return map[string]any{
		"allow_single_users_simultaneous_login": policy.AllowSingleUsersSimultaneousLogin,
		"lockout_duration":                      policy.LockoutDuration,
		"login_failed_times":                    policy.LoginFailedTimes,
		"period_with_login_failures":            policy.PeriodWithLoginFailures,
		"session_duration":                      policy.SessionDuration,
	}
}
