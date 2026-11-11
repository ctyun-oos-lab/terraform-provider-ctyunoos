// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package s3

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	awstypes "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/fwdiag"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/framework"
	fwtypes "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/framework/types"
	frameworkvalidators "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/framework/validators"
	fwobjectvalidator "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/framework/validators/objectvalidator"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/retry"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/tfresource"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/aws-sdk-go-base/v2/tfawserr"
	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/int32validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	sdkid "github.com/hashicorp/terraform-plugin-sdk/v2/helper/id"
)

// @FrameworkResource("ctyun_oos_bucket_lifecycle_configuration", name="Bucket Lifecycle Configuration")
// @IdentityAttribute("bucket")
// @Testing(hasNoPreExistingResource=true, importStateIdAttribute="bucket", disableRegionOverrideTest=true)
func newBucketLifecycleConfigurationResource(context.Context) (resource.ResourceWithConfigure, error) {
	return &bucketLifecycleConfigurationResource{}, nil
}

type bucketLifecycleConfigurationResource struct {
	framework.ResourceWithModel[bucketLifecycleConfigurationResourceModel]
	framework.WithImportByIdentity
}

const (
	lifecycleRuleStatusDisabled = "Disabled"
	lifecycleRuleStatusEnabled  = "Enabled"
)

func lifecycleRuleStatus_Values() []string {
	return []string{lifecycleRuleStatusEnabled, lifecycleRuleStatusDisabled}
}

func (r *bucketLifecycleConfigurationResource) Schema(ctx context.Context, request resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			names.AttrBucket: schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthBetween(3, 63),
					stringvalidator.RegexMatches(regexp.MustCompile(`^[a-z0-9.-]+$`), "must contain only lowercase letters, numbers, periods, and hyphens"),
				},
			},
		},
		Blocks: map[string]schema.Block{
			names.AttrRule: schema.ListNestedBlock{
				CustomType: fwtypes.NewListNestedObjectTypeOf[lifecycleRuleModel](ctx),
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						names.AttrID: schema.StringAttribute{
							Optional: true,
							Computed: true, // OOS generates an ID when omitted.
							Validators: []validator.String{
								stringvalidator.LengthBetween(1, 255),
							},
						},
						names.AttrPrefix: schema.StringAttribute{
							Required: true,
							Validators: []validator.String{
								stringvalidator.LengthBetween(0, 1024),
							},
						},
						names.AttrStatus: schema.StringAttribute{
							Required: true,
							Validators: []validator.String{
								stringvalidator.OneOf(lifecycleRuleStatus_Values()...),
							},
						},
					},
					Blocks: map[string]schema.Block{
						"expiration": schema.ListNestedBlock{
							CustomType: fwtypes.NewListNestedObjectTypeOf[lifecycleExpirationModel](ctx),
							Validators: []validator.List{
								listvalidator.SizeAtMost(1),
							},
							NestedObject: schema.NestedBlockObject{
								Validators: []validator.Object{
									fwobjectvalidator.ExactlyOneOfChildren(
										path.MatchRelative().AtName("date"),
										path.MatchRelative().AtName("days"),
									),
								},
								Attributes: map[string]schema.Attribute{
									"date": schema.StringAttribute{
										CustomType: timetypes.RFC3339Type{},
										Optional:   true,
										Validators: []validator.String{
											frameworkvalidators.LifecycleDate(),
										},
									},
									"days": schema.Int32Attribute{
										Optional: true,
										Validators: []validator.Int32{
											int32validator.AtLeast(1),
										},
									},
								},
							},
						},
						// OOS allows at most one transition per rule.
						"transition": schema.SetNestedBlock{
							CustomType: fwtypes.NewSetNestedObjectTypeOf[lifecycleTransitionModel](ctx),
							Validators: []validator.Set{
								setvalidator.SizeAtMost(1),
							},
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"date": schema.StringAttribute{
										CustomType: timetypes.RFC3339Type{},
										Optional:   true,
										Validators: []validator.String{
											frameworkvalidators.LifecycleDate(),
										},
									},
									"days": schema.Int32Attribute{
										Optional: true,
										Validators: []validator.Int32{
											int32validator.AtLeast(1),
										},
									},
									names.AttrStorageClass: schema.StringAttribute{
										Required: true,
										Validators: []validator.String{
											stringvalidator.OneOf(string(awstypes.TransitionStorageClassStandardIa)),
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *bucketLifecycleConfigurationResource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var data bucketLifecycleConfigurationResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &data)...)
	if response.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().S3Client(ctx)
	bucket := data.Bucket.ValueString()
	unlock := lockBucketControlPlane(bucket)
	defer unlock()

	rules, diags := expandLifecycleRules(ctx, data.Rules)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}

	_, err := tfresource.RetryWhenAWSErrCodeEquals(ctx, bucketPropagationTimeout, func(ctx context.Context) (any, error) {
		return nil, oosPutBucketLifecycle(ctx, conn, bucket, rules)
	}, errCodeNoSuchBucket)
	if err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("creating OOS Bucket (%s) Lifecycle Configuration", bucket), err.Error())
		return
	}

	out, err := tfresource.RetryWhenNotFound(ctx, bucketPropagationTimeout, func(ctx context.Context) ([]oosLifecycleRule, error) {
		return findBucketLifecycleConfiguration(ctx, conn, bucket)
	})
	if err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("waiting for OOS Bucket (%s) Lifecycle Configuration create", bucket), err.Error())
		return
	}

	response.Diagnostics.Append(flattenLifecycleRules(ctx, out, &data)...)
	if response.Diagnostics.HasError() {
		return
	}

	response.Diagnostics.Append(response.State.Set(ctx, &data)...)
}

func (r *bucketLifecycleConfigurationResource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var data bucketLifecycleConfigurationResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &data)...)
	if response.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().S3Client(ctx)
	bucket := data.Bucket.ValueString()

	rules, err := findBucketLifecycleConfiguration(ctx, conn, bucket)
	if retry.NotFound(err) {
		response.Diagnostics.Append(fwdiag.NewResourceNotFoundWarningDiagnostic(err))
		response.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("reading OOS Bucket Lifecycle Configuration (%s)", bucket), err.Error())
		return
	}

	response.Diagnostics.Append(flattenLifecycleRules(ctx, rules, &data)...)
	if response.Diagnostics.HasError() {
		return
	}

	response.Diagnostics.Append(response.State.Set(ctx, &data)...)
}

func (r *bucketLifecycleConfigurationResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	var data bucketLifecycleConfigurationResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &data)...)
	if response.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().S3Client(ctx)
	bucket := data.Bucket.ValueString()
	unlock := lockBucketControlPlane(bucket)
	defer unlock()

	rules, diags := expandLifecycleRules(ctx, data.Rules)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}

	_, err := tfresource.RetryWhenAWSErrCodeEquals(ctx, bucketPropagationTimeout, func(ctx context.Context) (any, error) {
		return nil, oosPutBucketLifecycle(ctx, conn, bucket, rules)
	}, errCodeNoSuchBucket)
	if err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("updating OOS Bucket (%s) Lifecycle Configuration", bucket), err.Error())
		return
	}

	out, err := findBucketLifecycleConfiguration(ctx, conn, bucket)
	if err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("reading OOS Bucket Lifecycle Configuration (%s)", bucket), err.Error())
		return
	}

	response.Diagnostics.Append(flattenLifecycleRules(ctx, out, &data)...)
	if response.Diagnostics.HasError() {
		return
	}

	response.Diagnostics.Append(response.State.Set(ctx, &data)...)
}

func (r *bucketLifecycleConfigurationResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var data bucketLifecycleConfigurationResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &data)...)
	if response.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().S3Client(ctx)
	bucket := data.Bucket.ValueString()
	unlock := lockBucketControlPlane(bucket)
	defer unlock()

	_, err := conn.DeleteBucketLifecycle(ctx, &s3.DeleteBucketLifecycleInput{
		Bucket: aws.String(bucket),
	})
	if tfawserr.ErrCodeEquals(err, errCodeNoSuchBucket, errCodeNoSuchLifecycleConfiguration, errCodeMethodNotAllowed) {
		return
	}
	if err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("deleting OOS Bucket Lifecycle Configuration (%s)", bucket), err.Error())
		return
	}

	_, err = tfresource.RetryUntilNotFound(ctx, bucketPropagationTimeout, func(ctx context.Context) (any, error) {
		return findBucketLifecycleConfiguration(ctx, conn, bucket)
	})
	if err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("waiting for OOS Bucket Lifecycle Configuration (%s) delete", bucket), err.Error())
		return
	}
}

// findBucketLifecycleConfiguration retrieves the OOS lifecycle rules, returning a
// NotFoundError when the bucket is missing or has no lifecycle configuration.
func findBucketLifecycleConfiguration(ctx context.Context, conn *s3.Client, bucket string) ([]oosLifecycleRule, error) {
	rules, err := oosGetBucketLifecycle(ctx, conn, bucket)

	if tfawserr.ErrCodeEquals(err, errCodeNoSuchBucket, errCodeNoSuchLifecycleConfiguration) {
		return nil, &retry.NotFoundError{LastError: err}
	}

	if err != nil {
		return nil, err
	}

	if len(rules) == 0 {
		return nil, tfresource.NewEmptyResultError()
	}

	return rules, nil
}

type bucketLifecycleConfigurationResourceModel struct {
	framework.WithRegionModel
	Bucket types.String                                        `tfsdk:"bucket"`
	Rules  fwtypes.ListNestedObjectValueOf[lifecycleRuleModel] `tfsdk:"rule"`
}

type lifecycleRuleModel struct {
	ID         types.String                                              `tfsdk:"id"`
	Prefix     types.String                                              `tfsdk:"prefix"`
	Status     types.String                                              `tfsdk:"status"`
	Expiration fwtypes.ListNestedObjectValueOf[lifecycleExpirationModel] `tfsdk:"expiration"`
	Transition fwtypes.SetNestedObjectValueOf[lifecycleTransitionModel]  `tfsdk:"transition"`
}

type lifecycleExpirationModel struct {
	Date timetypes.RFC3339 `tfsdk:"date"`
	Days types.Int32       `tfsdk:"days"`
}

type lifecycleTransitionModel struct {
	Date         timetypes.RFC3339 `tfsdk:"date"`
	Days         types.Int32       `tfsdk:"days"`
	StorageClass types.String      `tfsdk:"storage_class"`
}

// expandLifecycleRules converts the resource model into OOS lifecycle rules. The
// OOS structs use XML tags (autoflex cannot drive them), so the mapping is done
// by hand: the date attribute is already a full RFC3339 timestamp and is passed
// through verbatim, Days/Date are mutually exclusive, and a missing rule ID is
// generated client-side.
func expandLifecycleRules(ctx context.Context, list fwtypes.ListNestedObjectValueOf[lifecycleRuleModel]) ([]oosLifecycleRule, diag.Diagnostics) {
	var diags diag.Diagnostics

	models, d := list.ToSlice(ctx)
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}

	rules := make([]oosLifecycleRule, 0, len(models))
	for _, m := range models {
		rule := oosLifecycleRule{
			Prefix: m.Prefix.ValueString(),
			Status: m.Status.ValueString(),
		}

		if v := m.ID.ValueString(); v != "" {
			rule.ID = v
		} else {
			rule.ID = sdkid.PrefixedUniqueId("tf-s3-lifecycle-")
		}

		expiration, d := m.Expiration.ToPtr(ctx)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		if expiration != nil {
			rule.Expiration = &oosLifecycleExpiry{}
			if date := expiration.Date.ValueString(); date != "" {
				rule.Expiration.Date = normalizeLifecycleRFC3339(date)
			} else if !expiration.Days.IsNull() {
				rule.Expiration.Days = aws.Int32(expiration.Days.ValueInt32())
			}
		}

		transition, d := m.Transition.ToPtr(ctx)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		if transition != nil {
			transitionHasDate := !transition.Date.IsNull() && !transition.Date.IsUnknown() && transition.Date.ValueString() != ""
			transitionHasDays := !transition.Days.IsNull() && !transition.Days.IsUnknown()
			if transitionHasDate == transitionHasDays {
				diags.AddError(
					"Invalid Lifecycle Transition",
					fmt.Sprintf("rule %q: exactly one of transition date or days is required", rule.ID),
				)
				return nil, diags
			}
			rule.Transition = &oosLifecycleTransit{
				StorageClass: transition.StorageClass.ValueString(),
			}
			if date := transition.Date.ValueString(); date != "" {
				rule.Transition.Date = normalizeLifecycleRFC3339(date)
			} else if !transition.Days.IsNull() {
				rule.Transition.Days = aws.Int32(transition.Days.ValueInt32())
			}
		}

		// OOS requires each rule to carry at least one of Expiration/Transition.
		if rule.Expiration == nil && rule.Transition == nil {
			diags.AddError(
				"Invalid Lifecycle Rule",
				fmt.Sprintf("rule %q: at least one of expiration or transition is required", rule.ID),
			)
			return nil, diags
		}

		rules = append(rules, rule)
	}

	return rules, diags
}

// normalizeLifecycleRFC3339 normalizes equivalent RFC3339 timestamps to a
// stable UTC string representation so import/read paths do not drift on
// formatting-only differences such as `Z` vs `.000Z`.
func normalizeLifecycleRFC3339(v string) string {
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t.UTC().Format("2006-01-02T00:00:00.000Z")
	}

	return v
}

// flattenLifecycleRules maps OOS lifecycle rules back into the resource model.
func flattenLifecycleRules(ctx context.Context, rules []oosLifecycleRule, data *bucketLifecycleConfigurationResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	models := make([]*lifecycleRuleModel, 0, len(rules))
	for _, rule := range rules {
		m := lifecycleRuleModel{
			ID:         types.StringValue(rule.ID),
			Prefix:     types.StringValue(rule.Prefix),
			Status:     types.StringValue(rule.Status),
			Expiration: fwtypes.NewListNestedObjectValueOfNull[lifecycleExpirationModel](ctx),
			Transition: fwtypes.NewSetNestedObjectValueOfNull[lifecycleTransitionModel](ctx),
		}

		if e := rule.Expiration; e != nil {
			em := lifecycleExpirationModel{
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
			tm := lifecycleTransitionModel{
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
			m.Transition = fwtypes.NewSetNestedObjectValueOfPtrMust(ctx, &tm)
		}

		models = append(models, &m)
	}

	list, d := fwtypes.NewListNestedObjectValueOfSlice(ctx, models, nil)
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}
	data.Rules = list

	return diags
}
