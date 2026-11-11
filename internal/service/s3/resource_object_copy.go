// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package s3

import (
	"context"
	"log"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/enum"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/sdkdiag"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/flex"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/retry"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// @SDKResource("ctyun_oos_object_copy", name="Object Copy")
// @NoImport
func resourceObjectCopy() *schema.Resource {
	return &schema.Resource{
		CreateWithoutTimeout: resourceObjectCopyCreate,
		ReadWithoutTimeout:   resourceObjectCopyRead,
		UpdateWithoutTimeout: resourceObjectCopyUpdate,
		DeleteWithoutTimeout: resourceObjectCopyDelete,

		Schema: map[string]*schema.Schema{
			names.AttrARN: {
				Type:     schema.TypeString,
				Computed: true,
			},
			names.AttrBucket: {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				ValidateFunc: validation.All(
					validation.StringLenBetween(3, 63),
					validation.StringMatch(regexp.MustCompile(`^[a-z0-9.-]+$`), "must contain only lowercase letters, numbers, periods, and hyphens"),
				),
			},
			"cache_control": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"content_disposition": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"content_encoding": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			names.AttrContentType: {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"copy_if_match": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"copy_if_modified_since": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.IsRFC3339Time,
			},
			"copy_if_none_match": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"copy_if_unmodified_since": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.IsRFC3339Time,
			},
			// data_location_constraint maps to the OOS x-ctyun-data-location request header,
			// controlling where the copied object's data is physically placed.
			"data_location_constraint": {
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						names.AttrType: {
							Type:     schema.TypeString,
							Required: true,
							ValidateFunc: validation.StringInSlice([]string{
								oosDataLocationTypeLocal,
								oosDataLocationTypeSpecified,
							}, false),
						},
						"locations": {
							Type:     schema.TypeList,
							Optional: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
						"schedule_strategy": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  oosDefaultScheduleStrategy,
							ValidateFunc: validation.StringInSlice([]string{
								"Allowed",
								"NotAllowed",
							}, false),
						},
					},
				},
			},
			// data_location reflects the OOS x-ctyun-data-location response header:
			// the resolved location where the copied object's data is stored.
			"data_location": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"etag": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"expiration": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"server_side_encryption": {
				Type:     schema.TypeString,
				Computed: true,
			},
			names.AttrKey: {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.NoZeroValues,
			},
			"last_modified": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"metadata": {
				Type:         schema.TypeMap,
				ValidateFunc: validateMetadataIsLowerCase,
				Optional:     true,
				Computed:     true,
				Elem:         &schema.Schema{Type: schema.TypeString},
			},
			"metadata_directive": {
				Type:             schema.TypeString,
				Optional:         true,
				ValidateDiagFunc: enum.Validate[types.MetadataDirective](),
			},
			// metadata_location reflects the OOS x-ctyun-metadata-location response
			// header: the location of the object's index.
			"metadata_location": {
				Type:     schema.TypeString,
				Computed: true,
			},
			names.AttrSource: {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.NoZeroValues,
			},
			names.AttrStorageClass: {
				Type:         schema.TypeString,
				Optional:     true,
				Computed:     true,
				ValidateFunc: validation.StringInSlice([]string{"STANDARD", "STANDARD_IA"}, false),
			},
		},
	}
}

func resourceObjectCopyCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	return append(diags, resourceObjectCopyDoCopy(ctx, d, meta)...)
}

func resourceObjectCopyRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).S3Client(ctx)

	bucket := d.Get(names.AttrBucket).(string)
	key := d.Get(names.AttrKey).(string)

	// Capture the OOS-specific x-ctyun-* response headers (metadata/data
	// location) that the SDK's HeadObjectOutput does not model.
	var oosHeaders oosObjectHeaders
	optFns := []func(*s3.Options){func(o *s3.Options) {
		o.APIOptions = append(o.APIOptions, captureOOSObjectHeadersMiddleware(&oosHeaders))
	}}

	output, err := findObjectByBucketAndKey(ctx, conn, bucket, key, "", "", optFns...)

	if !d.IsNewResource() && retry.NotFound(err) {
		log.Printf("[WARN] S3 Object (%s) not found, removing from state", d.Id())
		d.SetId("")
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading S3 Object (%s): %s", d.Id(), err)
	}

	arn, err := newObjectARN(bucket, key)
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading S3 Object (%s): %s", d.Id(), err)
	}
	d.Set(names.AttrARN, arn.String())

	d.Set("cache_control", output.CacheControl)
	d.Set("content_disposition", output.ContentDisposition)
	d.Set("content_encoding", output.ContentEncoding)
	d.Set(names.AttrContentType, output.ContentType)
	// See https://forums.aws.amazon.com/thread.jspa?threadID=44003
	d.Set("etag", strings.Trim(aws.ToString(output.ETag), `"`))
	d.Set("expiration", output.Expiration)
	d.Set("server_side_encryption", output.ServerSideEncryption)
	d.Set("last_modified", flattenObjectDate(output.LastModified))
	d.Set("metadata", output.Metadata)
	d.Set("data_location", oosHeaders.dataLocation)
	d.Set("metadata_location", oosHeaders.metadataLocation)
	// The "STANDARD" (which is also the default) storage
	// class when set would not be included in the results.
	d.Set(names.AttrStorageClass, types.ObjectStorageClassStandard)
	if output.StorageClass != "" {
		d.Set(names.AttrStorageClass, output.StorageClass)
	}

	return diags
}

func resourceObjectCopyUpdate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics

	// if any of these exist, let the API decide whether to copy
	for _, key := range []string{
		"copy_if_match",
		"copy_if_modified_since",
		"copy_if_none_match",
		"copy_if_unmodified_since",
	} {
		if _, ok := d.GetOk(key); ok {
			return append(diags, resourceObjectCopyDoCopy(ctx, d, meta)...)
		}
	}

	args := []string{
		names.AttrBucket,
		"cache_control",
		"content_disposition",
		"content_encoding",
		names.AttrContentType,
		names.AttrKey,
		"metadata",
		"metadata_directive",
		names.AttrSource,
		names.AttrStorageClass,
	}
	if d.HasChanges(args...) {
		return append(diags, resourceObjectCopyDoCopy(ctx, d, meta)...)
	}

	return diags
}

func resourceObjectCopyDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).S3Client(ctx)

	bucket := d.Get(names.AttrBucket).(string)
	key := d.Get(names.AttrKey).(string)

	// OOS has no object versioning, so a single DELETE Object suffices.
	if err := deleteObjectVersion(ctx, conn, bucket, key, "", false); err != nil {
		return sdkdiag.AppendErrorf(diags, "deleting S3 Bucket (%s) Object (%s): %s", bucket, key, err)
	}

	_, err := findObjectByBucketAndKey(ctx, conn, bucket, key, "", "")
	if err == nil {
		return sdkdiag.AppendErrorf(diags, "checking S3 Bucket (%s) Object (%s) delete: object still exists", bucket, key)
	}
	if !retry.NotFound(err) {
		return sdkdiag.AppendErrorf(diags, "checking S3 Bucket (%s) Object (%s) delete: %s", bucket, key, err)
	}

	return diags
}

func resourceObjectCopyDoCopy(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).S3Client(ctx)

	bucket := d.Get(names.AttrBucket).(string)

	input := &s3.CopyObjectInput{
		Bucket:     aws.String(bucket),
		CopySource: aws.String(d.Get(names.AttrSource).(string)),
		Key:        aws.String(d.Get(names.AttrKey).(string)),
	}

	if v, ok := d.GetOk("cache_control"); ok {
		input.CacheControl = aws.String(v.(string))
	}

	if v, ok := d.GetOk("content_disposition"); ok {
		input.ContentDisposition = aws.String(v.(string))
	}

	if v, ok := d.GetOk("content_encoding"); ok {
		input.ContentEncoding = aws.String(v.(string))
	}

	if v, ok := d.GetOk(names.AttrContentType); ok {
		input.ContentType = aws.String(v.(string))
	}

	if v, ok := d.GetOk("copy_if_match"); ok {
		input.CopySourceIfMatch = aws.String(v.(string))
	}

	if v, ok := d.GetOk("copy_if_modified_since"); ok {
		input.CopySourceIfModifiedSince = expandObjectDate(v.(string))
	}

	if v, ok := d.GetOk("copy_if_none_match"); ok {
		input.CopySourceIfNoneMatch = aws.String(v.(string))
	}

	if v, ok := d.GetOk("copy_if_unmodified_since"); ok {
		input.CopySourceIfUnmodifiedSince = expandObjectDate(v.(string))
	}

	var metadata map[string]string
	if v, ok := d.GetOk("metadata"); ok {
		metadata = flex.ExpandStringValueMap(v.(map[string]any))
	}

	if v, ok := d.GetOk("metadata_directive"); ok {
		input.MetadataDirective = types.MetadataDirective(v.(string))
	}

	if v, ok := d.GetOk(names.AttrStorageClass); ok {
		input.StorageClass = types.StorageClass(v.(string))
	}

	// OOS-specific request headers. data_location_constraint and lowercase x-amz-meta-*
	// metadata are carried via request-scoped middleware (Build step, before
	// signing) so SigV4 covers them.
	optFns := []func(*s3.Options){func(o *s3.Options) {
		// OOS does not support the SDK's default flexible checksums.
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired

		if header := formatDataLocationHeader(expandObjectDataLocation(d.Get("data_location_constraint").([]any))); header != "" {
			o.APIOptions = append(o.APIOptions, setDataLocationHeaderMiddleware(header))
		}
		if len(metadata) > 0 {
			o.APIOptions = append(o.APIOptions, setLowercaseMetadataHeadersMiddleware(metadata))
		}
	}}

	_, err := conn.CopyObject(ctx, input, optFns...)

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "copying %s to S3 Bucket (%s) Object (%s): %s", aws.ToString(input.CopySource), aws.ToString(input.Bucket), aws.ToString(input.Key), err)
	}

	if d.IsNewResource() {
		d.SetId(d.Get(names.AttrKey).(string))
	}

	return append(diags, resourceObjectCopyRead(ctx, d, meta)...)
}
