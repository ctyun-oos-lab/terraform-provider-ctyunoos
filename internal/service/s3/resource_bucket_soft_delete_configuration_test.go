// Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package s3_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/YakDriver/regexache"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	tfs3 "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/s3"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/aws-sdk-go-base/v2/tfawserr"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TC-S3-SOFTDELETE-01 Basic CRUD - creates a basic soft-delete days configuration and reads it back correctly.
// TestAccS3BucketSoftDelete_BasicCRUD_basic verifies that the soft-delete configuration is written successfully, the state is correct, and it can be imported.
func TestAccS3BucketSoftDelete_BasicCRUD_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_soft_delete_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketSoftDeleteDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketSoftDeleteConfig_basic(rName, 1),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketSoftDeleteExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "days", "1"),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TC-S3-SOFTDELETE-02 Update Retention Days - updates the retention days and verifies the remote side and state are consistent.
func TestAccS3BucketSoftDelete_UpdateRetentionDays_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_soft_delete_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketSoftDeleteDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketSoftDeleteConfig_basic(rName, 2),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketSoftDeleteExists(ctx, t, resourceName),
					testAccCheckBucketSoftDeleteDays(ctx, t, resourceName, 2),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "days", "2"),
				),
			},
			{
				Config: testAccBucketSoftDeleteConfig_basic(rName, 14),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketSoftDeleteExists(ctx, t, resourceName),
					testAccCheckBucketSoftDeleteDays(ctx, t, resourceName, 14),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "days", "14"),
				),
			},
			{
				Config: testAccBucketSoftDeleteConfig_basic(rName, 6),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketSoftDeleteExists(ctx, t, resourceName),
					testAccCheckBucketSoftDeleteDays(ctx, t, resourceName, 6),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "days", "6"),
				),
			},
		},
	})
}

// TC-S3-SOFTDELETE-03 Retention Bounds - covers the minimum, middle, and maximum retention days.
// TestAccS3BucketSoftDelete_RetentionBounds_validDays verifies that after writing valid boundary values, the remote side and state are consistent.
func TestAccS3BucketSoftDelete_RetentionBounds_validDays(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_soft_delete_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketSoftDeleteDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketSoftDeleteConfig_basic(rName, 1),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketSoftDeleteExists(ctx, t, resourceName),
					testAccCheckBucketSoftDeleteDays(ctx, t, resourceName, 1),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "days", "1"),
				),
			},
			{
				Config: testAccBucketSoftDeleteConfig_basic(rName, 7),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketSoftDeleteExists(ctx, t, resourceName),
					testAccCheckBucketSoftDeleteDays(ctx, t, resourceName, 7),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "days", "7"),
				),
			},
			{
				Config: testAccBucketSoftDeleteConfig_basic(rName, 15),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketSoftDeleteExists(ctx, t, resourceName),
					testAccCheckBucketSoftDeleteDays(ctx, t, resourceName, 15),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "days", "15"),
				),
			},
		},
	})
}

// TC-S3-SOFTDELETE-03 Retention Bounds - verifies that illegal retention days are rejected by the schema.
// TestAccS3BucketSoftDelete_RetentionBounds_invalidDays verifies that negative and over-limit values are rejected at the local schema validation stage.
func TestAccS3BucketSoftDelete_RetentionBounds_invalidDays(t *testing.T) {
	testCases := []struct {
		name string
		days int
	}{
		{name: "negative", days: -1},
		{name: "above-maximum", days: 16},
		{name: "above-minimum", days: 0},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := acctest.Context(t)
			rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

			acctest.ParallelTest(ctx, t, resource.TestCase{
				PreCheck:                 func() { acctest.PreCheck(ctx, t) },
				ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
				ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config:      testAccBucketSoftDeleteConfig_basic(rName, testCase.days),
						ExpectError: regexache.MustCompile(`(?i)expected days to be in the range \(1 - 15\)`),
					},
				},
			})
		})
	}
}

// TC-S3-SOFTDELETE-04 Remove Config Restores Default - restores the default 7 days after the resource declaration is removed, and verifies the state recovers stably.
// TestAccS3BucketSoftDelete_RemoveConfigRestoresDefault_basic verifies that the delete operation writes back the default value, and that after re-declaring, the remote side and state are consistent.
func TestAccS3BucketSoftDelete_RemoveConfigRestoresDefault_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_soft_delete_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketSoftDeleteDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketSoftDeleteConfig_basic(rName, 13),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketSoftDeleteExists(ctx, t, resourceName),
					testAccCheckBucketSoftDeleteDays(ctx, t, resourceName, 13),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "days", "13"),
				),
			},
			{
				Config: testAccBucketSoftDeleteConfig_bucketOnly(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketSoftDeleteNotInState(resourceName),
					testAccCheckBucketSoftDeleteDays(ctx, t, "ctyun_oos_bucket.test", 7),
				),
			},
			{
				Config: testAccBucketSoftDeleteConfig_basic(rName, 13),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketSoftDeleteExists(ctx, t, resourceName),
					testAccCheckBucketSoftDeleteDays(ctx, t, resourceName, 13),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "days", "13"),
				),
			},
		},
	})
}

// TC-S3-SOFTDELETE-05 Default Fillback - creates the configuration using the server-side default retention days, and verifies there is no repeated change after the back-fill.
// TestAccS3BucketSoftDelete_DefaultFillback_basic verifies that the plan is empty again after the default days is back-filled into state.
func TestAccS3BucketSoftDelete_DefaultFillback_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_soft_delete_configuration.test"
	dataSourceName := "data.ctyun_oos_bucket_soft_delete_configuration.default"
	config := testAccBucketSoftDeleteConfig_defaultFillback(rName)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketSoftDeleteDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketSoftDeleteExists(ctx, t, resourceName),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttrPair(resourceName, "days", dataSourceName, "days"),
				),
			},
			{
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// TC-S3-SOFTDELETE-06 Import - uses the bucket as the import identifier and verifies the days read-back is stable.
// TestAccS3BucketSoftDelete_Import_bucket verifies that the imported soft-delete configuration matches the remote state.
func TestAccS3BucketSoftDelete_Import_bucket(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_soft_delete_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketSoftDeleteDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketSoftDeleteConfig_basic(rName, 7),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketSoftDeleteExists(ctx, t, resourceName),
					testAccCheckBucketSoftDeleteDays(ctx, t, resourceName, 7),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "days", "7"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateIdFunc: acctest.AttrImportStateIdFunc(resourceName, names.AttrBucket),
				ImportStateVerify: true,
			},
			{
				RefreshState: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketSoftDeleteExists(ctx, t, resourceName),
					testAccCheckBucketSoftDeleteDays(ctx, t, resourceName, 7),
					resource.TestCheckResourceAttr(resourceName, "days", "7"),
				),
			},
		},
	})
}

// TC-S3-SOFTDELETE-07 Disappears - after the remote Bucket is deleted, verifies the soft-delete resource disappears from state and a re-creation is planned.
func TestAccS3BucketSoftDelete_Disappears_bucket(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_soft_delete_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketSoftDeleteDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketSoftDeleteConfig_basic(rName, 7),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketSoftDeleteExists(ctx, t, resourceName),
					acctest.CheckSDKResourceDisappears(ctx, t, tfs3.ResourceBucket(), "ctyun_oos_bucket.test"),
				),
				ExpectNonEmptyPlan: true,
			},
			{
				RefreshState:       true,
				Check:              testAccCheckBucketSoftDeleteNotInState(resourceName),
				ExpectNonEmptyPlan: true,
			},
			{
				Config:             testAccBucketSoftDeleteConfig_basic(rName, 7),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func testAccCheckBucketSoftDeleteNotInState(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if _, ok := s.RootModule().Resources[n]; ok {
			return fmt.Errorf("expected %s to be absent from state", n)
		}

		return nil
	}
}

func testAccCheckBucketSoftDeleteExists(ctx context.Context, t *testing.T, n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)

		bucket := rs.Primary.Attributes[names.AttrBucket]
		cfg, err := tfs3.FindBucketSoftDelete(ctx, conn, bucket)
		if err != nil {
			return err
		}
		if cfg == nil {
			return fmt.Errorf("OOS Bucket (%s) Soft Delete returned an empty configuration", bucket)
		}

		return nil
	}
}

func testAccCheckBucketSoftDeleteDays(ctx context.Context, t *testing.T, n string, want int32) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)
		cfg, err := tfs3.FindBucketSoftDelete(ctx, conn, rs.Primary.Attributes[names.AttrBucket])
		if err != nil {
			return err
		}

		if cfg.Days != want {
			return fmt.Errorf("expected soft delete days %d, got %d", want, cfg.Days)
		}

		return nil
	}
}

func testAccCheckBucketSoftDeleteDestroy(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "ctyun_oos_bucket_soft_delete_configuration" {
				continue
			}

			// Deleting the soft delete configuration restores the default retention before
			// Terraform destroys the bucket, so a successful final destroy requires the
			// bucket itself to be gone.
			_, err := tfs3.FindBucketSoftDelete(ctx, conn, rs.Primary.Attributes[names.AttrBucket])

			if tfawserr.ErrCodeEquals(err, tfs3.ErrCodeNoSuchBucket) {
				continue
			}

			if err != nil {
				return err
			}
		}

		return nil
	}
}

func testAccBucketSoftDeleteConfig_basic(rName string, days int) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q

}

resource "ctyun_oos_bucket_soft_delete_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket
  days   = %[2]d
}
`, rName, days)
}

func testAccBucketSoftDeleteConfig_defaultFillback(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

data "ctyun_oos_bucket_soft_delete_configuration" "default" {
  bucket = ctyun_oos_bucket.test.bucket
}

resource "ctyun_oos_bucket_soft_delete_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket
  days   = data.ctyun_oos_bucket_soft_delete_configuration.default.days
}
`, rName)
}
func testAccBucketSoftDeleteConfig_bucketOnly(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q

}
`, rName)
}
