// Copyright IBM Corp. 2014, 2026
// Modifications Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package s3

import (
	"context"
	"regexp"

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

// @FrameworkDataSource("ctyun_oos_bucket_server_side_encryption_configuration", name="Bucket Server Side Encryption Configuration")
func newBucketServerSideEncryptionConfigurationDataSource(context.Context) (datasource.DataSourceWithConfigure, error) {
	return &bucketServerSideEncryptionConfigurationDataSource{}, nil
}

const (
	DSNameBucketServerSideEncryptionConfiguration = "Bucket Server Side Encryption Configuration Data Source"
)

type bucketServerSideEncryptionConfigurationDataSource struct {
	framework.DataSourceWithModel[bucketServerSideEncryptionConfigurationDataSourceModel]
}

func (d *bucketServerSideEncryptionConfigurationDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
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
			names.AttrRule: framework.DataSourceComputedListOfObjectAttribute[dataBucketSSEConfigRuleModel](ctx),
		},
	}
}

func (d *bucketServerSideEncryptionConfigurationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	conn := d.Meta().S3Client(ctx)

	var data bucketServerSideEncryptionConfigurationDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucket := data.Bucket.ValueString()

	out, err := findServerSideEncryptionConfiguration(ctx, conn, bucket)
	if err != nil {
		resp.Diagnostics.AddError(
			create.ProblemStandardMessage(names.S3, create.ErrActionReading, DSNameBucketServerSideEncryptionConfiguration, bucket, err),
			err.Error(),
		)
		return
	}

	data.ID = types.StringValue(bucket)
	data.Rule = flattenSSEConfigDataSourceRule(ctx, out)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// flattenSSEConfigDataSourceRule maps the OOS server-side encryption configuration
// into the data source's rule model.
func flattenSSEConfigDataSourceRule(ctx context.Context, cfg *oosServerSideEncryptionConfiguration) fwtypes.ListNestedObjectValueOf[dataBucketSSEConfigRuleModel] {
	if cfg == nil || cfg.Rule == nil || cfg.Rule.ApplyServerSideEncryptionByDefault == nil {
		return fwtypes.NewListNestedObjectValueOfNull[dataBucketSSEConfigRuleModel](ctx)
	}

	byDefault := dataBucketSSEConfigByDefaultModel{
		SSEAlgorithm: types.StringValue(cfg.Rule.ApplyServerSideEncryptionByDefault.SSEAlgorithm),
	}

	ruleModel := dataBucketSSEConfigRuleModel{
		ApplyServerSideEncryptionByDefault: fwtypes.NewListNestedObjectValueOfPtrMust(ctx, &byDefault),
	}

	return fwtypes.NewListNestedObjectValueOfPtrMust(ctx, &ruleModel)
}

type bucketServerSideEncryptionConfigurationDataSourceModel struct {
	framework.WithRegionModel
	ID     types.String                                                  `tfsdk:"id"`
	Bucket types.String                                                  `tfsdk:"bucket"`
	Rule   fwtypes.ListNestedObjectValueOf[dataBucketSSEConfigRuleModel] `tfsdk:"rule"`
}

type dataBucketSSEConfigRuleModel struct {
	ApplyServerSideEncryptionByDefault fwtypes.ListNestedObjectValueOf[dataBucketSSEConfigByDefaultModel] `tfsdk:"apply_server_side_encryption_by_default"`
}

type dataBucketSSEConfigByDefaultModel struct {
	SSEAlgorithm types.String `tfsdk:"sse_algorithm"`
}
