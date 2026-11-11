// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package iam

import (
	"context"
	"net/url"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/sdkdiag"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// @SDKDataSource("ctyun_iam_role", name="Role")
func dataSourceRole() *schema.Resource {
	return &schema.Resource{
		ReadWithoutTimeout: dataSourceRoleRead,

		Schema: map[string]*schema.Schema{
			names.AttrARN: {
				Type:     schema.TypeString,
				Computed: true,
			},
			"assume_role_policy": {
				Type:     schema.TypeString,
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
			"max_session_duration": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			names.AttrName: {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validResourceName(roleNameMaxLen),
			},
			"unique_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func dataSourceRoleRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	name := d.Get(names.AttrName).(string)
	role, err := findRoleByName(ctx, conn, name)

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading IAM Role (%s): %s", name, err)
	}

	d.SetId(name)
	d.Set(names.AttrARN, role.Arn)
	d.Set("create_date", policyTimeString(role.CreateDate))
	d.Set(names.AttrDescription, role.Description)
	d.Set("max_session_duration", role.MaxSessionDuration)
	d.Set(names.AttrName, role.RoleName)
	d.Set("unique_id", role.RoleId)

	assumeRolePolicy, err := url.QueryUnescape(aws.ToString(role.AssumeRolePolicyDocument))
	if err != nil {
		return sdkdiag.AppendFromErr(diags, err)
	}
	d.Set("assume_role_policy", assumeRolePolicy)

	return diags
}
