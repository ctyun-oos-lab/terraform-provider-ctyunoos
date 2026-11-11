// Copyright IBM Corp. 2014, 2026
// Modifications Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
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

// @SDKDataSource("ctyun_iam_groups", name="Groups")
func dataSourceGroups() *schema.Resource {
	return &schema.Resource{
		ReadWithoutTimeout: dataSourceGroupsRead,

		Schema: map[string]*schema.Schema{
			names.AttrARNs: {
				Type:     schema.TypeSet,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"groups": {
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
						"group_id": {
							Type:     schema.TypeString,
							Computed: true,
						},
						names.AttrGroupName: {
							Type:     schema.TypeString,
							Computed: true,
						},
						"policies": {
							Type:     schema.TypeInt,
							Computed: true,
						},
						"users": {
							Type:     schema.TypeInt,
							Computed: true,
						},
					},
				},
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
		},
	}
}

func dataSourceGroupsRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	groups, metadata, err := findGroups(ctx, conn, &iam.ListGroupsInput{})
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading IAM Groups: %s", err)
	}

	if v, ok := d.GetOk("name_regex"); ok {
		nameRegex := v.(string)
		r, err := regexp.Compile(nameRegex)
		if err != nil {
			return sdkdiag.AppendErrorf(diags, "compiling name_regex (%s): %s", nameRegex, err)
		}

		groups = filterGroupsByNameRegex(groups, r)
	}

	d.SetId(meta.(*conns.AWSClient).EffectiveRealRegion(ctx))

	tfGroups, arns, groupNames := flattenGroups(groups, metadata)
	if err := d.Set("groups", tfGroups); err != nil {
		return sdkdiag.AppendErrorf(diags, "setting groups: %s", err)
	}
	d.Set(names.AttrARNs, arns)
	d.Set(names.AttrNames, groupNames)

	return diags
}

func findGroups(ctx context.Context, conn *iam.Client, input *iam.ListGroupsInput, optFns ...func(*iam.Options)) ([]awstypes.Group, map[string]oosListGroupsMetadata, error) {
	var output []awstypes.Group
	metadata := map[string]oosListGroupsMetadata{}

	paginator := iam.NewListGroupsPaginator(conn, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx, append(optFns, withOOSIAMListGroupsResponse())...)
		if err != nil {
			return nil, nil, err
		}

		if pageMetadata, ok := page.ResultMetadata.Get(oosIAMListGroupsMetadataKey).(map[string]oosListGroupsMetadata); ok {
			for groupName, groupMetadata := range pageMetadata {
				metadata[groupName] = groupMetadata
			}
		}

		output = append(output, page.Groups...)
	}

	return output, metadata, nil
}

func filterGroupsByNameRegex(groups []awstypes.Group, r *regexp.Regexp) []awstypes.Group {
	var output []awstypes.Group
	for _, group := range groups {
		if r.MatchString(aws.ToString(group.GroupName)) {
			output = append(output, group)
		}
	}
	return output
}

func flattenGroups(groups []awstypes.Group, metadata map[string]oosListGroupsMetadata) ([]map[string]any, []string, []string) {
	if len(groups) == 0 {
		return []map[string]any{}, nil, nil
	}

	output := make([]map[string]any, 0, len(groups))
	arns := make([]string, 0, len(groups))
	groupNames := make([]string, 0, len(groups))
	for _, group := range groups {
		groupName := aws.ToString(group.GroupName)
		arn := aws.ToString(group.Arn)
		groupMetadata := metadata[groupName]

		arns = append(arns, arn)
		groupNames = append(groupNames, groupName)
		output = append(output, map[string]any{
			names.AttrARN:       arn,
			"create_date":       policyTimeString(group.CreateDate),
			"group_id":          aws.ToString(group.GroupId),
			names.AttrGroupName: groupName,
			"policies":          groupMetadata.Policies,
			"users":             groupMetadata.Users,
		})
	}

	return output, arns, groupNames
}
