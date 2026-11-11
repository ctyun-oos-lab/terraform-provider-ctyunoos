// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package iam

import (
	"context"
	"regexp"

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

// @SDKDataSource("ctyun_iam_users", name="Users")
func dataSourceUsers() *schema.Resource {
	return &schema.Resource{
		ReadWithoutTimeout: dataSourceUsersRead,

		Schema: map[string]*schema.Schema{
			names.AttrARNs: {
				Type:     schema.TypeSet,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"name_regex": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringIsValidRegExp,
			},
			names.AttrNames: {
				Type:     schema.TypeSet,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
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

func dataSourceUsersRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	results, _, err := findUsers(ctx, conn, &iam.ListUsersInput{})

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading IAM Users: %s", err)
	}

	if v, ok := d.GetOk("name_regex"); ok {
		nameRegex := v.(string)
		r, err := regexp.Compile(nameRegex)
		if err != nil {
			return sdkdiag.AppendErrorf(diags, "compiling name_regex (%s): %s", nameRegex, err)
		}

		results = filterUsersByNameRegex(results, r)
	}

	d.SetId(meta.(*conns.AWSClient).EffectiveRealRegion(ctx))
	tfUsers, arns, userNames := flattenUsers(results)
	d.Set(names.AttrARNs, arns)
	d.Set(names.AttrNames, userNames)
	d.Set("users", tfUsers)

	return diags
}

func filterUsersByNameRegex(users []awstypes.User, r *regexp.Regexp) []awstypes.User {
	var output []awstypes.User
	for _, user := range users {
		if r.MatchString(aws.ToString(user.UserName)) {
			output = append(output, user)
		}
	}
	return output
}

func flattenUsers(users []awstypes.User) ([]map[string]any, []string, []string) {
	if len(users) == 0 {
		return []map[string]any{}, nil, nil
	}

	output := make([]map[string]any, 0, len(users))
	arns := make([]string, 0, len(users))
	userNames := make([]string, 0, len(users))
	for _, user := range users {
		userName := aws.ToString(user.UserName)
		arn := aws.ToString(user.Arn)
		arns = append(arns, arn)
		userNames = append(userNames, userName)
		output = append(output, map[string]any{
			names.AttrARN:        arn,
			"create_date":        policyTimeString(user.CreateDate),
			"password_last_used": policyTimeString(user.PasswordLastUsed),
			"user_id":            aws.ToString(user.UserId),
			names.AttrUserName:   userName,
		})
	}

	return output, arns, userNames
}

func findUsers(ctx context.Context, conn *iam.Client, input *iam.ListUsersInput, optFns ...func(*iam.Options)) ([]awstypes.User, map[string]oosListUsersMetadata, error) {
	var output []awstypes.User
	metadata := map[string]oosListUsersMetadata{}

	pages := iam.NewListUsersPaginator(conn, input)
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx, append(optFns, withOOSIAMListUsersResponse())...)

		if err != nil {
			return nil, nil, err
		}

		if pageMetadata, ok := page.ResultMetadata.Get(oosIAMListUsersMetadataKey).(map[string]oosListUsersMetadata); ok {
			for userName, userMetadata := range pageMetadata {
				metadata[userName] = userMetadata
			}
		}
		output = append(output, page.Users...)
	}

	return output, metadata, nil
}
