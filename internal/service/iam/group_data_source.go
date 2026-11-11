// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package iam

import (
	"context"

	"github.com/YakDriver/regexache"
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

// @SDKDataSource("ctyun_iam_group", name="Group")
func dataSourceGroup() *schema.Resource {
	return &schema.Resource{
		ReadWithoutTimeout: dataSourceGroupRead,

		Schema: map[string]*schema.Schema{
			names.AttrARN: {
				Type:     schema.TypeString,
				Computed: true,
			},
			"group_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
			names.AttrGroupName: {
				Type:     schema.TypeString,
				Required: true,
				ValidateFunc: validation.All(
					validation.StringLenBetween(1, 128),
					validation.StringMatch(
						regexache.MustCompile(`^[0-9A-Za-z=,.@\-_+]+$`),
						"must only contain alphanumeric characters, hyphens, underscores, commas, periods, @ symbols, plus and equals signs",
					),
				),
			},
			"users": {
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
						"password_last_used": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"user_id": {
							Type:     schema.TypeString,
							Computed: true,
						},
						names.AttrUserName: {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
		},
	}
}

func dataSourceGroupRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	groupName := d.Get(names.AttrGroupName).(string)

	req := &iam.GetGroupInput{
		GroupName: aws.String(groupName),
	}

	var users []awstypes.User
	var group *awstypes.Group

	pages := iam.NewGetGroupPaginator(conn, req)
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx, withOOSIAMGetGroupResponse())
		if err != nil {
			return sdkdiag.AppendErrorf(diags, "getting group: %s", err)
		}
		if group == nil {
			group = page.Group
		}

		users = append(users, page.Users...)
	}

	if group == nil {
		return sdkdiag.AppendErrorf(diags, "no IAM group found")
	}

	d.SetId(aws.ToString(group.GroupId))
	d.Set(names.AttrARN, aws.ToString(group.Arn))
	d.Set("group_id", aws.ToString(group.GroupId))
	d.Set(names.AttrGroupName, aws.ToString(group.GroupName))
	if err := d.Set("users", dataSourceGroupUsersRead(users)); err != nil {
		return sdkdiag.AppendErrorf(diags, "setting users: %s", err)
	}

	return diags
}

func dataSourceGroupUsersRead(iamUsers []awstypes.User) []map[string]any {
	users := make([]map[string]any, 0, len(iamUsers))
	for _, i := range iamUsers {
		u := make(map[string]any)
		u[names.AttrARN] = aws.ToString(i.Arn)
		u["create_date"] = policyTimeString(i.CreateDate)
		u["password_last_used"] = policyTimeString(i.PasswordLastUsed)
		u["user_id"] = aws.ToString(i.UserId)
		u[names.AttrUserName] = aws.ToString(i.UserName)
		users = append(users, u)
	}
	return users
}
