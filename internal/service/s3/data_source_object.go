// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package s3

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/sdkdiag"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/sdkv2/types/nullable"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// @SDKDataSource("ctyun_oos_object", name="Object")
func dataSourceObject() *schema.Resource {
	return &schema.Resource{
		ReadWithoutTimeout: dataSourceObjectRead,

		Schema: map[string]*schema.Schema{
			names.AttrARN: {
				Type:     schema.TypeString,
				Computed: true,
			},
			"body": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"body_base64": {
				Type:     schema.TypeString,
				Computed: true,
			},
			names.AttrBucket: {
				Type:     schema.TypeString,
				Required: true,
				ValidateFunc: validation.All(
					validation.StringLenBetween(3, 63),
					validation.StringMatch(regexp.MustCompile(`^[a-z0-9.-]+$`), "must contain only lowercase letters, numbers, periods, and hyphens"),
				),
			},
			"cache_control": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"content_disposition": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"content_encoding": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"content_length": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			names.AttrContentType: {
				Type:     schema.TypeString,
				Computed: true,
			},
			"download_body": {
				Type:         nullable.TypeNullableBool,
				Optional:     true,
				ValidateFunc: nullable.ValidateTypeStringNullableBool,
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
			"expires": {
				Type:     schema.TypeString,
				Computed: true,
			},
			names.AttrKey: {
				Type:     schema.TypeString,
				Required: true,
			},
			"last_modified": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"metadata": {
				Type:     schema.TypeMap,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			// data_location reflects the OOS x-ctyun-data-location response header.
			"data_location": {
				Type:     schema.TypeString,
				Computed: true,
			},
			// metadata_location reflects the OOS x-ctyun-metadata-location response header.
			"metadata_location": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"range": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringMatch(regexp.MustCompile(`^bytes=(?:\d+-\d*|-\d+)$`), "must be a valid HTTP byte range"),
			},
			names.AttrStorageClass: {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func dataSourceObjectRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).S3Client(ctx)

	bucket := d.Get(names.AttrBucket).(string)
	key := d.Get(names.AttrKey).(string)
	input := s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	// Capture the OOS-specific x-ctyun-* response headers (metadata/data location).
	var oosHeaders oosObjectHeaders
	optFns := []func(*s3.Options){func(o *s3.Options) {
		o.APIOptions = append(o.APIOptions, captureOOSObjectHeadersMiddleware(&oosHeaders))
	}}

	output, err := findObject(ctx, conn, &input, optFns...)

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading S3 Bucket (%s) Object (%s): %s", bucket, key, err)
	}

	if err := objectDeleteMarkerError(bucket, key, aws.ToBool(output.DeleteMarker)); err != nil {
		return sdkdiag.AppendErrorf(diags, "%s", err)
	}

	id := bucket + "/" + d.Get(names.AttrKey).(string)
	d.SetId(id)

	arn, err := newObjectARN(bucket, key)
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading S3 Bucket (%s) Object (%s): %s", bucket, key, err)
	}
	d.Set(names.AttrARN, arn.String())

	d.Set("cache_control", output.CacheControl)
	d.Set("content_disposition", output.ContentDisposition)
	d.Set("content_encoding", output.ContentEncoding)
	d.Set("content_length", output.ContentLength)
	d.Set(names.AttrContentType, output.ContentType)
	// See https://forums.aws.amazon.com/thread.jspa?threadID=44003
	d.Set("etag", strings.Trim(aws.ToString(output.ETag), `"`))
	d.Set("expiration", output.Expiration)
	d.Set("server_side_encryption", output.ServerSideEncryption)
	d.Set("expires", output.ExpiresString) // formatted in RFC1123
	if output.LastModified != nil {
		d.Set("last_modified", output.LastModified.Format(time.RFC1123))
	} else {
		d.Set("last_modified", nil)
	}
	d.Set("metadata", output.Metadata)
	d.Set("data_location", oosHeaders.dataLocation)
	d.Set("metadata_location", oosHeaders.metadataLocation)
	// The "STANDARD" (which is also the default) storage
	// class when set would not be included in the results.
	d.Set(names.AttrStorageClass, types.ObjectStorageClassStandard)
	if output.StorageClass != "" {
		d.Set(names.AttrStorageClass, output.StorageClass)
	}

	downloadBody, downloadBodyNull, err := nullable.Bool(d.Get("download_body").(string)).ValueBool()
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading S3 Bucket (%s) Object (%s): %s", bucket, key, err)
	}

	// skip object download if download_body is explicitly set to false
	if !downloadBodyNull && !downloadBody {
		return diags
	}

	if downloadBody || isContentTypeAllowed(output.ContentType) {
		inputObjectDownload := s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		}
		if v, ok := d.GetOk("range"); ok {
			inputObjectDownload.Range = aws.String(v.(string))
		}

		objectOutput, err := conn.GetObject(ctx, &inputObjectDownload)
		if err != nil {
			return sdkdiag.AppendErrorf(diags, "downloading S3 Bucket (%s) Object (%s): %s", bucket, key, err)
		}
		defer objectOutput.Body.Close()

		body, err := io.ReadAll(objectOutput.Body)
		if err != nil {
			return sdkdiag.AppendErrorf(diags, "reading S3 Bucket (%s) Object (%s) body: %s", bucket, key, err)
		}

		if downloadBody {
			base64Body := base64.StdEncoding.EncodeToString(body)
			d.Set("body_base64", base64Body)
		}

		if isContentTypeAllowed(output.ContentType) {
			d.Set("body", string(body))
		}
	}

	return diags
}

func objectDeleteMarkerError(bucket, key string, deleteMarker bool) error {
	if !deleteMarker {
		return nil
	}

	return fmt.Errorf("S3 Bucket (%s) Object (%s) has been deleted", bucket, key)
}

// This is to prevent potential issues w/ binary files and generally unprintable characters.
// See https://github.com/hashicorp/terraform/pull/3858#issuecomment-156856738.
func isContentTypeAllowed(contentType *string) bool {
	if contentType == nil {
		return false
	}

	allowedContentTypes := []*regexp.Regexp{
		regexache.MustCompile(`^application/atom\+xml$`),
		regexache.MustCompile(`^application/json$`),
		regexache.MustCompile(`^application/ld\+json$`),
		regexache.MustCompile(`^application/x-csh$`),
		regexache.MustCompile(`^application/x-httpd-php$`),
		regexache.MustCompile(`^application/x-sh$`),
		regexache.MustCompile(`^application/xhtml\+xml$`),
		regexache.MustCompile(`^application/xml$`),
		regexache.MustCompile(`^application/x-sql$`),
		regexache.MustCompile(`^application/yaml$`),
		regexache.MustCompile(`^text/.+`),
	}
	for _, r := range allowedContentTypes {
		if r.MatchString(aws.ToString(contentType)) {
			return true
		}
	}

	return false
}
