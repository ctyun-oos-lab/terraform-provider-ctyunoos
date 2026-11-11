// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package s3

import (
	"context"
	"fmt"
	"log"
	"regexp"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/sdkdiag"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/retry"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/sdkv2"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/tfresource"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/verify"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/aws-sdk-go-base/v2/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/structure"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// @SDKResource("ctyun_oos_bucket_policy", name="Bucket Policy")
// @IdentityAttribute("bucket")
// @Testing(hasNoPreExistingResource=true, disableRegionOverrideTest=true)
func resourceBucketPolicy() *schema.Resource {
	return &schema.Resource{
		CreateWithoutTimeout: resourceBucketPolicyPut,
		ReadWithoutTimeout:   resourceBucketPolicyRead,
		UpdateWithoutTimeout: resourceBucketPolicyPut,
		DeleteWithoutTimeout: resourceBucketPolicyDelete,

		Schema: map[string]*schema.Schema{
			names.AttrBucket: {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				ValidateFunc: validation.All(
					validation.StringLenBetween(3, 63),
					validation.StringMatch(regexp.MustCompile(`^[a-z0-9.-]+$`), "must contain only lowercase letters, numbers, periods, and hyphens"),
				),
			},
			names.AttrPolicy: sdkv2.IAMPolicyDocumentSchemaRequired(),
		},
	}
}

func resourceBucketPolicyPut(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).S3Client(ctx)

	policy, err := structure.NormalizeJsonString(d.Get(names.AttrPolicy).(string))
	if err != nil {
		return sdkdiag.AppendFromErr(diags, err)
	}

	bucket := d.Get(names.AttrBucket).(string)
	unlock := lockBucketControlPlane(bucket)
	defer unlock()

	// OOS PUT Bucket Policy requires the legacy Content-MD5 header (it rejects the
	// SDK's default x-amz-checksum-crc32); oosPutBucketPolicy installs it.
	_, err = tfresource.RetryWhenAWSErrCodeEquals(ctx, bucketPropagationTimeout, func(ctx context.Context) (any, error) {
		return nil, oosPutBucketPolicy(ctx, conn, bucket, policy)
	}, errCodeMalformedPolicy, errCodeNoSuchBucket)

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "putting OOS Bucket (%s) Policy: %s", bucket, err)
	}

	if d.IsNewResource() {
		d.SetId(bucket)

		_, err = tfresource.RetryWhenNotFound(ctx, bucketPropagationTimeout, func(ctx context.Context) (any, error) {
			return findBucketPolicy(ctx, conn, bucket)
		})

		if err != nil {
			return sdkdiag.AppendErrorf(diags, "waiting for OOS Bucket Policy (%s) create: %s", d.Id(), err)
		}
	}

	return append(diags, resourceBucketPolicyRead(ctx, d, meta)...)
}

func resourceBucketPolicyRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).S3Client(ctx)

	bucket := d.Id()

	policy, err := findBucketPolicy(ctx, conn, bucket)

	if !d.IsNewResource() && retry.NotFound(err) {
		log.Printf("[WARN] OOS Bucket Policy (%s) not found, removing from state", d.Id())
		d.SetId("")
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading OOS Bucket Policy (%s): %s", d.Id(), err)
	}

	policy, err = verify.PolicyToSet(d.Get(names.AttrPolicy).(string), policy)
	if err != nil {
		return sdkdiag.AppendFromErr(diags, err)
	}

	err = resourceBucketPolicyFlatten(ctx, policy, d)
	if err != nil {
		return sdkdiag.AppendFromErr(diags, err)
	}

	return diags
}

func resourceBucketPolicyDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).S3Client(ctx)

	bucket := d.Id()
	unlock := lockBucketControlPlane(bucket)
	defer unlock()

	log.Printf("[DEBUG] Deleting OOS Bucket Policy: %s", d.Id())
	_, err := conn.DeleteBucketPolicy(ctx, &s3.DeleteBucketPolicyInput{
		Bucket: aws.String(bucket),
	})

	if tfawserr.ErrCodeEquals(err, errCodeNoSuchBucket) {
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "deleting OOS Bucket Policy (%s): %s", d.Id(), err)
	}

	_, err = tfresource.RetryUntilNotFound(ctx, bucketPropagationTimeout, func(ctx context.Context) (any, error) {
		return findBucketPolicy(ctx, conn, bucket)
	})

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "waiting for OOS Bucket Policy (%s) delete: %s", d.Id(), err)
	}

	return diags
}

func findBucketPolicy(ctx context.Context, conn *s3.Client, bucket string) (string, error) {
	input := &s3.GetBucketPolicyInput{
		Bucket: aws.String(bucket),
	}

	output, err := conn.GetBucketPolicy(ctx, input)

	if tfawserr.ErrCodeEquals(err, errCodeNoSuchBucket, errCodeNoSuchBucketPolicy, errCodeNotSuchBucketPolicy) {
		return "", &retry.NotFoundError{
			LastError: err,
		}
	}

	if err != nil {
		return "", err
	}

	if output == nil || output.Policy == nil {
		return "", tfresource.NewEmptyResultError()
	}

	return aws.ToString(output.Policy), nil
}

func resourceBucketPolicyFlatten(_ context.Context, policy string, d *schema.ResourceData) error {
	policy, err := structure.NormalizeJsonString(policy)
	if err != nil {
		return fmt.Errorf("could not normalize policy JSON: %w", err)
	}
	d.Set(names.AttrPolicy, policy)
	return nil
}
