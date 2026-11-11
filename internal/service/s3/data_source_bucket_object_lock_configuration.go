// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package s3

import (
	"context"
	"regexp"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/create"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/framework"
	fwtypes "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/framework/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// @FrameworkDataSource("ctyun_oos_bucket_object_lock_configuration", name="Bucket Object Lock Configuration")
func newBucketObjectLockConfigurationDataSource(context.Context) (datasource.DataSourceWithConfigure, error) {
	return &bucketObjectLockConfigurationDataSource{}, nil
}

const (
	DSNameBucketObjectLockConfiguration = "Bucket Object Lock Configuration Data Source"
)

type bucketObjectLockConfigurationDataSource struct {
	framework.DataSourceWithModel[bucketObjectLockConfigurationDataSourceModel]
}

func (d *bucketObjectLockConfigurationDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			names.AttrBucket: schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(3, 63),
					stringvalidator.RegexMatches(regexp.MustCompile(`^[a-z0-9.-]+$`), "must contain only lowercase letters, numbers, periods, and hyphens"),
				},
			},
			names.AttrID: schema.StringAttribute{
				Computed: true,
			},
			"object_lock_enabled": schema.StringAttribute{
				Computed: true,
			},
			names.AttrRule: framework.DataSourceComputedListOfObjectAttribute[dataBucketObjectLockConfigRuleModel](ctx),
		},
	}
}

func (d *bucketObjectLockConfigurationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	conn := d.Meta().S3Client(ctx)

	var data bucketObjectLockConfigurationDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucket := data.Bucket.ValueString()

	out, err := findObjectLockConfiguration(ctx, conn, bucket)
	if err != nil {
		resp.Diagnostics.AddError(
			create.ProblemStandardMessage(names.S3, create.ErrActionReading, DSNameBucketObjectLockConfiguration, bucket, err),
			err.Error(),
		)
		return
	}

	data.ID = types.StringValue(bucket)
	data.ObjectLockEnabled = types.StringValue(out.ObjectLockEnabled)
	data.Rule = flattenOOSObjectLockConfigDataSourceRule(ctx, out.Rule)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// flattenOOSObjectLockConfigDataSourceRule maps the OOS object lock rule into the
// data source's rule model.
func flattenOOSObjectLockConfigDataSourceRule(ctx context.Context, rule *oosLockRule) fwtypes.ListNestedObjectValueOf[dataBucketObjectLockConfigRuleModel] {
	if rule == nil || rule.DefaultRetention == nil {
		return fwtypes.NewListNestedObjectValueOfNull[dataBucketObjectLockConfigRuleModel](ctx)
	}

	dr := rule.DefaultRetention
	retention := dataBucketObjectLockConfigDefaultRetentionModel{
		Days:  types.Int64Null(),
		Mode:  types.StringValue(dr.Mode),
		Years: types.Int64Null(),
	}
	if dr.Days != nil {
		retention.Days = types.Int64Value(int64(aws.ToInt32(dr.Days)))
	}
	if dr.Years != nil {
		retention.Years = types.Int64Value(int64(aws.ToInt32(dr.Years)))
	}

	ruleModel := dataBucketObjectLockConfigRuleModel{
		DefaultRetention: fwtypes.NewListNestedObjectValueOfPtrMust(ctx, &retention),
	}

	return fwtypes.NewListNestedObjectValueOfPtrMust(ctx, &ruleModel)
}

type bucketObjectLockConfigurationDataSourceModel struct {
	framework.WithRegionModel
	ID                types.String                                                         `tfsdk:"id"`
	Bucket            types.String                                                         `tfsdk:"bucket"`
	ObjectLockEnabled types.String                                                         `tfsdk:"object_lock_enabled"`
	Rule              fwtypes.ListNestedObjectValueOf[dataBucketObjectLockConfigRuleModel] `tfsdk:"rule"`
}

type dataBucketObjectLockConfigRuleModel struct {
	DefaultRetention fwtypes.ListNestedObjectValueOf[dataBucketObjectLockConfigDefaultRetentionModel] `tfsdk:"default_retention"`
}

type dataBucketObjectLockConfigDefaultRetentionModel struct {
	Days  types.Int64  `tfsdk:"days"`
	Mode  types.String `tfsdk:"mode"`
	Years types.Int64  `tfsdk:"years"`
}
