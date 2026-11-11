// Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package s3

import (
	"context"
	"crypto/sha256"
	"fmt"
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

// @SDKResource("ctyun_oos_object_recover", name="Object Recover")
// @NoImport
func resourceObjectRecover() *schema.Resource {
	recoverSelectorAttrs := []string{
		"recover",
		"soft_delete_after",
		"soft_delete_before",
	}

	return &schema.Resource{
		CreateWithoutTimeout: resourceObjectRecoverCreate,
		ReadWithoutTimeout:   resourceObjectRecoverRead,
		DeleteWithoutTimeout: schema.NoopContext,

		CustomizeDiff: resourceObjectRecoverCustomizeDiff,

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
			"errors": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						names.AttrKey: {
							Type:     schema.TypeString,
							Computed: true,
						},
						"code": {
							Type:     schema.TypeString,
							Computed: true,
						},
						names.AttrMessage: {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
			"encoding_type": {
				Type:             schema.TypeString,
				Optional:         true,
				ForceNew:         true,
				ValidateDiagFunc: enum.ValidateIgnoreCase[types.EncodingType](),
			},
			"recover": {
				Type:          schema.TypeList,
				Optional:      true,
				ForceNew:      true,
				MinItems:      1,
				MaxItems:      1000,
				ConflictsWith: []string{"soft_delete_after", "soft_delete_before"},
				AtLeastOneOf:  recoverSelectorAttrs,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						names.AttrKey: {
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: validation.NoZeroValues,
						},
					},
				},
			},
			"recovered_keys": {
				Type:     schema.TypeList,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"soft_delete_after": {
				Type:          schema.TypeInt,
				Optional:      true,
				ForceNew:      true,
				ConflictsWith: []string{"recover"},
				AtLeastOneOf:  recoverSelectorAttrs,
				ValidateFunc:  validation.IntAtLeast(1),
			},
			"soft_delete_before": {
				Type:          schema.TypeInt,
				Optional:      true,
				ForceNew:      true,
				ConflictsWith: []string{"recover"},
				AtLeastOneOf:  recoverSelectorAttrs,
				ValidateFunc:  validation.IntAtLeast(1),
			},
		},
	}
}

func resourceObjectRecoverCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).S3Client(ctx)

	input := expandObjectRecoverInput(d)

	result, err := oosRecoverObjects(ctx, conn, input)
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "recovering OOS Objects in Bucket (%s): %s", input.bucket, err)
	}
	if err := decodeRecoverObjectsResult(result, input.encodingType); err != nil {
		return sdkdiag.AppendErrorf(diags, "decoding OOS Object Recover result for Bucket (%s): %s", input.bucket, err)
	}

	d.SetId(objectRecoverID(input))

	if err := d.Set("recovered_keys", flattenRecoveredObjectKeys(result)); err != nil {
		return sdkdiag.AppendErrorf(diags, "setting recovered_keys: %s", err)
	}
	if err := d.Set("errors", flattenRecoverErrors(result.RecoverError)); err != nil {
		return sdkdiag.AppendErrorf(diags, "setting errors: %s", err)
	}

	if len(result.RecoverError) > 0 {
		return sdkdiag.AppendErrorf(diags, "recovering OOS Objects in Bucket (%s): %s", input.bucket, formatRecoverErrors(result.RecoverError))
	}

	return diags
}

func resourceObjectRecoverRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	return nil
}

func resourceObjectRecoverCustomizeDiff(_ context.Context, d *schema.ResourceDiff, meta any) error {
	afterRaw, hasAfter := d.GetOk("soft_delete_after")
	beforeRaw, hasBefore := d.GetOk("soft_delete_before")

	var after, before int
	if hasAfter {
		after = afterRaw.(int)
	}
	if hasBefore {
		before = beforeRaw.(int)
	}

	return validateObjectRecoverTimeWindow(after, before, hasAfter, hasBefore)
}

func validateObjectRecoverTimeWindow(after, before int, hasAfter, hasBefore bool) error {
	if hasAfter && hasBefore && before < after {
		return fmt.Errorf("soft_delete_before must be greater than or equal to soft_delete_after")
	}

	return nil
}

func expandObjectRecoverInput(d *schema.ResourceData) oosRecoverObjectsInput {
	input := oosRecoverObjectsInput{
		bucket: d.Get(names.AttrBucket).(string),
	}

	input.keys = expandObjectRecoverKeys(d.Get("recover").([]any))

	if v, ok := d.GetOk("encoding_type"); ok {
		input.encodingType = v.(string)
	}

	if v, ok := d.GetOk("soft_delete_after"); ok {
		after := v.(int)
		input.softDeleteAfter = strconv.Itoa(after)
	}
	if v, ok := d.GetOk("soft_delete_before"); ok {
		before := v.(int)
		input.softDeleteBefore = strconv.Itoa(before)
	}
	return input
}

func expandObjectRecoverKeys(items []any) []string {
	var keys []string
	for _, item := range items {
		if itemMap, ok := item.(map[string]any); ok {
			if key, ok := itemMap[names.AttrKey].(string); ok {
				keys = append(keys, key)
			}
		}
	}

	return keys
}
func objectRecoverID(input oosRecoverObjectsInput) string {
	hash := sha256.New()
	hash.Write([]byte(input.bucket))
	hash.Write([]byte{0})
	hash.Write([]byte(input.softDeleteAfter))
	hash.Write([]byte{0})
	hash.Write([]byte(input.softDeleteBefore))

	hash.Write([]byte{0})
	hash.Write([]byte(input.encodingType))
	for _, key := range input.keys {
		hash.Write([]byte{0})
		hash.Write([]byte(key))
	}

	return fmt.Sprintf("%s/recover/%x", input.bucket, hash.Sum(nil))
}

func decodeRecoverObjectsResult(result *oosRecoverObjectsResult, encodingType string) error {
	if result == nil || encodingType == "" {
		return nil
	}

	for i := range result.Recovered {
		key, err := url.QueryUnescape(result.Recovered[i].Key)
		if err != nil {
			return fmt.Errorf("unescaping recovered key %q: %w", result.Recovered[i].Key, err)
		}
		result.Recovered[i].Key = key
	}
	for i := range result.RecoverError {
		key, err := url.QueryUnescape(result.RecoverError[i].Key)
		if err != nil {
			return fmt.Errorf("unescaping recover error key %q: %w", result.RecoverError[i].Key, err)
		}
		result.RecoverError[i].Key = key
	}

	return nil
}
func flattenRecoveredObjectKeys(result *oosRecoverObjectsResult) []string {
	if result == nil {
		return nil
	}

	keys := make([]string, 0, len(result.Recovered))
	for _, recovered := range result.Recovered {
		keys = append(keys, recovered.Key)
	}

	return keys
}

func flattenRecoverErrors(errors []oosRecoverError) []map[string]any {
	result := make([]map[string]any, 0, len(errors))
	for _, err := range errors {
		result = append(result, map[string]any{
			names.AttrKey:     err.Key,
			"code":            err.Code,
			names.AttrMessage: err.Message,
		})
	}

	return result
}

func formatRecoverErrors(errors []oosRecoverError) string {
	parts := make([]string, 0, len(errors))
	for _, err := range errors {
		parts = append(parts, fmt.Sprintf("%s: %s (%s)", err.Key, err.Message, err.Code))
	}

	return strings.Join(parts, "; ")
}
