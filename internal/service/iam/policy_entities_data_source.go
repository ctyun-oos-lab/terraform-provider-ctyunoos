// Copyright IBM Corp. 2014, 2026
// Modifications Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package iam

import (
	"context"

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

// @SDKDataSource("ctyun_iam_policy_entities", name="Policy Entities")
func dataSourcePolicyEntities() *schema.Resource {
	return &schema.Resource{
		ReadWithoutTimeout: dataSourcePolicyEntitiesRead,

		Schema: map[string]*schema.Schema{
			names.AttrARN: {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringLenBetween(20, 2048),
			},
			"groups": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"group_name": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"group_id": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
			"roles": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"role_name": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"role_id": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
			"users": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"user_name": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"user_id": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
		},
	}
}

func dataSourcePolicyEntitiesRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	policyARN := d.Get(names.AttrARN).(string)
	input := &iam.ListEntitiesForPolicyInput{
		PolicyArn: aws.String(policyARN),
	}

	groups, roles, users, err := findEntitiesForPolicy(ctx, conn, input)
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading IAM Policy Entities (%s): %s", policyARN, err)
	}

	d.SetId(policyARN)
	if err := d.Set("groups", flattenPolicyGroups(groups)); err != nil {
		return sdkdiag.AppendErrorf(diags, "setting groups: %s", err)
	}
	if err := d.Set("roles", flattenPolicyRoles(roles)); err != nil {
		return sdkdiag.AppendErrorf(diags, "setting roles: %s", err)
	}
	if err := d.Set("users", flattenPolicyUsers(users)); err != nil {
		return sdkdiag.AppendErrorf(diags, "setting users: %s", err)
	}

	return diags
}

func findEntitiesForPolicy(ctx context.Context, conn *iam.Client, input *iam.ListEntitiesForPolicyInput) ([]awstypes.PolicyGroup, []awstypes.PolicyRole, []awstypes.PolicyUser, error) {
	var groups []awstypes.PolicyGroup
	var roles []awstypes.PolicyRole
	var users []awstypes.PolicyUser

	paginator := iam.NewListEntitiesForPolicyPaginator(conn, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, nil, nil, err
		}

		groups = append(groups, page.PolicyGroups...)
		roles = append(roles, page.PolicyRoles...)
		users = append(users, page.PolicyUsers...)
	}

	return groups, roles, users, nil
}

func flattenPolicyGroups(groups []awstypes.PolicyGroup) []map[string]any {
	if len(groups) == 0 {
		return []map[string]any{}
	}

	output := make([]map[string]any, 0, len(groups))
	for _, group := range groups {
		output = append(output, map[string]any{
			"group_name": aws.ToString(group.GroupName),
			"group_id":   aws.ToString(group.GroupId),
		})
	}

	return output
}

func flattenPolicyRoles(roles []awstypes.PolicyRole) []map[string]any {
	if len(roles) == 0 {
		return []map[string]any{}
	}

	output := make([]map[string]any, 0, len(roles))
	for _, role := range roles {
		output = append(output, map[string]any{
			"role_name": aws.ToString(role.RoleName),
			"role_id":   aws.ToString(role.RoleId),
		})
	}

	return output
}

func flattenPolicyUsers(users []awstypes.PolicyUser) []map[string]any {
	if len(users) == 0 {
		return []map[string]any{}
	}

	output := make([]map[string]any, 0, len(users))
	for _, user := range users {
		output = append(output, map[string]any{
			"user_name": aws.ToString(user.UserName),
			"user_id":   aws.ToString(user.UserId),
		})
	}

	return output
}
