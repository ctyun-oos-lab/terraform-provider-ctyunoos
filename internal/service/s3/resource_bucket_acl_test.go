// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package s3_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	tfs3 "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/s3"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestParseBucketACLResourceID(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		TestName       string
		InputID        string
		ExpectError    bool
		ExpectedACL    string
		ExpectedBucket string
	}{
		{
			TestName:    "empty ID",
			InputID:     "",
			ExpectError: true,
		},
		{
			TestName:    "incorrect bucket and ACL format with slash separator",
			InputID:     "test/private",
			ExpectError: true,
		},
		{
			TestName:    "incorrect bucket, ACL, and extra segment",
			InputID:     "test,private,extra",
			ExpectError: true,
		},
		{
			TestName:       "valid ID with bucket",
			InputID:        "example",
			ExpectedACL:    "",
			ExpectedBucket: "example",
		},
		{
			TestName:       "valid ID with bucket that has hyphens",
			InputID:        "my-example-bucket",
			ExpectedACL:    "",
			ExpectedBucket: "my-example-bucket",
		},
		{
			TestName:       "valid ID with bucket that has dots, hyphen, and numbers",
			InputID:        "my-example.bucket.4000",
			ExpectedACL:    "",
			ExpectedBucket: "my-example.bucket.4000",
		},
		{
			TestName:    "bucket and acl legacy format no longer accepted",
			InputID:     "example," + string(types.BucketCannedACLPrivate),
			ExpectError: true,
		},
		{
			TestName:    "bucket and acl legacy format with hyphens no longer accepted",
			InputID:     "example," + string(types.BucketCannedACLPublicReadWrite),
			ExpectError: true,
		},
		{
			TestName:    "bucket and acl legacy format with dotted bucket no longer accepted",
			InputID:     "my-example.bucket.4000," + string(types.BucketCannedACLPublicReadWrite),
			ExpectError: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.TestName, func(t *testing.T) {
			t.Parallel()

			gotBucket, gotACL, err := tfs3.ParseBucketACLResourceID(testCase.InputID)

			if err == nil && testCase.ExpectError {
				t.Fatalf("expected error, got no error")
			}

			if err != nil && !testCase.ExpectError {
				t.Fatalf("got unexpected error: %s", err)
			}

			if gotACL != testCase.ExpectedACL {
				t.Errorf("got ACL %s, expected %s", gotACL, testCase.ExpectedACL)
			}

			if gotBucket != testCase.ExpectedBucket {
				t.Errorf("got bucket %s, expected %s", gotBucket, testCase.ExpectedBucket)
			}
		})
	}
}

// TC-S3-BUCKET-ACL-08 Basic CRUD - creates a canned ACL and verifies the full state expansion.
// TestAccS3BucketACL_basic verifies the creation and read of an explicit private ACL and the access_control_policy expansion.
// Run with:
// source .env.oos.local
// make t PKG=s3 T=TestAccS3BucketACL_basic P=1
//
// Example .env.oos.local:
// export CTYUNOOS_ENDPOINT=<your-endpoint>
// export CTYUNOOS_ACCESS_KEY=<your-ak>
// export CTYUNOOS_SECRET_KEY=<your-sk>
func TestAccS3BucketACL_basic(t *testing.T) {
	ctx := acctest.Context(t)
	bucketName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_acl.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyNoop,
		Steps: []resource.TestStep{
			{
				Config: testAccBucketACLConfig_cannedACL(bucketName, string(types.BucketCannedACLPrivate)),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Verify the current ACL can be read back remotely via GetBucketAcl.
					testAccCheckBucketACLExists(ctx, t, resourceName),
					// The provider should normalize the canned ACL back to "private".
					resource.TestCheckResourceAttr(resourceName, "acl", string(types.BucketCannedACLPrivate)),
					// The ACL resource should always be bound to the bucket created by this test.
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					// The GetBucketAcl response should be flattened into a single access_control_policy block.
					resource.TestCheckResourceAttr(resourceName, "access_control_policy.#", "1"),
					// The owner structure should exist, even though OOS may return empty owner field values.
					resource.TestCheckResourceAttr(resourceName, "access_control_policy.0.owner.#", "1"),
					// A private ACL should still produce one grant record after flattening.
					resource.TestCheckResourceAttr(resourceName, "access_control_policy.0.grant.#", "1"),
					// The OOS private ACL should expand into a grant pointing to AllUsers with an empty permission.
					resource.TestCheckTypeSetElemNestedAttrs(resourceName, "access_control_policy.0.grant.*", map[string]string{
						"grantee.#":      "1",
						"grantee.0.type": "Group",
						"grantee.0.uri":  "http://acs.amazonaws.com/groups/global/AllUsers",
						"permission":     "",
					}),
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

// TC-S3-BUCKET-ACL-05 Disappears - after the parent Bucket is deleted remotely, the associated ACL sub-resource should disappear from state.
// TestAccS3BucketACL_disappears deletes only the Bucket and verifies that the ACL sub-resource therefore needs re-creation.
func TestAccS3BucketACL_disappears(t *testing.T) {
	ctx := acctest.Context(t)
	bucketName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_acl.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyNoop,
		Steps: []resource.TestStep{
			{
				Config: testAccBucketACLConfig_cannedACL(bucketName, string(types.BucketCannedACLPrivate)),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketACLExists(ctx, t, resourceName),
					// Bucket ACL cannot be destroyed, but we can verify Bucket deletion
					// will result in a missing Bucket ACL resource.
					acctest.CheckSDKResourceDisappears(ctx, t, tfs3.ResourceBucket(), "ctyun_oos_bucket.test"),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TC-S3-BUCKET-ACL-01 Default ACL - reads back the default private ACL when no ACL is explicitly set.
// TestAccS3BucketACL_defaultACL verifies that the default private ACL is normalized into an empty-permission grant for AllUsers, rather than an empty grant list.
func TestAccS3BucketACL_defaultACL(t *testing.T) {
	ctx := acctest.Context(t)
	bucketName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_acl.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyNoop,
		Steps: []resource.TestStep{
			{
				Config: testAccBucketACLConfig_default(bucketName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketACLExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "acl", string(types.BucketCannedACLPrivate)),
					resource.TestCheckResourceAttr(resourceName, names.AttrID, bucketName),
					resource.TestCheckResourceAttr(resourceName, "access_control_policy.0.grant.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(resourceName, "access_control_policy.0.grant.*", map[string]string{
						"grantee.#":      "1",
						"grantee.0.type": "Group",
						"grantee.0.uri":  "http://acs.amazonaws.com/groups/global/AllUsers",
						"permission":     "",
					}),
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

// TC-S3-BUCKET-ACL-02 ACL Transitions - switches among the three canned ACLs and verifies state normalization.
// TestAccS3BucketACL_updateACL verifies that private, public-read, and public-read-write expand into empty-permission, READ, and FULL_CONTROL grants respectively.
func TestAccS3BucketACL_updateACL(t *testing.T) {
	ctx := acctest.Context(t)
	bucketName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_acl.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyNoop,
		Steps: []resource.TestStep{
			{
				Config: testAccBucketACLConfig_default(bucketName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketACLExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "acl", string(types.BucketCannedACLPrivate)),
					resource.TestCheckResourceAttr(resourceName, names.AttrID, bucketName),
					resource.TestCheckResourceAttr(resourceName, "access_control_policy.0.grant.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(resourceName, "access_control_policy.0.grant.*", map[string]string{
						"grantee.#":      "1",
						"grantee.0.type": "Group",
						"grantee.0.uri":  "http://acs.amazonaws.com/groups/global/AllUsers",
						"permission":     "",
					}),
				),
			},
			{
				Config: testAccBucketACLConfig_cannedACL(bucketName, string(types.BucketCannedACLPublicRead)),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketACLExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "acl", string(types.BucketCannedACLPublicRead)),
					resource.TestCheckResourceAttr(resourceName, names.AttrID, bucketName),
					resource.TestCheckResourceAttr(resourceName, "access_control_policy.0.grant.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(resourceName, "access_control_policy.0.grant.*", map[string]string{
						"grantee.#":      "1",
						"grantee.0.type": "Group",
						"grantee.0.uri":  "http://acs.amazonaws.com/groups/global/AllUsers",
						"permission":     "READ",
					}),
				),
			},
			{
				Config: testAccBucketACLConfig_cannedACL(bucketName, string(types.BucketCannedACLPublicReadWrite)),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketACLExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "acl", string(types.BucketCannedACLPublicReadWrite)),
					resource.TestCheckResourceAttr(resourceName, names.AttrID, bucketName),
					resource.TestCheckResourceAttr(resourceName, "access_control_policy.0.grant.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(resourceName, "access_control_policy.0.grant.*", map[string]string{
						"grantee.#":      "1",
						"grantee.0.type": "Group",
						"grantee.0.uri":  "http://acs.amazonaws.com/groups/global/AllUsers",
						"permission":     "FULL_CONTROL",
					}),
				),
			},
			{
				Config: testAccBucketACLConfig_cannedACL(bucketName, string(types.BucketCannedACLPrivate)),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketACLExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "acl", string(types.BucketCannedACLPrivate)),
					resource.TestCheckResourceAttr(resourceName, names.AttrID, bucketName),
					resource.TestCheckResourceAttr(resourceName, "access_control_policy.0.grant.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(resourceName, "access_control_policy.0.grant.*", map[string]string{
						"grantee.#":      "1",
						"grantee.0.type": "Group",
						"grantee.0.uri":  "http://acs.amazonaws.com/groups/global/AllUsers",
						"permission":     "",
					}),
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

// TC-S3-BUCKET-ACL-03 Remove Config - performs a remote read-back when the acl configuration is omitted
// TestAccS3BucketACL_removeFromConfigDefaultsToPrivate verifies that the public-read, default, and public-read-write round trips can perform a remote read-back
func TestAccS3BucketACL_removeFromConfigDefaultsToPrivate(t *testing.T) {
	ctx := acctest.Context(t)
	bucketName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_acl.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyNoop,
		Steps: []resource.TestStep{
			{
				Config: testAccBucketACLConfig_cannedACL(bucketName, string(types.BucketCannedACLPublicRead)),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketACLExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "acl", string(types.BucketCannedACLPublicRead)),
					resource.TestCheckResourceAttr(resourceName, names.AttrID, bucketName),
					resource.TestCheckResourceAttr(resourceName, "access_control_policy.0.grant.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(resourceName, "access_control_policy.0.grant.*", map[string]string{
						"grantee.#":      "1",
						"grantee.0.type": "Group",
						"grantee.0.uri":  "http://acs.amazonaws.com/groups/global/AllUsers",
						"permission":     "READ",
					}),
				),
			},
			{
				Config: testAccBucketACLConfig_default(bucketName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketACLExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "acl", string(types.BucketCannedACLPublicRead)),
					resource.TestCheckResourceAttr(resourceName, names.AttrID, bucketName),
					resource.TestCheckResourceAttr(resourceName, "access_control_policy.0.grant.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(resourceName, "access_control_policy.0.grant.*", map[string]string{
						"grantee.#":      "1",
						"grantee.0.type": "Group",
						"grantee.0.uri":  "http://acs.amazonaws.com/groups/global/AllUsers",
						"permission":     "READ",
					}),
				),
			},
			{
				Config: testAccBucketACLConfig_cannedACL(bucketName, string(types.BucketCannedACLPublicReadWrite)),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketACLExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "acl", string(types.BucketCannedACLPublicReadWrite)),
					resource.TestCheckResourceAttr(resourceName, names.AttrID, bucketName),
					resource.TestCheckResourceAttr(resourceName, "access_control_policy.0.grant.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(resourceName, "access_control_policy.0.grant.*", map[string]string{
						"grantee.#":      "1",
						"grantee.0.type": "Group",
						"grantee.0.uri":  "http://acs.amazonaws.com/groups/global/AllUsers",
						"permission":     "FULL_CONTROL",
					}),
				),
			},
			{
				Config: testAccBucketACLConfig_default(bucketName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketACLExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "acl", string(types.BucketCannedACLPublicReadWrite)),
					resource.TestCheckResourceAttr(resourceName, names.AttrID, bucketName),
					resource.TestCheckResourceAttr(resourceName, "access_control_policy.0.grant.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(resourceName, "access_control_policy.0.grant.*", map[string]string{
						"grantee.#":      "1",
						"grantee.0.type": "Group",
						"grantee.0.uri":  "http://acs.amazonaws.com/groups/global/AllUsers",
						"permission":     "FULL_CONTROL",
					}),
				),
			},
		},
	})
}

// TC-S3-BUCKET-ACL-03 Remove Config - removing the entire ACL resource should not modify the remote ACL.
// TestAccS3BucketACL_removeResourceRetainsRemoteACL verifies that after the resource is removed from state, the cloud Bucket's public-read ACL stays unchanged.
func TestAccS3BucketACL_removeResourceRetainsRemoteACL(t *testing.T) {
	ctx := acctest.Context(t)
	bucketName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_acl.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketACLConfig_cannedACL(bucketName, string(types.BucketCannedACLPublicRead)),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketACLExists(ctx, t, resourceName),
					testAccCheckBucketACL(ctx, t, bucketName, string(types.BucketCannedACLPublicRead)),
				),
			},
			{
				Config: testAccBucketACLConfig_bucketOnly(bucketName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckResourceNotInState(resourceName),
					testAccCheckBucketACL(ctx, t, bucketName, string(types.BucketCannedACLPublicRead)),
				),
			},
		},
	})
}

// TC-S3-BUCKET-ACL-04 Drift - the configured value is restored by a subsequent apply after the remote ACL drifts.
// TestAccS3BucketACL_drift verifies that after the private ACL becomes public-read remotely, both state and remote are restored to private.
func TestAccS3BucketACL_drift(t *testing.T) {
	ctx := acctest.Context(t)
	bucketName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_acl.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyNoop,
		Steps: []resource.TestStep{
			{
				Config: testAccBucketACLConfig_cannedACL(bucketName, string(types.BucketCannedACLPrivate)),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketACLExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "acl", string(types.BucketCannedACLPrivate)),
					testAccCheckBucketACL(ctx, t, bucketName, string(types.BucketCannedACLPrivate)),
				),
			},
			{
				PreConfig: func() {
					_, err := acctest.ProviderMeta(ctx, t).S3Client(ctx).CreateBucket(ctx, &awss3.CreateBucketInput{
						ACL:    types.BucketCannedACLPublicRead,
						Bucket: aws.String(bucketName),
					})
					if err != nil {
						t.Fatal(err)
					}
				},
				RefreshState: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketACLExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "acl", string(types.BucketCannedACLPublicRead)),
					testAccCheckBucketACL(ctx, t, bucketName, string(types.BucketCannedACLPublicRead)),
				),
				ExpectNonEmptyPlan: true,
			},
			{
				Config: testAccBucketACLConfig_cannedACL(bucketName, string(types.BucketCannedACLPrivate)),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketACLExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "acl", string(types.BucketCannedACLPrivate)),
					testAccCheckBucketACL(ctx, t, bucketName, string(types.BucketCannedACLPrivate)),
				),
			},
		},
	})
}

// TC-S3-BUCKET-ACL-06 Import - imports the default private ACL using the bucket as the Import ID.
// TestAccS3BucketACL_importBucketOnlyCanonicalizesID verifies that the state is normalized to the default private ACL after import.
func TestAccS3BucketACL_importBucketOnlyCanonicalizesID(t *testing.T) {
	ctx := acctest.Context(t)
	bucketName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_acl.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyNoop,
		Steps: []resource.TestStep{
			{
				Config: testAccBucketACLConfig_default(bucketName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketACLExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "acl", string(types.BucketCannedACLPrivate)),
					resource.TestCheckResourceAttr(resourceName, names.AttrID, bucketName),
				),
			},
			{
				ResourceName: resourceName,
				ImportState:  true,
				ImportStateIdFunc: func(_ *terraform.State) (string, error) {
					return bucketName, nil
				},
				ImportStateVerify: true,
			},
		},
	})
}

// TC-S3-BUCKET-ACL-09 Import Rejection - rejects unsupported Import IDs carrying an ACL.
// TestAccS3BucketACL_importBucketAndACLRejected verifies that only bucket is supported as the Import ID.
func TestAccS3BucketACL_importBucketAndACLRejected(t *testing.T) {
	ctx := acctest.Context(t)
	bucketName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_acl.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyNoop,
		Steps: []resource.TestStep{
			{
				Config: testAccBucketACLConfig_cannedACL(bucketName, string(types.BucketCannedACLPublicRead)),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketACLExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "acl", string(types.BucketCannedACLPublicRead)),
					resource.TestCheckResourceAttr(resourceName, names.AttrID, bucketName),
				),
			},
			{
				ResourceName: resourceName,
				ImportState:  true,
				ImportStateIdFunc: func(_ *terraform.State) (string, error) {
					return fmt.Sprintf("%s,%s", bucketName, string(types.BucketCannedACLPublicRead)), nil
				},
				ExpectError: regexache.MustCompile(`unexpected format for ID .* expected BUCKET`),
			},
		},
	})
}

// TC-S3-BUCKET-ACL-07 Invalid ACL And Bucket Negative - illegal ACLs are rejected by the schema.
// TestAccS3BucketACL_InvalidACLAndBucketNegative_invalidACL verifies that inputs outside the canned ACLs do not trigger remote operations.
func TestAccS3BucketACL_InvalidACLAndBucketNegative_invalidACL(t *testing.T) {
	ctx := acctest.Context(t)
	bucketName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketACLConfig_cannedACL(bucketName, "invalid-acl"),
				ExpectError: regexache.MustCompile(`(?i)expected acl to be one of .*got`),
			},
		},
	})
}

// TC-S3-BUCKET-ACL-07 Invalid ACL And Bucket Negative - eight categories of illegal bucket names return clear errors.
// TestAccS3BucketACL_InvalidACLAndBucketNegative_invalidBucketNames distinguishes local schema validation from OOS server-side hit validation.
func TestAccS3BucketACL_InvalidACLAndBucketNegative_invalidBucketNames(t *testing.T) {
	testCases := []struct {
		name        string
		bucket      string
		errorRegexp string
	}{
		{name: "consecutive-dots", bucket: "foo..bar", errorRegexp: `api error InvalidBucketName`},
		{name: "uppercase", bucket: "Foo.Bar", errorRegexp: `must contain only lowercase letters, numbers, periods, and hyphens`},
		{name: "public-ipv4", bucket: "192.168.0.1", errorRegexp: `api error InvalidBucketName`},
		{name: "loopback-ipv4", bucket: "127.0.0.1", errorRegexp: `api error InvalidBucketName`},
		{name: "leading-dot", bucket: ".foo", errorRegexp: `api error InvalidBucketName`},
		{name: "trailing-dot", bucket: "bar.", errorRegexp: `api error InvalidBucketName`},
		{name: "underscore", bucket: "foo_bar", errorRegexp: `must contain only lowercase letters, numbers, periods, and hyphens`},
		{name: "too-long", bucket: strings.Repeat("a", 64), errorRegexp: `expected length of bucket to be in the range \(3 - 63\), got`},
		{name: "empty-string", bucket: "", errorRegexp: `expected length of bucket to be in the range \(3 - 63\), got`},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := acctest.Context(t)

			acctest.ParallelTest(ctx, t, resource.TestCase{
				PreCheck:                 func() { acctest.PreCheck(ctx, t) },
				ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
				ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
				CheckDestroy:             testAccCheckBucketDestroy(ctx, t),
				Steps: []resource.TestStep{
					{
						Config:      testAccBucketACLConfig_cannedACL(testCase.bucket, string(types.BucketCannedACLPrivate)),
						ExpectError: regexache.MustCompile(testCase.errorRegexp),
					},
				},
			})
		})
	}
}

// TC-S3-BUCKET-ACL-07 Invalid ACL And Bucket Negative - an ACL sub-resource cannot be created for a non-existent bucket.
// TestAccS3BucketACL_InvalidACLAndBucketNegative_nonexistentBucket verifies that the Bucket read before Create returns a server-side error.
func TestAccS3BucketACL_InvalidACLAndBucketNegative_nonexistentBucket(t *testing.T) {
	ctx := acctest.Context(t)
	bucketName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyNoop,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketACLConfig_aclOnly(bucketName, string(types.BucketCannedACLPrivate)),
				ExpectError: regexache.MustCompile(`reading OOS Bucket.*: couldn't find resource`),
			},
		},
	})
}
func testAccCheckBucketACLExists(ctx context.Context, t *testing.T, n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		bucket, _, err := tfs3.ParseBucketACLResourceID(rs.Primary.ID)
		if err != nil {
			return err
		}

		conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)

		_, err = tfs3.FindBucketACL(ctx, conn, bucket)

		return err
	}
}

func testAccBucketACLConfig_aclOnly(bucketName, acl string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket_acl" "test" {
  bucket = %[1]q
  acl    = %[2]q
}
`, bucketName, acl)
}
func testAccBucketACLConfig_bucketOnly(bucketName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}
`, bucketName)
}
func testAccBucketACLConfig_default(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_acl" "test" {
  bucket = ctyun_oos_bucket.test.bucket
}
`, rName)
}

func testAccBucketACLConfig_cannedACL(rName, acl string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_acl" "test" {
  bucket = ctyun_oos_bucket.test.bucket
  acl    = %[2]q
}
`, rName, acl)
}
