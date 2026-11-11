// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package s3_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	tfs3 "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/s3"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TC-S3-SSE-01 Basic AES256
// TestAccS3BucketServerSideEncryptionConfiguration_basic verifies the AES256 encryption configuration.
func TestAccS3BucketServerSideEncryptionConfiguration_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_server_side_encryption_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyNoop,
		Steps: []resource.TestStep{
			{
				Config: testAccBucketServerSideEncryptionConfigurationConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketServerSideEncryptionConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, names.AttrID, rName),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, names.AttrBucket, rName),
					resource.TestCheckResourceAttr(resourceName, "rule.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.apply_server_side_encryption_by_default.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.apply_server_side_encryption_by_default.0.sse_algorithm", "AES256"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				Check:             resource.TestCheckResourceAttr(resourceName, names.AttrID, rName),
			},
		},
	})
}

// TC-S3-SSE-02 Full Update Chain
// TestAccS3BucketServerSideEncryptionConfiguration_FullUpdateChain verifies the full update chain of the AES256 configuration.
func TestAccS3BucketServerSideEncryptionConfiguration_FullUpdateChain(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_server_side_encryption_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyNoop,
		Steps: []resource.TestStep{
			{
				Config: testAccBucketServerSideEncryptionConfigurationConfig_bucketOnly(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ctyun_oos_bucket.test", names.AttrBucket, rName),
				),
			},
			{
				Config: testAccBucketServerSideEncryptionConfigurationConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketServerSideEncryptionConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, names.AttrID, rName),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, names.AttrBucket, rName),
					resource.TestCheckResourceAttr(resourceName, "rule.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.apply_server_side_encryption_by_default.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.apply_server_side_encryption_by_default.0.sse_algorithm", "AES256"),
				),
			},
			{
				Config: testAccBucketServerSideEncryptionConfigurationConfig_bucketOnly(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketServerSideEncryptionConfigurationRetained(ctx, t, rName),
					testAccCheckBucketServerSideEncryptionConfigurationNotInState(resourceName),
				),
			},
			{
				Config: testAccBucketServerSideEncryptionConfigurationConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketServerSideEncryptionConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, names.AttrID, rName),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, names.AttrBucket, rName),
					resource.TestCheckResourceAttr(resourceName, "rule.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.apply_server_side_encryption_by_default.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.apply_server_side_encryption_by_default.0.sse_algorithm", "AES256"),
				),
			},
			{
				Config:             testAccBucketServerSideEncryptionConfigurationConfig_basic(rName),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// TC-S3-SSE-05 Disappears
// TestAccS3BucketServerSideEncryptionConfiguration_Disappears_bucket verifies the re-creation plan after remote deletion.
func TestAccS3BucketServerSideEncryptionConfiguration_Disappears_bucket(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_server_side_encryption_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyNoop,
		Steps: []resource.TestStep{
			{
				Config: testAccBucketServerSideEncryptionConfigurationConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketServerSideEncryptionConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, names.AttrID, rName),
					acctest.CheckSDKResourceDisappears(ctx, t, tfs3.ResourceBucket(), "ctyun_oos_bucket.test"),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TC-S3-SSE-03 Remove Config Retains Remote
// TestAccS3BucketServerSideEncryptionConfiguration_RemoveConfigRetainsRemote verifies that the remote configuration is retained after the resource is removed.
func TestAccS3BucketServerSideEncryptionConfiguration_RemoveConfigRetainsRemote(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_server_side_encryption_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyNoop,
		Steps: []resource.TestStep{
			{
				Config: testAccBucketServerSideEncryptionConfigurationConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketServerSideEncryptionConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, names.AttrID, rName),
				),
			},
			{
				Config: testAccBucketServerSideEncryptionConfigurationConfig_bucketOnly(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketServerSideEncryptionConfigurationRetained(ctx, t, rName),
					testAccCheckBucketServerSideEncryptionConfigurationNotInState(resourceName),
				),
			},
			{
				Config: testAccBucketServerSideEncryptionConfigurationConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketServerSideEncryptionConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, names.AttrID, rName),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, names.AttrBucket, rName),
					resource.TestCheckResourceAttr(resourceName, "rule.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.apply_server_side_encryption_by_default.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.apply_server_side_encryption_by_default.0.sse_algorithm", "AES256"),
				),
			},
			{
				Config:             testAccBucketServerSideEncryptionConfigurationConfig_basic(rName),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// TC-S3-SSE-04 Order Stability
// TestAccS3BucketServerSideEncryptionConfiguration_OrderStability verifies that the single-element nested configuration keeps stable state and plan after repeated refresh/apply.
func TestAccS3BucketServerSideEncryptionConfiguration_OrderStability(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_server_side_encryption_configuration.test"

	check := resource.ComposeAggregateTestCheckFunc(
		testAccCheckBucketServerSideEncryptionConfigurationExists(ctx, t, resourceName),
		resource.TestCheckResourceAttr(resourceName, names.AttrID, rName),
		resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
		resource.TestCheckResourceAttr(resourceName, names.AttrBucket, rName),
		resource.TestCheckResourceAttr(resourceName, "rule.#", "1"),
		resource.TestCheckResourceAttr(resourceName, "rule.0.apply_server_side_encryption_by_default.#", "1"),
		resource.TestCheckResourceAttr(resourceName, "rule.0.apply_server_side_encryption_by_default.0.sse_algorithm", "AES256"),
	)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyNoop,
		Steps: []resource.TestStep{
			{Config: testAccBucketServerSideEncryptionConfigurationConfig_basic(rName), Check: check},
			{RefreshState: true, Check: check},
			{Config: testAccBucketServerSideEncryptionConfigurationConfig_basic(rName), Check: check},
			{RefreshState: true, Check: check},
			{
				Config:             testAccBucketServerSideEncryptionConfigurationConfig_basic(rName),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// TC-S3-SSE-06 Import And Identity
// TestAccS3BucketServerSideEncryptionConfiguration_ImportAndIdentity_bucket verifies consistent state after importing by bucket.
func TestAccS3BucketServerSideEncryptionConfiguration_ImportAndIdentity_bucket(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_server_side_encryption_configuration.test"

	check := resource.ComposeAggregateTestCheckFunc(
		testAccCheckBucketServerSideEncryptionConfigurationExists(ctx, t, resourceName),
		resource.TestCheckResourceAttr(resourceName, names.AttrID, rName),
		resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
		resource.TestCheckResourceAttr(resourceName, names.AttrBucket, rName),
		resource.TestCheckResourceAttr(resourceName, "rule.#", "1"),
		resource.TestCheckResourceAttr(resourceName, "rule.0.apply_server_side_encryption_by_default.#", "1"),
		resource.TestCheckResourceAttr(resourceName, "rule.0.apply_server_side_encryption_by_default.0.sse_algorithm", "AES256"),
	)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyNoop,
		Steps: []resource.TestStep{
			{Config: testAccBucketServerSideEncryptionConfigurationConfig_basic(rName), Check: check},
			{
				ResourceName: resourceName,
				ImportState:  true,
				ImportStateIdFunc: func(_ *terraform.State) (string, error) {
					return rName, nil
				},
				ImportStateVerify: true,
				Check:             resource.TestCheckResourceAttr(resourceName, names.AttrID, rName),
			},
			{
				Config:             testAccBucketServerSideEncryptionConfigurationConfig_basic(rName),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}
func testAccCheckBucketServerSideEncryptionConfigurationExists(ctx context.Context, t *testing.T, n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)

		_, err := tfs3.FindServerSideEncryptionConfiguration(ctx, conn, rs.Primary.Attributes[names.AttrBucket])

		return err
	}
}

func testAccCheckBucketServerSideEncryptionConfigurationNotInState(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if _, ok := s.RootModule().Resources[n]; ok {
			return fmt.Errorf("SSE configuration unexpectedly remains in state: %s", n)
		}

		return nil
	}
}
func testAccCheckBucketServerSideEncryptionConfigurationRetained(ctx context.Context, t *testing.T, bucket string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)

		_, err := tfs3.FindServerSideEncryptionConfiguration(ctx, conn, bucket)
		return err
	}
}

func testAccBucketServerSideEncryptionConfigurationConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q

}

resource "ctyun_oos_bucket_server_side_encryption_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}
`, rName)
}

func testAccBucketServerSideEncryptionConfigurationConfig_bucketOnly(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q

}
`, rName)
}
