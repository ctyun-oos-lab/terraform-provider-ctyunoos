// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go/middleware"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/sdkdiag"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/flex"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/retry"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/sdkv2"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/tfresource"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/aws-sdk-go-base/v2/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/mitchellh/go-homedir"
)

// @SDKResource("ctyun_oos_object", name="Object")
// @IdentityAttribute("bucket")
// @IdentityAttribute("key")
// @IdAttrFormat("{bucket}/{key}")
// @ImportIDHandler("objectImportID")
// Object identity tests are manually maintained for OOS because they require
// shared-bucket setup rather than the generator's default isolated config model.
// @Testing(identityTest=false)
// @Testing(existsType="github.com/aws/aws-sdk-go-v2/service/s3;s3.GetObjectOutput")
// @Testing(hasNoPreExistingResource=true, disableRegionOverrideTest=true)
func resourceObject() *schema.Resource {
	return &schema.Resource{
		CreateWithoutTimeout: resourceObjectCreate,
		ReadWithoutTimeout:   resourceObjectRead,
		UpdateWithoutTimeout: resourceObjectUpdate,
		DeleteWithoutTimeout: resourceObjectDelete,

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
			},
			names.AttrContent: {
				Type:          schema.TypeString,
				Optional:      true,
				ConflictsWith: []string{names.AttrSource},
			},
			"content_disposition": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"content_md5": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"content_encoding": {
				Type:     schema.TypeString,
				Optional: true,
			},
			names.AttrContentType: {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			// data_location_constraint maps to the OOS x-ctyun-data-location request header,
			// controlling where the object's data is physically placed. It is a
			// write-only policy input and is not surfaced back from the resource.
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
			// the resolved location where the object's data is stored.
			"data_location": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"etag": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"server_side_encryption": {
				Type:     schema.TypeString,
				Computed: true,
			},
			// forbid_overwrite maps to the OOS x-ctyun-forbid-overwrite request
			// header: when true, OOS rejects a PUT whose key already exists. It
			// is a write-time guard, not a persistent object property, and
			// HeadObject does not return it, so Read cannot reconstruct it.
			//
			// It is intentionally NOT ForceNew: forcing a replace would delete
			// then recreate the object, which both destroys data and bypasses the
			// guard (the key no longer exists at recreate time). Instead, changing
			// object content while forbid_overwrite is true re-PUTs with the
			// header set, letting OOS reject the overwrite in place.
			"forbid_overwrite": {
				Type:     schema.TypeBool,
				Optional: true,
			},
			names.AttrKey: {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.NoZeroValues,
			},
			// limit maps to the OOS x-amz-limit request header to throttle upload
			// rate, e.g. "rate=1024" (KiB/s).
			"limit": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"metadata": {
				Type:         schema.TypeMap,
				Optional:     true,
				Computed:     true,
				Elem:         &schema.Schema{Type: schema.TypeString},
				ValidateFunc: validateMetadataIsLowerCase,
			},
			// metadata_location reflects the OOS x-ctyun-metadata-location response
			// header: the location of the object's index.
			"metadata_location": {
				Type:     schema.TypeString,
				Computed: true,
			},
			names.AttrSource: {
				Type:          schema.TypeString,
				Optional:      true,
				ConflictsWith: []string{names.AttrContent},
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

func resourceObjectCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	return append(diags, resourceObjectUpload(ctx, d, meta)...)
}

func resourceObjectRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
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
	d.Set("content_md5", oosHeaders.contentMD5)
	d.Set("content_encoding", output.ContentEncoding)
	d.Set(names.AttrContentType, output.ContentType)
	// See https://forums.aws.amazon.com/thread.jspa?threadID=44003
	d.Set("etag", strings.Trim(aws.ToString(output.ETag), `"`))
	d.Set("server_side_encryption", output.ServerSideEncryption)
	d.Set("metadata", output.Metadata)
	d.Set("data_location", oosHeaders.dataLocation)
	d.Set("metadata_location", oosHeaders.metadataLocation)
	// forbid_overwrite is a write-time header that HeadObject never returns.
	// Re-set it from the current value so the attribute always exists in state
	// (including right after import), avoiding a spurious diff on the next plan.
	d.Set("forbid_overwrite", d.Get("forbid_overwrite"))
	// The "STANDARD" (which is also the default) storage
	// class when set would not be included in the results.
	d.Set(names.AttrStorageClass, types.ObjectStorageClassStandard)
	if output.StorageClass != "" {
		d.Set(names.AttrStorageClass, output.StorageClass)
	}

	return diags
}

func resourceObjectUpdate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics

	// OOS has no per-attribute object mutation APIs (no PutObjectAcl /
	// PutObjectLegalHold / PutObjectRetention). Every mutable attribute is a
	// content-change attribute, so an update is always a re-upload (PUT Object).
	if hasObjectContentChanges(d) {
		return append(diags, resourceObjectUpload(ctx, d, meta)...)
	}

	return append(diags, resourceObjectRead(ctx, d, meta)...)
}

func resourceObjectDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
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

func resourceObjectUpload(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).S3Client(ctx)

	var body io.ReadSeeker

	if v, ok := d.GetOk(names.AttrSource); ok {
		source := v.(string)
		path, err := homedir.Expand(source)
		if err != nil {
			return sdkdiag.AppendErrorf(diags, "expanding homedir in source (%s): %s", source, err)
		}
		file, err := os.Open(path)
		if err != nil {
			return sdkdiag.AppendErrorf(diags, "opening S3 object source (%s): %s", path, err)
		}

		body = file
		defer func() {
			err := file.Close()
			if err != nil {
				log.Printf("[WARN] Error closing S3 object source (%s): %s", path, err)
			}
		}()
	} else if v, ok := d.GetOk(names.AttrContent); ok {
		body = strings.NewReader(v.(string))
	} else {
		body = bytes.NewReader([]byte{})
	}

	input := &s3.PutObjectInput{
		Body:   body,
		Bucket: aws.String(d.Get(names.AttrBucket).(string)),
		Key:    aws.String(d.Get(names.AttrKey).(string)),
	}

	if v, ok := d.GetOk("cache_control"); ok {
		input.CacheControl = aws.String(v.(string))
	}

	if v, ok := d.GetOk("content_disposition"); ok {
		input.ContentDisposition = aws.String(v.(string))
	}

	if v, ok := d.GetOk("content_md5"); ok {
		input.ContentMD5 = aws.String(v.(string))
	}

	if v, ok := d.GetOk("content_encoding"); ok {
		input.ContentEncoding = aws.String(v.(string))
	}

	if v, ok := d.GetOk(names.AttrContentType); ok {
		input.ContentType = aws.String(v.(string))
	}

	var metadata map[string]string
	if v, ok := d.GetOk("metadata"); ok {
		metadata = flex.ExpandStringValueMap(v.(map[string]any))
	}

	if v, ok := d.GetOk(names.AttrStorageClass); ok {
		input.StorageClass = types.StorageClass(v.(string))
	}

	// OOS-specific request headers. data_location_constraint and limit are carried via
	// request-scoped middleware (Build step, before signing) so SigV4 covers
	// them; forbid_overwrite maps to the x-ctyun-forbid-overwrite header.
	var apiOptions []func(*middleware.Stack) error

	if header := formatDataLocationHeader(expandObjectDataLocation(d.Get("data_location_constraint").([]any))); header != "" {
		apiOptions = append(apiOptions, setDataLocationHeaderMiddleware(header))
	}

	if v, ok := d.GetOk("limit"); ok {
		apiOptions = append(apiOptions, setRequestHeaderMiddleware("OOSSetLimit", "x-amz-limit", v.(string)))
	}

	if d.Get("forbid_overwrite").(bool) {
		apiOptions = append(apiOptions, setRequestHeaderMiddleware("OOSSetForbidOverwrite", "x-ctyun-forbid-overwrite", "true"))
	}

	if len(metadata) > 0 {
		apiOptions = append(apiOptions, setLowercaseMetadataHeadersMiddleware(metadata))
	}

	optFns := []func(*s3.Options){func(o *s3.Options) {
		// OOS does not support the SDK's default flexible checksums
		// (x-amz-checksum-*). PutObject is not a RequireChecksum operation, so
		// opting out of "when supported" suppresses the checksum headers
		// entirely without needing the Content-MD5 swap used by bucket ops.
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.APIOptions = append(o.APIOptions, apiOptions...)
	}}

	if _, err := conn.PutObject(ctx, input, optFns...); err != nil {
		return sdkdiag.AppendErrorf(diags, "uploading S3 Object (%s) to Bucket (%s): %s", aws.ToString(input.Key), aws.ToString(input.Bucket), err)
	}

	if d.IsNewResource() {
		d.SetId(createObjectImportID(d))
	}

	return append(diags, resourceObjectRead(ctx, d, meta)...)
}

func validateMetadataIsLowerCase(v any, k string) (ws []string, errors []error) {
	value := v.(map[string]any)

	for k := range value {
		if k != strings.ToLower(k) {
			errors = append(errors, fmt.Errorf(
				"Metadata must be lowercase only. Offending key: %q", k))
		}
	}
	return
}

func hasObjectContentChanges(d sdkv2.ResourceDiffer) bool {
	return slices.ContainsFunc([]string{
		"cache_control",
		"content_disposition",
		"content_md5",
		"content_encoding",
		names.AttrContentType,
		names.AttrContent,
		"data_location_constraint",
		"etag",
		// forbid_overwrite is deliberately excluded: toggling it alone must not
		// trigger a re-upload. Enabling it (false->true) on an existing object
		// would otherwise PUT with the guard header and be rejected by OOS,
		// making the object impossible to protect. Real content changes
		// (metadata/content/etc.) still re-PUT and carry the current header value.
		"limit",
		"metadata",
		names.AttrSource,
		names.AttrStorageClass,
	}, d.HasChange)
}

// expandObjectDataLocation converts the data_location_constraint nested block into the
// internal policy struct used to build the x-ctyun-data-location header. Returns
// nil when the block is absent.
func expandObjectDataLocation(l []any) *oosObjectDataLocation {
	if len(l) == 0 || l[0] == nil {
		return nil
	}

	tfMap := l[0].(map[string]any)
	out := &oosObjectDataLocation{
		Type:             tfMap[names.AttrType].(string),
		ScheduleStrategy: tfMap["schedule_strategy"].(string),
	}
	if v, ok := tfMap["locations"].([]any); ok {
		out.Locations = flex.ExpandStringValueList(v)
	}

	return out
}

func findObjectByBucketAndKey(ctx context.Context, conn *s3.Client, bucket, key, etag, checksumAlgorithm string, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	input := &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}
	if checksumAlgorithm != "" {
		input.ChecksumMode = types.ChecksumModeEnabled
	}
	if etag != "" {
		input.IfMatch = aws.String(etag)
	}

	return findObject(ctx, conn, input, optFns...)
}

func findObject(ctx context.Context, conn *s3.Client, input *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	output, err := conn.HeadObject(ctx, input, optFns...)

	if tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
		return nil, &retry.NotFoundError{
			LastError: err,
		}
	}

	if err != nil {
		return nil, err
	}

	if output == nil {
		return nil, tfresource.NewEmptyResultError()
	}

	return output, nil
}

func expandObjectDate(v string) *time.Time {
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return nil
	}

	return aws.Time(t)
}

func flattenObjectDate(t *time.Time) string {
	if t == nil {
		return ""
	}

	return t.Format(time.RFC3339)
}

func createObjectImportID(d *schema.ResourceData) string {
	return fmt.Sprintf("%s/%s", d.Get(names.AttrBucket).(string), d.Get(names.AttrKey).(string))
}

type objectImportID struct{}

func (objectImportID) Create(d *schema.ResourceData) string {
	return createObjectImportID(d)
}

func (objectImportID) Parse(id string) (string, map[string]any, error) {
	id = strings.TrimPrefix(id, "s3://")

	bucket, key, found := strings.Cut(id, "/")
	if !found {
		return "", nil, fmt.Errorf("id \"%s\" should be in the format <bucket>/<key> or s3://<bucket>/<key>", id)
	}

	result := map[string]any{
		names.AttrBucket: bucket,
		names.AttrKey:    key,
	}
	return id, result, nil
}
