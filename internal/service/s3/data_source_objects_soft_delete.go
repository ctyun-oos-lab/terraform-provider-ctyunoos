// Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package s3

import (
	"context"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/enum"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/sdkdiag"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

const softDeleteObjectsPageSize = 1000

// @SDKDataSource("ctyun_oos_objects_soft_delete", name="Objects Soft Delete")
func dataSourceObjectsSoftDelete() *schema.Resource {
	return &schema.Resource{
		ReadWithoutTimeout: dataSourceObjectsSoftDeleteRead,

		Schema: map[string]*schema.Schema{
			names.AttrBucket: {
				Type:     schema.TypeString,
				Required: true,
				ValidateFunc: validation.All(
					validation.StringLenBetween(3, 63),
					validation.StringMatch(regexp.MustCompile(`^[a-z0-9.-]+$`), "must contain only lowercase letters, numbers, periods, and hyphens"),
				),
			},
			"contents": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						names.AttrKey: {
							Type:     schema.TypeString,
							Computed: true,
						},
						"soft_delete_at": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
			"encoding_type": {
				Type:             schema.TypeString,
				Optional:         true,
				Computed:         true,
				ValidateDiagFunc: enum.ValidateIgnoreCase[types.EncodingType](),
			},
			"max_keys": {
				Type:         schema.TypeInt,
				Optional:     true,
				Computed:     true,
				ValidateFunc: validation.IntAtLeast(1),
			},
			"soft_delete_after": {
				Type:         schema.TypeInt,
				Optional:     true,
				ValidateFunc: validation.IntAtLeast(1),
			},
			"soft_delete_before": {
				Type:         schema.TypeInt,
				Optional:     true,
				ValidateFunc: validation.IntAtLeast(1),
			},
		},
	}
}

func dataSourceObjectsSoftDeleteRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).S3Client(ctx)

	input := oosListObjectsSoftDeleteInput{
		bucket: d.Get(names.AttrBucket).(string),
	}

	if v, ok := d.GetOk("soft_delete_after"); ok {
		after := v.(int)
		input.softDeleteAfter = strconv.Itoa(after)
	}
	if v, ok := d.GetOk("soft_delete_before"); ok {
		before := v.(int)
		input.softDeleteBefore = strconv.Itoa(before)
	}
	if after, afterOk := d.GetOk("soft_delete_after"); afterOk {
		if before, beforeOk := d.GetOk("soft_delete_before"); beforeOk && before.(int) < after.(int) {
			return sdkdiag.AppendErrorf(diags, "soft_delete_before must be greater than or equal to soft_delete_after")
		}
	}
	if v, ok := d.GetOk("max_keys"); ok {
		input.maxKeys = v.(int)
	} else {
		input.maxKeys = 1000
	}
	if v, ok := d.GetOk("encoding_type"); ok {
		input.encodingType = v.(string)
	}

	maxKeys := input.maxKeys
	pageInput := input
	pageInput.maxKeys = 0
	if maxKeys < softDeleteObjectsPageSize {
		pageInput.maxKeys = maxKeys
	}

	var (
		contents             []oosListObjectsSoftDeleteObj
		responseEncodingType string
	)

pageLoop:
	for {
		output, err := oosListObjectsSoftDelete(ctx, conn, pageInput)
		if err != nil {
			return sdkdiag.AppendErrorf(diags, "listing OOS Bucket (%s) Soft Deleted Objects: %s", input.bucket, err)
		}
		if responseEncodingType == "" {
			responseEncodingType = output.EncodingType
		}

		for _, object := range output.Contents {
			key, err := decodeSoftDeletedObjectKey(object.Key, input.encodingType)
			if err != nil {
				return sdkdiag.AppendErrorf(diags, "listing OOS Bucket (%s) Soft Deleted Objects: unescaping object key: %s", input.bucket, err)
			}
			object.Key = key

			if len(contents) >= maxKeys {
				break pageLoop
			}

			contents = append(contents, object)
		}

		if len(contents) >= maxKeys {
			break
		}

		// Stop when the server signals no more pages: either IsTruncated is
		// false, or it is truncated but carries no NextMarker.
		if !output.IsTruncated || output.NextMarker == "" {
			break
		}

		pageInput.marker = output.NextMarker
	}

	encodingType := input.encodingType
	if encodingType == "" {
		encodingType = responseEncodingType
	}

	d.SetId(input.bucket)
	if err := d.Set("encoding_type", encodingType); err != nil {
		return sdkdiag.AppendErrorf(diags, "setting encoding_type: %s", err)
	}
	if err := d.Set("max_keys", maxKeys); err != nil {
		return sdkdiag.AppendErrorf(diags, "setting max_keys: %s", err)
	}
	if err := d.Set("contents", flattenSoftDeletedObjectContents(contents)); err != nil {
		return sdkdiag.AppendErrorf(diags, "setting contents: %s", err)
	}

	return diags
}

func decodeSoftDeletedObjectKey(key, encodingType string) (string, error) {
	if !strings.EqualFold(encodingType, string(types.EncodingTypeUrl)) {
		return key, nil
	}

	return url.QueryUnescape(key)
}

func flattenSoftDeletedObjectContents(objects []oosListObjectsSoftDeleteObj) []map[string]any {
	result := make([]map[string]any, 0, len(objects))
	for _, object := range objects {
		result = append(result, map[string]any{
			names.AttrKey:    object.Key,
			"soft_delete_at": object.softDeleteAt(),
		})
	}

	return result
}
