// Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package s3

import (
	"strings"
	"testing"

	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestUnitS3BucketSoftDeleteDataSource_emptyConfiguration(t *testing.T) {
	bucket := "test-bucket"
	d := schema.TestResourceDataRaw(t, dataSourceBucketSoftDelete().Schema, map[string]any{
		names.AttrBucket: bucket,
	})

	diags := dataSourceBucketSoftDeleteSetState(d, bucket, nil)

	if !diags.HasError() {
		t.Fatal("expected empty configuration to return an error diagnostic")
	}
	if got := diags[0].Summary; !strings.Contains(got, "empty configuration response") {
		t.Errorf("diagnostic summary = %q, want empty configuration response", got)
	}
	if got := d.Id(); got != "" {
		t.Errorf("resource ID = %q, want empty", got)
	}
}
