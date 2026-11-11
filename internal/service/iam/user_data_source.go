// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package iam

import (
	"context"
	"fmt"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/sdkdiag"
	tftags "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/tags"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// @SDKDataSource("ctyun_iam_user", name="User")
// @Tags
// @Testing(tagsIdentifierAttribute="user_name", tagsResourceType="User")
func dataSourceUser() *schema.Resource {
	return &schema.Resource{
		ReadWithoutTimeout: dataSourceUserRead,

		Schema: map[string]*schema.Schema{
			names.AttrARN: {
				Type:     schema.TypeString,
				Computed: true,
			},
			"ip_last_used": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"password_last_used": {
				Type:     schema.TypeString,
				Computed: true,
			},
			names.AttrUserName: {
				Type:     schema.TypeString,
				Required: true,
				ValidateFunc: validation.All(
					validation.StringLenBetween(1, 64),
					validation.StringMatch(
						regexache.MustCompile(`^[0-9A-Za-z=,.@\-_+]+$`),
						"must only contain alphanumeric characters, hyphens, underscores, commas, periods, @ symbols, plus and equals signs",
					),
				),
			},
			"user_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
			names.AttrTags: tftags.TagsSchemaComputed(),
		},
	}
}

func dataSourceUserRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	userName := d.Get(names.AttrUserName).(string)
	output, err := conn.GetUser(ctx, &iam.GetUserInput{
		UserName: aws.String(userName),
	}, withOOSIAMGetUserResponse())

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading IAM User (%s): %s", userName, err)
	}
	if output == nil || output.User == nil {
		return sdkdiag.AppendFromErr(diags, fmt.Errorf("reading IAM User (%s): empty result", userName))
	}

	user := output.User

	d.SetId(aws.ToString(user.UserId))
	d.Set(names.AttrARN, user.Arn)
	d.Set("password_last_used", policyTimeString(user.PasswordLastUsed))
	d.Set("user_id", user.UserId)
	d.Set(names.AttrUserName, user.UserName)
	// Always write to state: when an IP is present, use the metadata; when it is not returned or is an empty string, the Get fails and the value becomes "".
	ipLastUsed, _ := output.ResultMetadata.Get(oosIAMUserIPLastUsedMetadataKey).(string)
	d.Set("ip_last_used", ipLastUsed)

	setTagsOut(ctx, user.Tags)

	return diags
}
