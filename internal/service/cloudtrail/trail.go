// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package cloudtrail

import (
	"context"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/sdkdiag"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/retry"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/tfresource"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

const propagationTimeout = 2 * time.Minute

// @SDKResource("ctyun_cloudtrail", name="Trail")
// @ArnIdentity
// @Testing(hasNoPreExistingResource=true, disableRegionOverrideTest=true)
func resourceTrail() *schema.Resource {
	return &schema.Resource{
		CreateWithoutTimeout: resourceTrailCreate,
		ReadWithoutTimeout:   resourceTrailRead,
		UpdateWithoutTimeout: resourceTrailUpdate,
		DeleteWithoutTimeout: resourceTrailDelete,

		Schema: map[string]*schema.Schema{
			names.AttrARN: {
				Type:     schema.TypeString,
				Computed: true,
			},
			"enable_logging": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
			"event_selector": eventSelectorResourceSchema(),
			names.AttrName: {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringLenBetween(3, 128),
			},
			names.AttrS3BucketName: {
				Type:     schema.TypeString,
				Required: true,
			},
			names.AttrS3KeyPrefix: {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringLenBetween(0, 200),
			},
		},
	}
}

func resourceTrailCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).CloudTrailClient(ctx)

	name := d.Get(names.AttrName).(string)
	input := &cloudtrail.CreateTrailInput{
		Name:         aws.String(name),
		S3BucketName: aws.String(d.Get(names.AttrS3BucketName).(string)),
	}

	if v, ok := d.GetOk(names.AttrS3KeyPrefix); ok {
		input.S3KeyPrefix = aws.String(v.(string))
	}

	output, err := conn.CreateTrail(ctx, input)
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "creating CloudTrail Trail (%s): %s", name, err)
	}

	d.SetId(aws.ToString(output.TrailARN))

	if _, ok := d.GetOk("event_selector"); ok {
		if err := setEventSelectors(ctx, conn, name, d.Get("event_selector").([]any)); err != nil {
			return sdkdiag.AppendFromErr(diags, err)
		}
	}

	if d.Get("enable_logging").(bool) {
		if err := setLogging(ctx, conn, name, true); err != nil {
			return sdkdiag.AppendFromErr(diags, err)
		}
	}

	return append(diags, resourceTrailRead(ctx, d, meta)...)
}

func resourceTrailRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).CloudTrailClient(ctx)

	trail, err := tfresource.RetryWhenNewResourceNotFound(ctx, propagationTimeout, func(ctx context.Context) (*types.Trail, error) {
		return findTrailByARN(ctx, conn, d.Id())
	}, d.IsNewResource())

	if !d.IsNewResource() && retry.NotFound(err) {
		log.Printf("[WARN] CloudTrail Trail (%s) not found, removing from state", d.Id())
		d.SetId("")
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading CloudTrail Trail (%s): %s", d.Id(), err)
	}

	trailName := aws.ToString(trail.Name)

	d.Set(names.AttrARN, trail.TrailARN)
	d.Set(names.AttrName, trail.Name)
	d.Set(names.AttrS3BucketName, trail.S3BucketName)
	d.Set(names.AttrS3KeyPrefix, trail.S3KeyPrefix)

	status, err := readTrailLoggingStatus(ctx, conn, trailName)
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading CloudTrail Trail (%s) status: %s", d.Id(), err)
	}

	d.Set("enable_logging", status.IsLogging)

	eventSelectors, err := readTrailEventSelectors(ctx, conn, trailName)
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading CloudTrail Trail (%s) event selectors: %s", d.Id(), err)
	}

	if err := d.Set("event_selector", eventSelectors); err != nil {
		return sdkdiag.AppendErrorf(diags, "setting event_selector: %s", err)
	}

	return diags
}

// trailEventSelectorConfigured reports whether the current Terraform config
// explicitly declares the optional event_selector block. Optional+Computed
// values may still be present in ResourceData after Read, so GetOk cannot
// distinguish configuration from state here.
func trailEventSelectorConfigured(d *schema.ResourceData) bool {
	config := d.GetRawConfig()
	if config.IsNull() || !config.IsKnown() {
		return false
	}

	value := config.GetAttr("event_selector")
	return !value.IsNull()
}

func resourceTrailUpdate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).CloudTrailClient(ctx)

	trailName, err := trailAPIName(d)
	if err != nil {
		return sdkdiag.AppendFromErr(diags, err)
	}

	if d.HasChangesExcept("event_selector", "enable_logging") {
		input := &cloudtrail.UpdateTrailInput{
			Name: aws.String(trailName),
		}

		if d.HasChange(names.AttrS3BucketName) {
			input.S3BucketName = aws.String(d.Get(names.AttrS3BucketName).(string))
		}

		if d.HasChange(names.AttrS3KeyPrefix) {
			input.S3KeyPrefix = aws.String(d.Get(names.AttrS3KeyPrefix).(string))
		}

		if _, err := conn.UpdateTrail(ctx, input); err != nil {
			return sdkdiag.AppendErrorf(diags, "updating CloudTrail Trail (%s): %s", d.Id(), err)
		}
	}

	if d.HasChange("enable_logging") && !d.Get("enable_logging").(bool) {
		if err := setLogging(ctx, conn, trailName, false); err != nil {
			return sdkdiag.AppendFromErr(diags, err)
		}
	}

	if d.HasChange("event_selector") && trailEventSelectorConfigured(d) {
		if err := setEventSelectors(ctx, conn, trailName, d.Get("event_selector").([]any)); err != nil {
			return sdkdiag.AppendFromErr(diags, err)
		}
	}

	if d.HasChange("enable_logging") && d.Get("enable_logging").(bool) {
		if err := setLogging(ctx, conn, trailName, true); err != nil {
			return sdkdiag.AppendFromErr(diags, err)
		}
	}

	return append(diags, resourceTrailRead(ctx, d, meta)...)
}

func resourceTrailDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).CloudTrailClient(ctx)

	trailName, err := trailAPIName(d)
	if err != nil {
		return sdkdiag.AppendFromErr(diags, err)
	}

	log.Printf("[DEBUG] Deleting CloudTrail Trail: %s", trailName)
	_, err = conn.DeleteTrail(ctx, &cloudtrail.DeleteTrailInput{
		Name: aws.String(trailName),
	})

	if retry.NotFound(err) {
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "deleting CloudTrail Trail (%s): %s", d.Id(), err)
	}

	return diags
}
