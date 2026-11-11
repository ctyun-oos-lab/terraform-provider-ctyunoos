// Copyright IBM Corp. 2014, 2026
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

// @FrameworkDataSource("ctyun_iam_role_policy_attachments", name="Role Policy Attachments")
func newRolePolicyAttachmentsDataSource(context.Context) (datasource.DataSourceWithConfigure, error) {
	return &rolePolicyAttachmentsDataSource{}, nil
}

type rolePolicyAttachmentsDataSource struct {
	framework.DataSourceWithModel[rolePolicyAttachmentsDataSourceModel]
}

func (d *rolePolicyAttachmentsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"attached_policies": framework.DataSourceComputedListOfObjectAttribute[attachedPolicyModel](ctx),
			"id": schema.StringAttribute{
				Computed: true,
			},
			"role_name": schema.StringAttribute{
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

func (d *rolePolicyAttachmentsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	conn := d.Meta().IAMClient(ctx)

	var data rolePolicyAttachmentsDataSourceModel
	smerr.AddEnrich(ctx, &resp.Diagnostics, req.Config.Get(ctx, &data))
	if resp.Diagnostics.HasError() {
		return
	}

	var input iam.ListAttachedRolePoliciesInput
	smerr.AddEnrich(ctx, &resp.Diagnostics, flex.Expand(ctx, data, &input))
	if resp.Diagnostics.HasError() {
		return
	}

	roleName := data.RoleName.ValueString()
	out, policyMetadata, err := findRolePolicyAttachments(ctx, conn, &input)
	if err != nil {
		smerr.AddError(ctx, &resp.Diagnostics, err, "role_name", roleName)
		return
	}

	data.ID = types.StringValue(roleName)
	data.AttachedPolicies, resp.Diagnostics = fwtypes.NewListNestedObjectValueOfValueSlice(ctx, flattenRoleAttachedPolicies(out, policyMetadata))
	if resp.Diagnostics.HasError() {
		return
	}

	smerr.AddEnrich(ctx, &resp.Diagnostics, resp.State.Set(ctx, &data), "role_name", roleName)
}

func findRolePolicyAttachments(ctx context.Context, conn *iam.Client, input *iam.ListAttachedRolePoliciesInput, optFns ...func(*iam.Options)) ([]awstypes.AttachedPolicy, map[string]oosAttachedPolicyMetadata, error) {
	var output []awstypes.AttachedPolicy
	metadata := make(map[string]oosAttachedPolicyMetadata)

	paginator := iam.NewListAttachedRolePoliciesPaginator(conn, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx, append(optFns, withOOSIAMListAttachedPoliciesResponse())...)
		if err != nil {
			return nil, nil, err
		}
		output = append(output, page.AttachedPolicies...)
		if pageMetadata, ok := page.ResultMetadata.Get(oosIAMAttachedPoliciesKey).(map[string]oosAttachedPolicyMetadata); ok {
			for policyARN, policy := range pageMetadata {
				metadata[policyARN] = policy
			}
		}
	}

	return output, metadata, nil
}

func flattenRoleAttachedPolicies(policies []awstypes.AttachedPolicy, metadata map[string]oosAttachedPolicyMetadata) []attachedPolicyModel {
	output := make([]attachedPolicyModel, 0, len(policies))

	for _, policy := range policies {
		policyARN := aws.ToString(policy.PolicyArn)
		policyMetadata := metadata[policyARN]
		output = append(output, attachedPolicyModel{
			Description: types.StringValue(policyMetadata.Description),
			PolicyArn:   types.StringValue(policyARN),
			PolicyName:  types.StringValue(aws.ToString(policy.PolicyName)),
			Scope:       types.StringValue(policyMetadata.Scope),
		})
	}

	return output
}

type rolePolicyAttachmentsDataSourceModel struct {
	AttachedPolicies fwtypes.ListNestedObjectValueOf[attachedPolicyModel] `tfsdk:"attached_policies"`
	ID               types.String                                         `tfsdk:"id"`
	RoleName         types.String                                         `tfsdk:"role_name"`
}

type attachedPolicyModel struct {
	Description types.String `tfsdk:"description"`
	PolicyArn   types.String `tfsdk:"policy_arn"`
	PolicyName  types.String `tfsdk:"policy_name"`
	Scope       types.String `tfsdk:"scope"`
}
