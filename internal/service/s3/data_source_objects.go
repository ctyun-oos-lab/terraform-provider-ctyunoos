// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package s3

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/enum"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/sdkdiag"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

const keyRequestPageSize = 1000

// @SDKDataSource("ctyun_oos_objects", name="Objects")
func dataSourceObjects() *schema.Resource {
	return &schema.Resource{
		ReadWithoutTimeout: dataSourceObjectsRead,

		// OOS implements the legacy GET Bucket (List Objects) V1 API: it pages with
		// marker / NextMarker (not the V2 continuation token), delimiter only
		// supports "/", encoding_type only supports "url", and the response carries
		// no requester-pays metadata. The V2-only inputs (fetch_owner, start_after,
		// request_payer) and outputs (request_charged) are therefore not modeled.
		Schema: map[string]*schema.Schema{
			names.AttrBucket: {
				Type:     schema.TypeString,
				Required: true,
				ValidateFunc: validation.All(
					validation.StringLenBetween(3, 63),
					validation.StringMatch(regexp.MustCompile(`^[a-z0-9.-]+$`), "must contain only lowercase letters, numbers, periods, and hyphens"),
				),
			},
			"common_prefix": {
				Type:     schema.TypeList,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"delimiter": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringInSlice([]string{"/"}, false),
			},
			"encoding_type": {
				Type:             schema.TypeString,
				Optional:         true,
				ValidateDiagFunc: enum.ValidateIgnoreCase[types.EncodingType](),
			},
			"keys": {
				Type:     schema.TypeList,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"max_keys": {
				Type:         schema.TypeInt,
				Optional:     true,
				Default:      1000,
				ValidateFunc: validation.IntAtLeast(1),
			},
			names.AttrPrefix: {
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	}
}

func dataSourceObjectsNextMarker(bucket string, page *s3.ListObjectsOutput) (string, error) {
	if !aws.ToBool(page.IsTruncated) {
		return "", nil
	}

	nextMarker := aws.ToString(page.NextMarker)
	if nextMarker == "" {
		return "", fmt.Errorf("listing S3 Bucket (%s) Objects: truncated response missing NextMarker", bucket)
	}

	return nextMarker, nil
}
func dataSourceObjectsRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).S3Client(ctx)

	bucket := d.Get(names.AttrBucket).(string)

	input := &s3.ListObjectsInput{
		Bucket: aws.String(bucket),
	}

	if v, ok := d.GetOk("delimiter"); ok {
		input.Delimiter = aws.String(v.(string))
	}

	if v, ok := d.GetOk("encoding_type"); ok {
		input.EncodingType = types.EncodingType(v.(string))
	}

	if v, ok := d.GetOk(names.AttrPrefix); ok {
		input.Prefix = aws.String(v.(string))
	}

	// "input.MaxKeys" refers to max keys returned in a single request (i.e. page
	// size, capped at 1000 by OOS), not the total number of keys returned when
	// paging through the results. "max_keys" refers to total keys returned.
	maxKeys := int64(d.Get("max_keys").(int))
	if maxKeys < keyRequestPageSize {
		input.MaxKeys = aws.Int32(int32(maxKeys))
	}

	var nKeys int64
	var commonPrefixes, keys []string
	commonPrefixesSeen := make(map[string]struct{})
	var nextMarker string

	// OOS does not provide a V2 paginator, so page manually with marker /
	// NextMarker until IsTruncated is false or "max_keys" total keys are reached.
pageLoop:
	for {
		page, err := conn.ListObjects(ctx, input)

		if err != nil {
			return sdkdiag.AppendErrorf(diags, "listing S3 Bucket (%s) Objects: %s", bucket, err)
		}

		for _, v := range page.CommonPrefixes {
			prefix := aws.ToString(v.Prefix)
			if strings.EqualFold(string(input.EncodingType), string(types.EncodingTypeUrl)) {
				prefix, err = url.QueryUnescape(prefix)
				if err != nil {
					return sdkdiag.AppendErrorf(diags, "listing S3 Bucket (%s) Objects: unescaping common prefix: %s", bucket, err)
				}
			}
			if _, ok := commonPrefixesSeen[prefix]; ok {
				continue
			}
			commonPrefixesSeen[prefix] = struct{}{}
			commonPrefixes = append(commonPrefixes, prefix)
		}

		for _, v := range page.Contents {
			if nKeys >= maxKeys {
				break pageLoop
			}

			key := aws.ToString(v.Key)
			if strings.EqualFold(string(input.EncodingType), string(types.EncodingTypeUrl)) {
				key, err = url.QueryUnescape(key)
				if err != nil {
					return sdkdiag.AppendErrorf(diags, "listing S3 Bucket (%s) Objects: unescaping object key: %s", bucket, err)
				}
			}

			keys = append(keys, key)

			nKeys++
		}

		if nKeys >= maxKeys {
			break
		}

		if !aws.ToBool(page.IsTruncated) {
			break
		}

		// OOS returns NextMarker whenever more results are available. That marker
		// identifies the first object of the next page, and the follow-up request
		// includes that object in its response.
		nextMarker, err = dataSourceObjectsNextMarker(bucket, page)
		if err != nil {
			return sdkdiag.AppendErrorf(diags, "%s", err)
		}

		input.Marker = aws.String(nextMarker)
	}

	d.SetId(bucket)
	d.Set("common_prefix", commonPrefixes)
	d.Set("keys", keys)

	return diags
}
