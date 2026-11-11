// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package iam

import (
	"context"
	"net/url"
	"regexp"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	awstypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/sdkdiag"
	inttypes "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// @SDKDataSource("ctyun_iam_roles", name="Roles")
func dataSourceRoles() *schema.Resource {
	return &schema.Resource{
		ReadWithoutTimeout: dataSourceRolesRead,

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
			"roles": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
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
							Type:     schema.TypeString,
							Computed: true,
						},
						"unique_id": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
		},
	}
}

func dataSourceRolesRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	results, err := findRoles(ctx, conn, &iam.ListRolesInput{})
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading IAM roles: %s", err)
	}

	if v, ok := d.GetOk("name_regex"); ok {
		nameRegex := v.(string)
		r, err := regexp.Compile(nameRegex)
		if err != nil {
			return sdkdiag.AppendErrorf(diags, "compiling name_regex (%s): %s", nameRegex, err)
		}

		results = filterRolesByNameRegex(results, r)
	}

	d.SetId(meta.(*conns.AWSClient).EffectiveRealRegion(ctx))

	var arns, nms []string

	for _, r := range results {
		arns = append(arns, aws.ToString(r.Arn))
		nms = append(nms, aws.ToString(r.RoleName))
	}

	d.Set(names.AttrARNs, arns)
	d.Set(names.AttrNames, nms)
	if err := d.Set("roles", flattenRoles(results)); err != nil {
		return sdkdiag.AppendErrorf(diags, "setting roles: %s", err)
	}

	return diags
}

func findRoles(ctx context.Context, conn *iam.Client, input *iam.ListRolesInput) ([]awstypes.Role, error) {
	var results []awstypes.Role

	pages := iam.NewListRolesPaginator(conn, input)
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, role := range page.Roles {
			if p := &role; inttypes.IsZero(p) {
				continue
			}

			results = append(results, role)
		}
	}

	return results, nil
}

func filterRolesByNameRegex(roles []awstypes.Role, r *regexp.Regexp) []awstypes.Role {
	var output []awstypes.Role
	for _, role := range roles {
		if r.MatchString(aws.ToString(role.RoleName)) {
			output = append(output, role)
		}
	}
	return output
}

func flattenRoles(apiObjects []awstypes.Role) []map[string]any {
	if len(apiObjects) == 0 {
		return []map[string]any{}
	}

	output := make([]map[string]any, 0, len(apiObjects))

	for _, apiObject := range apiObjects {
		output = append(output, flattenRole(apiObject))
	}

	return output
}

func flattenRole(apiObject awstypes.Role) map[string]any {
	tfMap := map[string]any{
		names.AttrARN:          aws.ToString(apiObject.Arn),
		"create_date":          roleTimeString(apiObject.CreateDate),
		names.AttrDescription:  aws.ToString(apiObject.Description),
		"max_session_duration": int(aws.ToInt32(apiObject.MaxSessionDuration)),
		names.AttrName:         aws.ToString(apiObject.RoleName),
		"unique_id":            aws.ToString(apiObject.RoleId),
	}

	if apiObject.AssumeRolePolicyDocument != nil {
		assumeRolePolicy, err := url.QueryUnescape(aws.ToString(apiObject.AssumeRolePolicyDocument))
		if err == nil {
			tfMap["assume_role_policy"] = assumeRolePolicy
		} else {
			tfMap["assume_role_policy"] = aws.ToString(apiObject.AssumeRolePolicyDocument)
		}
	}

	return tfMap
}

func roleTimeString(t *time.Time) string {
	if t == nil {
		return ""
	}

	return t.Format(time.RFC3339)
}
