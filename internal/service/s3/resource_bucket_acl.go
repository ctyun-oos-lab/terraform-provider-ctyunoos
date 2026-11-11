// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package s3

import (
	"context"
	"fmt"
	"log"
	"regexp"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	awstypes "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/sdkdiag"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/retry"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/tfresource"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// @SDKResource("ctyun_oos_bucket_acl", name="Bucket ACL")
// @IdentityAttribute("bucket")
// @Testing(checkDestroyNoop=true)
// @Testing(hasNoPreExistingResource=true, disableRegionOverrideTest=true)
// @Testing(plannableImportAction="NoOp")
func resourceBucketACL() *schema.Resource {
	return &schema.Resource{
		CreateWithoutTimeout: resourceBucketACLCreate,
		ReadWithoutTimeout:   resourceBucketACLRead,
		UpdateWithoutTimeout: resourceBucketACLUpdate,
		DeleteWithoutTimeout: schema.NoopContext,
		CustomizeDiff: func(_ context.Context, d *schema.ResourceDiff, _ any) error {
			if d.HasChange("acl") {
				if err := d.SetNewComputed("access_control_policy"); err != nil {
					return err
				}
			}
			return nil
		},

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
			// OOS supports only canned ACLs (no per-grantee grants / owner). The
			// value is read back by mapping the bucket's AllUsers grants to the
			// closest canned ACL, see cannedACLFromGrants.
			"acl": {
				Type:         schema.TypeString,
				Optional:     true,
				Computed:     true,
				ValidateFunc: validation.StringInSlice([]string{"private", "public-read", "public-read-write"}, false),
			},
			"access_control_policy": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"owner": {
							Type:     schema.TypeList,
							Computed: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									names.AttrDisplayName: {
										Type:     schema.TypeString,
										Computed: true,
									},
									names.AttrID: {
										Type:     schema.TypeString,
										Computed: true,
									},
								},
							},
						},
						"grant": {
							Type:     schema.TypeList,
							Computed: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"permission": {
										Type:     schema.TypeString,
										Computed: true,
									},
									"grantee": {
										Type:     schema.TypeList,
										Computed: true,
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												names.AttrType: {
													Type:     schema.TypeString,
													Computed: true,
												},
												names.AttrURI: {
													Type:     schema.TypeString,
													Computed: true,
												},
											},
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

func resourceBucketACLCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).S3Client(ctx)

	bucket := d.Get(names.AttrBucket).(string)
	unlock := lockBucketControlPlane(bucket)
	defer unlock()
	acl := d.Get("acl").(string)
	if acl == "" {
		acl = "private"
	}

	if _, err := FindOOSBucket(ctx, conn, bucket); err != nil {
		return sdkdiag.AppendErrorf(diags, "reading OOS Bucket (%s): %s", bucket, err)
	}

	// OOS sets a bucket's ACL by re-issuing PUT /{bucket} with the x-amz-acl
	// header (there is no separate, OOS-accepted PUT ?acl request: the standard
	// one forces an x-amz-checksum-crc32 OOS rejects). oosPutBucket on an
	// existing bucket only updates the ACL/data position.
	_, err := tfresource.RetryWhenAWSErrCodeEquals(ctx, bucketPropagationTimeout, func(ctx context.Context) (any, error) {
		return nil, oosPutBucket(ctx, conn, oosCreateBucketInput{bucket: bucket, acl: acl})
	}, errCodeNoSuchBucket)

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "creating OOS Bucket (%s) ACL: %s", bucket, err)
	}

	d.SetId(bucket)

	_, err = tfresource.RetryWhenNotFound(ctx, bucketPropagationTimeout, func(ctx context.Context) (any, error) {
		return findOOSBucketACL(ctx, conn, bucket)
	})

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "waiting for OOS Bucket ACL (%s) create: %s", d.Id(), err)
	}

	return append(diags, resourceBucketACLRead(ctx, d, meta)...)
}

func resourceBucketACLRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).S3Client(ctx)

	bucket, _, err := parseBucketACLResourceID(d.Id())
	if err != nil {
		return sdkdiag.AppendFromErr(diags, err)
	}

	output, err := findOOSBucketACL(ctx, conn, bucket)

	if !d.IsNewResource() && retry.NotFound(err) {
		log.Printf("[WARN] OOS Bucket ACL (%s) not found, removing from state", d.Id())
		d.SetId("")
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading OOS Bucket ACL (%s): %s", d.Id(), err)
	}

	acl := cannedACLFromGrants(output)

	if err := d.Set("access_control_policy", flattenBucketACLPolicy(output)); err != nil {
		return sdkdiag.AppendErrorf(diags, "setting access_control_policy for OOS Bucket ACL (%s): %s", d.Id(), err)
	}
	if err := d.Set("acl", acl); err != nil {
		return sdkdiag.AppendErrorf(diags, "setting acl for OOS Bucket ACL (%s): %s", d.Id(), err)
	}
	if err := d.Set(names.AttrBucket, bucket); err != nil {
		return sdkdiag.AppendErrorf(diags, "setting bucket for OOS Bucket ACL (%s): %s", d.Id(), err)
	}
	d.SetId(bucket)

	return diags
}

func resourceBucketACLUpdate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).S3Client(ctx)

	bucket, acl, err := parseBucketACLResourceID(d.Id())
	if err != nil {
		return sdkdiag.AppendFromErr(diags, err)
	}
	unlock := lockBucketControlPlane(bucket)
	defer unlock()

	if d.HasChange("acl") {
		acl = d.Get("acl").(string)

		_, err = tfresource.RetryWhenAWSErrCodeEquals(ctx, bucketPropagationTimeout, func(ctx context.Context) (any, error) {
			return nil, oosPutBucket(ctx, conn, oosCreateBucketInput{bucket: bucket, acl: acl})
		}, errCodeNoSuchBucket)

		if err != nil {
			return sdkdiag.AppendErrorf(diags, "updating OOS Bucket ACL (%s): %s", d.Id(), err)
		}

		d.SetId(bucket)
	}

	return append(diags, resourceBucketACLRead(ctx, d, meta)...)
}

// parseBucketACLResourceID parses the resource ID into the bucket name and
// (optional) canned ACL value.
func parseBucketACLResourceID(id string) (string, string, error) {
	// Bucket name only, e.g. my-bucket or My_Bucket.
	// Reference: https://docs.aws.amazon.com/AmazonS3/latest/userguide/bucketnamingrules.html
	bucketRegex := regexache.MustCompile(`^([0-9a-z.-]{1,63}|[0-9A-Za-z_.-]{1,255})$`)

	if bucketRegex.MatchString(id) {
		return id, "", nil
	}

	return "", "", fmt.Errorf("unexpected format for ID (%s), expected BUCKET", id)
}

func flattenBucketACLPolicy(acl *awss3.GetBucketAclOutput) []any {
	if acl == nil {
		return []any{}
	}

	tfMap := map[string]any{
		"grant": flattenBucketACLGrants(acl.Grants),
		"owner": flattenBucketACLOwner(acl.Owner),
	}

	return []any{tfMap}
}

func flattenBucketACLGrants(grants []awstypes.Grant) []any {
	if len(grants) == 0 {
		return []any{}
	}

	result := make([]any, 0, len(grants))

	for _, grant := range grants {
		tfMap := map[string]any{
			"grantee":    flattenBucketACLGrantee(grant.Grantee),
			"permission": string(grant.Permission),
		}

		result = append(result, tfMap)
	}

	return result
}

func flattenBucketACLGrantee(grantee *awstypes.Grantee) []any {
	if grantee == nil {
		return []any{}
	}

	tfMap := map[string]any{
		names.AttrType: string(grantee.Type),
		names.AttrURI:  aws.ToString(grantee.URI),
	}

	return []any{tfMap}
}

func flattenBucketACLOwner(owner *awstypes.Owner) []any {
	if owner == nil {
		return []any{}
	}

	tfMap := map[string]any{
		names.AttrDisplayName: aws.ToString(owner.DisplayName),
		names.AttrID:          aws.ToString(owner.ID),
	}

	return []any{tfMap}
}
