// Copyright IBM Corp. 2014, 2026
// Modifications Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package iam

import (
	"context"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	awstypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/framework"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/framework/flex"
	fwtypes "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/framework/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/smerr"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// @FrameworkDataSource("ctyun_iam_user_group_membership", name="User Group Membership")
func newUserGroupMembershipDataSource(context.Context) (datasource.DataSourceWithConfigure, error) {
	return &userGroupMembershipDataSource{}, nil
}

type userGroupMembershipDataSource struct {
	framework.DataSourceWithModel[userGroupMembershipDataSourceModel]
}

func (d *userGroupMembershipDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"groups_for_user": framework.DataSourceComputedListOfObjectAttribute[groupForUserModel](ctx),
			"id": schema.StringAttribute{
				Computed: true,
			},
			"user_name": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 64),
					stringvalidator.RegexMatches(
						regexache.MustCompile(`^[0-9A-Za-z=,.@\-_+]+$`),
						"must only contain alphanumeric characters, hyphens, underscores, commas, periods, @ symbols, plus and equals signs",
					),
				},
			},
		},
	}
}

func (d *userGroupMembershipDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	conn := d.Meta().IAMClient(ctx)

	var data userGroupMembershipDataSourceModel
	smerr.AddEnrich(ctx, &resp.Diagnostics, req.Config.Get(ctx, &data))
	if resp.Diagnostics.HasError() {
		return
	}

	var input iam.ListGroupsForUserInput
	smerr.AddEnrich(ctx, &resp.Diagnostics, flex.Expand(ctx, data, &input))
	if resp.Diagnostics.HasError() {
		return
	}

	userName := data.UserName.ValueString()
	groups, err := findGroupsForUser(ctx, conn, &input)
	if err != nil {
		smerr.AddError(ctx, &resp.Diagnostics, err, "user_name", userName)
		return
	}

	data.ID = types.StringValue(userName)
	data.GroupsForUser, resp.Diagnostics = fwtypes.NewListNestedObjectValueOfValueSlice(ctx, flattenGroupsForUser(groups))
	if resp.Diagnostics.HasError() {
		return
	}

	smerr.AddEnrich(ctx, &resp.Diagnostics, resp.State.Set(ctx, &data), "user_name", userName)
}

func findGroupsForUser(ctx context.Context, conn *iam.Client, input *iam.ListGroupsForUserInput) ([]awstypes.Group, error) {
	var output []awstypes.Group

	paginator := iam.NewListGroupsForUserPaginator(conn, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		output = append(output, page.Groups...)
	}

	return output, nil
}

func flattenGroupsForUser(groups []awstypes.Group) []groupForUserModel {
	output := make([]groupForUserModel, 0, len(groups))

	for _, group := range groups {
		output = append(output, groupForUserModel{
			ARN:        types.StringValue(aws.ToString(group.Arn)),
			CreateDate: types.StringValue(policyTimeString(group.CreateDate)),
			GroupID:    types.StringValue(aws.ToString(group.GroupId)),
			GroupName:  types.StringValue(aws.ToString(group.GroupName)),
		})
	}

	return output
}

type userGroupMembershipDataSourceModel struct {
	GroupsForUser fwtypes.ListNestedObjectValueOf[groupForUserModel] `tfsdk:"groups_for_user"`
	ID            types.String                                       `tfsdk:"id"`
	UserName      types.String                                       `tfsdk:"user_name"`
}

type groupForUserModel struct {
	ARN        types.String `tfsdk:"arn"`
	CreateDate types.String `tfsdk:"create_date"`
	GroupID    types.String `tfsdk:"group_id"`
	GroupName  types.String `tfsdk:"group_name"`
}
