// Copyright IBM Corp. 2014, 2026
// Modifications Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
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
	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// @FrameworkDataSource("ctyun_oos_bucket_lifecycle_configuration", name="Bucket Lifecycle Configuration")
func newBucketLifecycleConfigurationDataSource(context.Context) (datasource.DataSourceWithConfigure, error) {
	return &bucketLifecycleConfigurationDataSource{}, nil
}

const (
	DSNameBucketLifecycleConfiguration = "Bucket Lifecycle Configuration Data Source"
)

type bucketLifecycleConfigurationDataSource struct {
	framework.DataSourceWithModel[bucketLifecycleConfigurationDataSourceModel]
}

func (d *bucketLifecycleConfigurationDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
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
			names.AttrRule: framework.DataSourceComputedListOfObjectAttribute[dataBucketLifecycleConfigRuleModel](ctx),
		},
	}
}

func (d *bucketLifecycleConfigurationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	conn := d.Meta().S3Client(ctx)

	var data bucketLifecycleConfigurationDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucket := data.Bucket.ValueString()

	rules, err := findBucketLifecycleConfiguration(ctx, conn, bucket)
	if err != nil {
		resp.Diagnostics.AddError(
			create.ProblemStandardMessage(names.S3, create.ErrActionReading, DSNameBucketLifecycleConfiguration, bucket, err),
			err.Error(),
		)
		return
	}

	data.ID = types.StringValue(bucket)
	data.Rule = flattenLifecycleConfigDataSourceRules(ctx, rules)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// flattenLifecycleConfigDataSourceRules maps OOS lifecycle rules into the data
// source's rule model.
func flattenLifecycleConfigDataSourceRules(ctx context.Context, rules []oosLifecycleRule) fwtypes.ListNestedObjectValueOf[dataBucketLifecycleConfigRuleModel] {
	models := make([]*dataBucketLifecycleConfigRuleModel, 0, len(rules))

	for _, rule := range rules {
		m := dataBucketLifecycleConfigRuleModel{
			ID:         types.StringValue(rule.ID),
			Prefix:     types.StringValue(rule.Prefix),
			Status:     types.StringValue(rule.Status),
			Expiration: fwtypes.NewListNestedObjectValueOfNull[dataBucketLifecycleConfigExpirationModel](ctx),
			Transition: fwtypes.NewListNestedObjectValueOfNull[dataBucketLifecycleConfigTransitionModel](ctx),
		}

		if e := rule.Expiration; e != nil {
			em := dataBucketLifecycleConfigExpirationModel{
				Date: timetypes.NewRFC3339Null(),
				Days: types.Int32Null(),
			}
			if e.Date != "" {
				em.Date = timetypes.NewRFC3339ValueMust(normalizeLifecycleRFC3339(e.Date))
			}
			if e.Days != nil {
				em.Days = types.Int32Value(aws.ToInt32(e.Days))
			}
			m.Expiration = fwtypes.NewListNestedObjectValueOfPtrMust(ctx, &em)
		}

		if t := rule.Transition; t != nil {
			tm := dataBucketLifecycleConfigTransitionModel{
				Date:         timetypes.NewRFC3339Null(),
				Days:         types.Int32Null(),
				StorageClass: types.StringValue(t.StorageClass),
			}
			if t.Date != "" {
				tm.Date = timetypes.NewRFC3339ValueMust(normalizeLifecycleRFC3339(t.Date))
			}
			if t.Days != nil {
				tm.Days = types.Int32Value(aws.ToInt32(t.Days))
			}
			m.Transition = fwtypes.NewListNestedObjectValueOfPtrMust(ctx, &tm)
		}

		models = append(models, &m)
	}

	return fwtypes.NewListNestedObjectValueOfSliceMust(ctx, models)
}

type bucketLifecycleConfigurationDataSourceModel struct {
	framework.WithRegionModel
	ID     types.String                                                        `tfsdk:"id"`
	Bucket types.String                                                        `tfsdk:"bucket"`
	Rule   fwtypes.ListNestedObjectValueOf[dataBucketLifecycleConfigRuleModel] `tfsdk:"rule"`
}

type dataBucketLifecycleConfigRuleModel struct {
	ID         types.String                                                              `tfsdk:"id"`
	Prefix     types.String                                                              `tfsdk:"prefix"`
	Status     types.String                                                              `tfsdk:"status"`
	Expiration fwtypes.ListNestedObjectValueOf[dataBucketLifecycleConfigExpirationModel] `tfsdk:"expiration"`
	Transition fwtypes.ListNestedObjectValueOf[dataBucketLifecycleConfigTransitionModel] `tfsdk:"transition"`
}

type dataBucketLifecycleConfigExpirationModel struct {
	Date timetypes.RFC3339 `tfsdk:"date"`
	Days types.Int32       `tfsdk:"days"`
}

type dataBucketLifecycleConfigTransitionModel struct {
	Date         timetypes.RFC3339 `tfsdk:"date"`
	Days         types.Int32       `tfsdk:"days"`
	StorageClass types.String      `tfsdk:"storage_class"`
}
