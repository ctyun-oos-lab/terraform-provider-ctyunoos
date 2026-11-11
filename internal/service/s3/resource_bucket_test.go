// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package s3_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	tfs3 "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/s3"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/tfresource"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	sdkid "github.com/hashicorp/terraform-plugin-sdk/v2/helper/id"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func init() {
	acctest.RegisterServiceErrorCheckFunc(names.S3ServiceID, testAccErrorCheckSkip)
}

// testAccErrorCheckSkip skips tests that have error messages indicating unsupported features
func testAccErrorCheckSkip(t *testing.T) resource.ErrorCheckFunc {
	return acctest.ErrorCheckSkipMessagesContaining(t,
		"Number of distinct destination bucket ARNs cannot exceed",
		"destination is not allowed",
		"blocked by the BlockPublicAcls block public access setting",
	)
}

// TC-S3-BUCKET-01 Basic CRUD
// TestAccS3Bucket_Basic_basic verifies that a Bucket is created normally with only bucket set, that the bucket, ACL, and location states are read, that import works, and that the remote Bucket does not exist after destruction.
func TestAccS3Bucket_Basic_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, "tf-test-bucket")
	resourceName := "ctyun_oos_bucket.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "acl", "private"),
					resource.TestCheckResourceAttr(resourceName, names.AttrARN, fmt.Sprintf("arn:ctyun:oos:::%s", rName)),
					resource.TestCheckResourceAttr(resourceName, names.AttrBucket, rName),
					resource.TestCheckResourceAttr(resourceName, names.AttrBucketPrefix, ""),
					resource.TestCheckResourceAttr(resourceName, names.AttrForceDestroy, acctest.CtFalse),
					resource.TestCheckResourceAttr(resourceName, "lifecycle_rule.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "logging.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "object_lock_configuration.#", "0"),
					resource.TestCheckResourceAttr(resourceName, names.AttrPolicy, ""),
					resource.TestCheckResourceAttr(resourceName, "metadata_location_constraint.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "data_location_constraint.#", "0"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{names.AttrForceDestroy},
			},
		},
	})
}

// TC-S3-BUCKET-01 Basic CRUD - duplicate bucket name
// TestAccS3Bucket_Basic_duplicateName verifies that creating a bucket name already taken by another account makes OOS return an error
func TestAccS3Bucket_Basic_duplicateName(t *testing.T) {
	ctx := acctest.Context(t)
	bucketName := acctest.SkipIfEnvVarNotSet(t, "CTYUNOOS_OTHER_ACCOUNT_BUCKET_NAME")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketConfig_basic(bucketName),
				ExpectError: regexache.MustCompile(`(?i)BucketAlreadyExists`),
			},
		},
	})
}

// TC-S3-BUCKET-01 Basic CRUD - invalid bucket names
// TestAccS3Bucket_Basic_invalidName verifies that all eight categories of bucket names violating the naming rules return clear validation errors.
func TestAccS3Bucket_Basic_invalidName(t *testing.T) {
	testCases := []struct {
		name        string
		bucket      string
		errorRegexp string
	}{
		{name: "consecutive-dots", bucket: "foo..bar", errorRegexp: `(?i)InvalidBucketName`},
		{name: "uppercase", bucket: "Foo.Bar", errorRegexp: `must contain only lowercase letters, numbers, periods, and hyphens`},
		{name: "public-ipv4", bucket: "192.168.0.1", errorRegexp: `(?i)InvalidBucketName`},
		{name: "loopback-ipv4", bucket: "127.0.0.1", errorRegexp: `(?i)InvalidBucketName`},
		{name: "leading-dot", bucket: ".foo", errorRegexp: `(?i)InvalidBucketName`},
		{name: "trailing-dot", bucket: "bar.", errorRegexp: `(?i)InvalidBucketName`},
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
				Steps: []resource.TestStep{
					{
						Config:      testAccBucketConfig_basic(testCase.bucket),
						ExpectError: regexache.MustCompile(testCase.errorRegexp),
					},
				},
			})
		})
	}
}

// // TC-S3-BUCKET-01 Basic CRUD - non-empty destroy with force_destroy disabled
// // TestAccS3Bucket_Basic_nonEmptyForceDestroyFalse verifies that destroying a non-empty Bucket with force_destroy=false fails and returns BucketNotEmpty. (Excluded from automated testing because the final bucket destroy fails due to lingering soft-deleted objects.)
// func TestAccS3Bucket_Basic_nonEmptyForceDestroyFalse(t *testing.T) {
// 	ctx := acctest.Context(t)
// 	rName := acctest.RandomWithPrefix(t, "tf-test-bucket")
// 	resourceName := "ctyun_oos_bucket.test"

// 	acctest.ParallelTest(ctx, t, resource.TestCase{
// 		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
// 		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
// 		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
// 		CheckDestroy:             testAccCheckBucketDestroy(ctx, t),
// 		Steps: []resource.TestStep{
// 			{
// 				Config: testAccBucketConfig_forceDestroy(rName, false),
// 				Check: resource.ComposeAggregateTestCheckFunc(
// 					testAccCheckBucketExists(ctx, t, resourceName),
// 					resource.TestCheckResourceAttr(resourceName, names.AttrForceDestroy, acctest.CtFalse),
// 					testAccCheckPutBucketObject(ctx, t, rName, "non-empty-destroy-test"),
// 				),
// 			},
// 			{
// 				Config:      testAccBucketConfig_forceDestroy(rName, false),
// 				Destroy:     true,
// 				ExpectError: regexache.MustCompile(`(?is)BucketNotEmpty:\s*The bucket you tried to delete is not empty\.(?:\s*Expired and uncleaned objects still exist\.\s*Please try again later\.)?`),
// 			},
// 		},
// 	})
// }

// TestAccS3Bucket_Basic_nameGenerated verifies generated bucket names and import behavior.
func TestAccS3Bucket_Basic_nameGenerated(t *testing.T) {
	ctx := acctest.Context(t)
	resourceName := "ctyun_oos_bucket.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketConfig_nameGenerated(),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketExists(ctx, t, resourceName),
					acctest.CheckResourceAttrNameGenerated(resourceName, names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, names.AttrBucketPrefix, sdkid.UniqueIdPrefix),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrID, resourceName, names.AttrBucket),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{names.AttrForceDestroy},
			},
		},
	})
}

// TC-S3-BUCKET-02 Prefix Naming - valid prefix
// TestAccS3Bucket_PrefixNaming_basic verifies that a valid bucket_prefix generates the final Bucket name and saves it to state.
func TestAccS3Bucket_PrefixNaming_basic(t *testing.T) {
	ctx := acctest.Context(t)
	resourceName := "ctyun_oos_bucket.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketConfig_namePrefix("tf-test-"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketExists(ctx, t, resourceName),
					acctest.CheckResourceAttrNameFromPrefix(resourceName, names.AttrBucket, "tf-test-"),
					resource.TestCheckResourceAttr(resourceName, names.AttrBucketPrefix, "tf-test-"),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrID, resourceName, names.AttrBucket),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{names.AttrForceDestroy},
			},
		},
	})
}

// TC-S3-BUCKET-02 Prefix Naming - bucket and prefix conflict
// TestAccS3Bucket_PrefixNaming_conflictsWithBucket verifies that bucket and bucket_prefix cannot be configured together.
func TestAccS3Bucket_PrefixNaming_conflictsWithBucket(t *testing.T) {
	ctx := acctest.Context(t)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketConfig_nameConflict("tf-test-bucket", "tf-test-"),
				ExpectError: regexache.MustCompile(`(?is)bucket.*conflicts with.*bucket_prefix|bucket_prefix.*conflicts with.*bucket`),
			},
		},
	})
}

// TC-S3-BUCKET-02 Prefix Naming - stable re-apply and plan
// TestAccS3Bucket_PrefixNaming_stablePlan verifies that after repeatedly applying the same bucket_prefix, the final name stays unchanged and subsequent plans show no diff.
func TestAccS3Bucket_PrefixNaming_stablePlan(t *testing.T) {
	ctx := acctest.Context(t)
	resourceName := "ctyun_oos_bucket.test"
	prefix := "tf-test-stable-"
	var generatedBucketName string

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketConfig_namePrefix(prefix),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketExists(ctx, t, resourceName),
					testAccCheckBucketNameStable(resourceName, &generatedBucketName),
				),
			},
			{
				Config: testAccBucketConfig_namePrefix(prefix),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketExists(ctx, t, resourceName),
					testAccCheckBucketNameStable(resourceName, &generatedBucketName),
				),
			},
			{
				Config:             testAccBucketConfig_namePrefix(prefix),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// TC-S3-BUCKET-02 Prefix Naming - prefix boundaries
// TestAccS3Bucket_PrefixNaming_invalidNames verifies the length and character boundaries of bucket_prefix and of the generated name.
func TestAccS3Bucket_PrefixNaming_invalidNames(t *testing.T) {
	maxPrefixLength := 63 - sdkid.UniqueIDSuffixLength
	resourceName := "ctyun_oos_bucket.test"

	testCases := []struct {
		name        string
		prefix      string
		errorRegexp string
	}{
		{name: "empty-string", prefix: "", errorRegexp: fmt.Sprintf(`expected length of bucket_prefix to be in the range \(3 - %d\), got`, maxPrefixLength)},
		{name: "two-characters", prefix: "ab", errorRegexp: fmt.Sprintf(`expected length of bucket_prefix to be in the range \(3 - %d\), got`, maxPrefixLength)},
		{name: "too-long", prefix: strings.Repeat("a", maxPrefixLength+1), errorRegexp: fmt.Sprintf(`expected length of bucket_prefix to be in the range \(3 - %d\), got`, maxPrefixLength)},
		{name: "uppercase", prefix: "FooBar", errorRegexp: `must contain only lowercase letters, numbers, periods, and hyphens`},
		{name: "trailing-hyphen", prefix: "foo-"},
		{name: "space", prefix: "foo bar", errorRegexp: `must contain only lowercase letters, numbers, periods, and hyphens`},
		{name: "ip-address", prefix: "192.168.0.1", errorRegexp: `(?i)(InvalidBucketName|invalid bucket|bucket[^\n]*invalid)`},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := acctest.Context(t)
			step := resource.TestStep{Config: testAccBucketConfig_namePrefix(testCase.prefix)}

			if testCase.errorRegexp != "" {
				step.ExpectError = regexache.MustCompile(testCase.errorRegexp)
			} else {
				step.Check = resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketExists(ctx, t, resourceName),
					acctest.CheckResourceAttrNameFromPrefix(resourceName, names.AttrBucket, testCase.prefix),
				)
			}

			acctest.ParallelTest(ctx, t, resource.TestCase{
				PreCheck:                 func() { acctest.PreCheck(ctx, t) },
				ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
				ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
				CheckDestroy:             testAccCheckBucketDestroy(ctx, t),
				Steps:                    []resource.TestStep{step},
			})
		})
	}
}

// TC-S3-BUCKET-03 B Location Error - metadata location
// TestAccS3Bucket_BLocationError_metadataLocation verifies that the server does not allow specifying a metadata location when creating a Bucket.
func TestAccS3Bucket_BLocationError_metadataLocation(t *testing.T) {
	ctx := acctest.Context(t)
	bucketName := acctest.RandomWithPrefix(t, "tf-test-bucket")
	metadataLocation := acctest.OOSMetadataLocation()

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketConfig_metadataLocation(bucketName, metadataLocation),
				ExpectError: regexache.MustCompile(`(?i)MethodNotAllowed`),
			},
		},
	})
}

// TC-S3-BUCKET-03 B Location Error - data location
// TestAccS3Bucket_BLocationError_dataLocation verifies that the server does not allow specifying a data location when creating a Bucket.
func TestAccS3Bucket_BLocationError_dataLocation(t *testing.T) {
	ctx := acctest.Context(t)
	bucketName := acctest.RandomWithPrefix(t, "tf-test-bucket")
	dataLocation := acctest.OOSMetadataLocation()

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketConfig_dataLocationOnly(bucketName, dataLocation),
				ExpectError: regexache.MustCompile(`(?i)MethodNotAllowed`),
			},
		},
	})
}

// TC-S3-BUCKET-03 B Location Error - metadata and data location
// TestAccS3Bucket_BLocationError_metadataAndDataLocation verifies that the server does not allow specifying both a metadata location and a data location when creating a Bucket.
func TestAccS3Bucket_BLocationError_metadataAndDataLocation(t *testing.T) {
	ctx := acctest.Context(t)
	bucketName := acctest.RandomWithPrefix(t, "tf-test-bucket")
	metadataLocation := acctest.OOSMetadataLocation()
	dataLocation := acctest.OOSMetadataLocation()

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketConfig_metadataAndDataLocation(bucketName, metadataLocation, dataLocation),
				ExpectError: regexache.MustCompile(`(?i)MethodNotAllowed`),
			},
		},
	})
}

// TC-S3-BUCKET-06 ACL Update - supported values
// TestAccS3Bucket_ACLUpdate_supportedValues verifies that the ACL can be updated among private, public-read, and public-read-write.
func TestAccS3Bucket_ACLUpdate_supportedValues(t *testing.T) {
	ctx := acctest.Context(t)
	bucketName := acctest.RandomWithPrefix(t, "tf-test-bucket")
	resourceName := "ctyun_oos_bucket.test"
	steps := []resource.TestStep{}

	for _, acl := range []string{"private", "public-read", "public-read-write", "public-read", "private"} {
		steps = append(steps, resource.TestStep{
			Config: testAccBucketConfig_acl(bucketName, acl),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(resourceName, "acl", acl),
				testAccCheckBucketACL(ctx, t, bucketName, acl),
			),
		})
	}

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketDestroy(ctx, t),
		Steps:                    steps,
	})
}

// TC-S3-BUCKET-06 ACL Update - default private ACL
// TestAccS3Bucket_ACLUpdate_defaultPrivate verifies that a Bucket can be created without a locally configured ACL, and that the private ACL is automatically read back.
func TestAccS3Bucket_ACLUpdate_defaultPrivate(t *testing.T) {
	ctx := acctest.Context(t)
	bucketName := acctest.RandomWithPrefix(t, "tf-test-bucket")
	resourceName := "ctyun_oos_bucket.test"
	config := testAccBucketConfig_basic(bucketName)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, names.AttrBucket, bucketName),
					resource.TestCheckResourceAttr(resourceName, "acl", "private"),
					testAccCheckBucketACL(ctx, t, bucketName, "private"),
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

// TC-S3-BUCKET-06 ACL Update - default ACL
// TestAccS3Bucket_ACLUpdate_readback verifies a remote read-back after the explicit ACL configuration is removed
func TestAccS3Bucket_ACLUpdate_readback(t *testing.T) {
	ctx := acctest.Context(t)
	bucketName := acctest.RandomWithPrefix(t, "tf-test-bucket")
	resourceName := "ctyun_oos_bucket.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketConfig_acl(bucketName, "public-read"),
				Check:  testAccCheckBucketACL(ctx, t, bucketName, "public-read"),
			},
			{
				Config: testAccBucketConfig_basic(bucketName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "acl", "public-read"),
					testAccCheckBucketACL(ctx, t, bucketName, "public-read"),
				),
			},
		},
	})
}

// TC-S3-BUCKET-06 ACL Update - invalid ACL
// TestAccS3Bucket_ACLUpdate_invalidACL verifies that an illegal ACL value is rejected by schema validation.
func TestAccS3Bucket_ACLUpdate_invalidACL(t *testing.T) {
	ctx := acctest.Context(t)
	bucketName := acctest.RandomWithPrefix(t, "tf-test-bucket")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketConfig_acl(bucketName, "invalid-acl"),
				ExpectError: regexache.MustCompile(`expected acl to be one of \["private" "public-read" "public-read-write"\], got invalid-acl`),
			},
		},
	})
}

// TC-S3-BUCKET-06 ACL Update - remote drift
// TestAccS3Bucket_ACLUpdate_remoteDrift verifies that after the remote ACL drifts, apply restores the private ACL from the configuration.
func TestAccS3Bucket_ACLUpdate_remoteDrift(t *testing.T) {
	ctx := acctest.Context(t)
	bucketName := acctest.RandomWithPrefix(t, "tf-test-bucket")
	resourceName := "ctyun_oos_bucket.test"
	config := testAccBucketConfig_acl(bucketName, "private")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: config,
			},
			{
				PreConfig: func() {
					_, err := acctest.ProviderMeta(ctx, t).S3Client(ctx).CreateBucket(ctx, &s3.CreateBucketInput{
						Bucket: aws.String(bucketName),
						ACL:    "public-read",
					})
					if err != nil {
						t.Fatal(err)
					}
				},
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "acl", "private"),
					testAccCheckBucketACL(ctx, t, bucketName, "private"),
				),
			},
		},
	})
}

// TC-S3-BUCKET-07 Force Destroy - empty bucket
// TestAccS3Bucket_ForceDestroy_empty verifies that an empty Bucket is destroyed normally.
func TestAccS3Bucket_ForceDestroy_empty(t *testing.T) {
	ctx := acctest.Context(t)
	bucketName := acctest.RandomWithPrefix(t, "tf-test-bucket")
	resourceName := "ctyun_oos_bucket.test"
	config := testAccBucketConfig_forceDestroy(bucketName, true)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, names.AttrForceDestroy, acctest.CtTrue),
				),
			},
			{
				Config:  config,
				Destroy: true,
			},
		},
	})
}

// // TC-S3-BUCKET-07 Force Destroy - non-empty bucket enabled
// // TestAccS3Bucket_ForceDestroy_nonEmptyEnabled verifies that force_destroy=true empties the objects and then deletes the Bucket.
// // (Excluded from automated testing: the second step's destroy fails — the soft-delete issue prevents bucket deletion. Manually clear the table before testing; a 409 is reported afterwards; after clearing the softDelete table, sweep can clean up.)
// func TestAccS3Bucket_ForceDestroy_nonEmptyEnabled(t *testing.T) {
// 	ctx := acctest.Context(t)
// 	bucketName := acctest.RandomWithPrefix(t, "tf-test-bucket")
// 	resourceName := "ctyun_oos_bucket.test"
// 	config := testAccBucketConfig_forceDestroy(bucketName, true)

// 	acctest.ParallelTest(ctx, t, resource.TestCase{
// 		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
// 		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
// 		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
// 		CheckDestroy:             testAccCheckBucketDestroy(ctx, t),
// 		Steps: []resource.TestStep{
// 			{
// 				Config: config,
// 				Check: resource.ComposeAggregateTestCheckFunc(
// 					testAccCheckBucketExists(ctx, t, resourceName),
// 					testAccCheckPutBucketObject(ctx, t, bucketName, "object.txt"),
// 				),
// 			},
// 			{
// 				Config:  config,
// 				Destroy: true,
// 			},
// 		},
// 	})
// }

// // TC-S3-BUCKET-07 Force Destroy - non-empty bucket disabled
// // TestAccS3Bucket_ForceDestroy_nonEmptyDisabled verifies that deleting a non-empty Bucket with force_destroy=false returns BucketNotEmpty, and restores the cleanup configuration
// // (Excluded from automated testing: the final bucket cleanup fails — the soft-delete issue prevents bucket deletion. Manually clear the table before testing; a 409 is reported afterwards; after clearing the softDelete table, sweep can clean up.)
// func TestAccS3Bucket_ForceDestroy_nonEmptyDisabled(t *testing.T) {
// 	ctx := acctest.Context(t)
// 	bucketName := acctest.RandomWithPrefix(t, "tf-test-bucket")
// 	resourceName := "ctyun_oos_bucket.test"
// 	disabledConfig := testAccBucketConfig_forceDestroy(bucketName, false)
// 	enabledConfig := testAccBucketConfig_forceDestroy(bucketName, true)

// 	acctest.ParallelTest(ctx, t, resource.TestCase{
// 		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
// 		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
// 		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
// 		CheckDestroy:             testAccCheckBucketDestroy(ctx, t),
// 		Steps: []resource.TestStep{
// 			{
// 				Config: disabledConfig,
// 				Check: resource.ComposeAggregateTestCheckFunc(
// 					testAccCheckBucketExists(ctx, t, resourceName),
// 					testAccCheckPutBucketObject(ctx, t, bucketName, "object.txt"),
// 				),
// 			},
// 			{
// 				Config:      disabledConfig,
// 				Destroy:     true,
// 				ExpectError: regexache.MustCompile(`(?i)BucketNotEmpty`),
// 			},
// 			{
// 				Config: enabledConfig,
// 				Check:  resource.TestCheckResourceAttr(resourceName, names.AttrForceDestroy, acctest.CtTrue),
// 			},
// 		},
// 	})
// }

// // TC-S3-BUCKET-07 Force Destroy - nested objects
// // TestAccS3Bucket_ForceDestroy_nestedObjects verifies that multiple objects and nested key prefixes are all cleaned up.
// // (Excluded from automated testing: the second step's destroy fails — the soft-delete issue prevents bucket deletion. Manually clear the table before testing; a 409 is reported afterwards; after clearing the softDelete table, sweep can clean up.)
// func TestAccS3Bucket_ForceDestroy_nestedObjects(t *testing.T) {
// 	ctx := acctest.Context(t)
// 	bucketName := acctest.RandomWithPrefix(t, "tf-test-bucket")
// 	resourceName := "ctyun_oos_bucket.test"
// 	config := testAccBucketConfig_forceDestroy(bucketName, true)

// 	acctest.ParallelTest(ctx, t, resource.TestCase{
// 		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
// 		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
// 		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
// 		CheckDestroy:             testAccCheckBucketDestroy(ctx, t),
// 		Steps: []resource.TestStep{
// 			{
// 				Config: config,
// 				Check: resource.ComposeAggregateTestCheckFunc(
// 					testAccCheckBucketExists(ctx, t, resourceName),
// 					testAccCheckPutBucketObjects(ctx, t, bucketName, []string{
// 						"top.txt",
// 						"logs/2026/07/15/one.txt",
// 						"logs/2026/07/15/two.txt",
// 					}),
// 				),
// 			},
// 			{
// 				Config:  config,
// 				Destroy: true,
// 			},
// 		},
// 	})
// }

// TC-S3-BUCKET-08 Disappears - complete deletion
// TestAccS3Bucket_Disappears_completeDeletion verifies that after the Bucket is deleted remotely, the plan schedules a re-creation without leaving dirty state.
func TestAccS3Bucket_Disappears_completeDeletion(t *testing.T) {
	ctx := acctest.Context(t)
	bucketName := acctest.RandomWithPrefix(t, "tf-test-bucket")
	resourceName := "ctyun_oos_bucket.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketConfig_basic(bucketName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, names.AttrBucket, bucketName),
					resource.TestCheckResourceAttr(resourceName, "acl", "private"),
					acctest.CheckSDKResourceDisappears(ctx, t, tfs3.ResourceBucket(), resourceName),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TC-S3-BUCKET-08 Disappears - refresh removes missing bucket from state
// TestAccS3Bucket_Disappears_refreshAfterDeletion verifies that after the Bucket is deleted remotely, refresh removes the resource from state.
func TestAccS3Bucket_Disappears_refreshAfterDeletion(t *testing.T) {
	ctx := acctest.Context(t)
	bucketName := acctest.RandomWithPrefix(t, "tf-test-bucket")
	resourceName := "ctyun_oos_bucket.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketConfig_basic(bucketName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketExists(ctx, t, resourceName),
					acctest.CheckSDKResourceDisappears(ctx, t, tfs3.ResourceBucket(), resourceName),
				),
				ExpectNonEmptyPlan: true,
			},
			{
				RefreshState:       true,
				Check:              testAccCheckResourceNotInState(resourceName),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TC-S3-BUCKET-08 Disappears - remote configuration drift
// TestAccS3Bucket_Disappears_remoteConfigurationDrift verifies that when the Bucket still exists but its ACL is modified remotely, refresh reads back the drift and a subsequent apply converges to the local configuration.
func TestAccS3Bucket_Disappears_remoteConfigurationDrift(t *testing.T) {
	ctx := acctest.Context(t)
	bucketName := acctest.RandomWithPrefix(t, "tf-test-bucket")
	resourceName := "ctyun_oos_bucket.test"
	config := testAccBucketConfig_acl(bucketName, "private")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, names.AttrBucket, bucketName),
					resource.TestCheckResourceAttr(resourceName, "acl", "private"),
					testAccCheckBucketACL(ctx, t, bucketName, "private"),
					testAccCheckBucketRemoteACLDrift(ctx, t, resourceName, "public-read-write"),
				),
				ExpectNonEmptyPlan: true,
			},
			{
				RefreshState: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "acl", "public-read-write"),
					testAccCheckBucketACL(ctx, t, bucketName, "public-read-write"),
				),
				ExpectNonEmptyPlan: true,
			},
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "acl", "private"),
					testAccCheckBucketACL(ctx, t, bucketName, "private"),
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

// TC-S3-BUCKET-08 Disappears - recreate after deletion
// TestAccS3Bucket_Disappears_recreateAfterDeletion verifies that after the Bucket is deleted remotely, a subsequent apply re-creates the resource and produces an empty plan.
func TestAccS3Bucket_Disappears_recreateAfterDeletion(t *testing.T) {
	ctx := acctest.Context(t)
	bucketName := acctest.RandomWithPrefix(t, "tf-test-bucket")
	resourceName := "ctyun_oos_bucket.test"
	config := testAccBucketConfig_basic(bucketName)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketExists(ctx, t, resourceName),
					acctest.CheckSDKResourceDisappears(ctx, t, tfs3.ResourceBucket(), resourceName),
				),
				ExpectNonEmptyPlan: true,
			},
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, names.AttrBucket, bucketName),
					resource.TestCheckResourceAttr(resourceName, "acl", "private"),
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

// TC-S3-BUCKET-09 Import - existing bucket
// TestAccS3Bucket_Import_existing verifies that an existing Bucket can be imported by bucket name.
func TestAccS3Bucket_Import_existing(t *testing.T) {
	ctx := acctest.Context(t)
	bucketName := acctest.RandomWithPrefix(t, "tf-test-bucket")
	resourceName := "ctyun_oos_bucket.test"
	config := testAccBucketConfig_basic(bucketName)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check:  testAccCheckBucketExists(ctx, t, resourceName),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{names.AttrForceDestroy},
			},
		},
	})
}

// TC-S3-BUCKET-09 Import - creation state
// TestAccS3Bucket_Import_stateMatchesCreation verifies that the imported state matches the key fields of the creation path.
func TestAccS3Bucket_Import_stateMatchesCreation(t *testing.T) {
	ctx := acctest.Context(t)
	bucketName := acctest.RandomWithPrefix(t, "tf-test-bucket")
	resourceName := "ctyun_oos_bucket.test"
	config := testAccBucketConfig_inlineFeatures(bucketName, "access-logs/", true)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, names.AttrBucket, bucketName),
					resource.TestCheckResourceAttr(resourceName, "acl", "private"),
					resource.TestCheckResourceAttrSet(resourceName, names.AttrPolicy),
					resource.TestCheckResourceAttr(resourceName, "logging.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "logging.0.target_prefix", "access-logs/"),
					resource.TestCheckResourceAttr(resourceName, "lifecycle_rule.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "lifecycle_rule.0.expiration.0.days", "1"),
					resource.TestCheckResourceAttr(resourceName, "server_side_encryption_configuration.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "server_side_encryption_configuration.0.rule.0.apply_server_side_encryption_by_default.0.sse_algorithm", "AES256"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{names.AttrForceDestroy},
			},
		},
	})
}

// TC-S3-BUCKET-09 Import - missing bucket
// TestAccS3Bucket_Import_notFound verifies that importing a non-existent Bucket fails.
func TestAccS3Bucket_Import_notFound(t *testing.T) {
	ctx := acctest.Context(t)
	bucketName := acctest.RandomWithPrefix(t, "tf-test-missing-bucket")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:        testAccBucketConfig_basic(bucketName),
				ResourceName:  "ctyun_oos_bucket.test",
				ImportState:   true,
				ImportStateId: bucketName,
				ExpectError:   regexache.MustCompile(`(?i)(NoSuchBucket|not found|non-existent)`),
			},
		},
	})
}

// TC-S3-BUCKET-09 Import - stable plan
// TestAccS3Bucket_Import_stablePlan verifies that planning again after importing a bucket does not produce the same change.
func TestAccS3Bucket_Import_stablePlan(t *testing.T) {
	ctx := acctest.Context(t)
	bucketName := acctest.RandomWithPrefix(t, "tf-test-bucket")
	resourceName := "ctyun_oos_bucket.test"
	config := testAccBucketConfig_basic(bucketName)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					_, err := acctest.ProviderMeta(ctx, t).S3Client(ctx).CreateBucket(ctx, &s3.CreateBucketInput{
						Bucket: aws.String(bucketName),
						ACL:    "private",
					})
					if err != nil {
						t.Fatal(err)
					}
				},
				Config:             config,
				ResourceName:       resourceName,
				ImportState:        true,
				ImportStateId:      bucketName,
				ImportStatePersist: true,
			},
			{
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{
				Config: config,
			},
			{
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// TC-S3-BUCKET-09 Import - normalized remote state
// TestAccS3Bucket_Import_normalizedInlineFeatures verifies that remote fields such as the ACL are normalized after import.
func TestAccS3Bucket_Import_normalizedInlineFeatures(t *testing.T) {
	ctx := acctest.Context(t)
	bucketName := acctest.RandomWithPrefix(t, "tf-test-bucket")
	resourceName := "ctyun_oos_bucket.test"
	config := testAccBucketConfig_importNormalizedInlineFeatures(bucketName)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "acl", "public-read-write"),
					resource.TestCheckResourceAttrSet(resourceName, names.AttrPolicy),
					resource.TestCheckResourceAttr(resourceName, "logging.0.target_prefix", "access-logs/"),
					resource.TestCheckResourceAttr(resourceName, "lifecycle_rule.0.expiration.0.days", "365"),
					resource.TestCheckResourceAttr(resourceName, "server_side_encryption_configuration.0.rule.0.apply_server_side_encryption_by_default.0.sse_algorithm", "AES256"),
					resource.TestCheckResourceAttr(resourceName, "object_lock_configuration.0.object_lock_enabled", "Enabled"),
					resource.TestCheckResourceAttr(resourceName, "object_lock_configuration.0.rule.0.default_retention.0.days", "60"),
					resource.TestCheckResourceAttr(resourceName, "metadata_location_constraint.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "data_location_constraint.#", "0"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{names.AttrForceDestroy},
			},
		},
	})
}

// TC-S3-BUCKET-10 Inline Features Idempotence - single policy
// TestAccS3Bucket_InlineFeatures_singlePolicy verifies that repeated applies show no diff when only an inline policy is configured.
func TestAccS3Bucket_InlineFeatures_singlePolicy(t *testing.T) {
	testAccBucketInlineFeature(t, testAccBucketConfig_inlinePolicy, func(resourceName string) resource.TestCheckFunc {
		return resource.TestCheckResourceAttrSet(resourceName, names.AttrPolicy)
	}, func(bucketName string) resource.TestStep {
		return resource.TestStep{
			Config:             testAccBucketConfig_inlinePolicyEquivalent(bucketName),
			PlanOnly:           true,
			ExpectNonEmptyPlan: false,
		}
	})
}

// TC-S3-BUCKET-10 Inline Features Idempotence - single logging
// TestAccS3Bucket_InlineFeatures_singleLogging verifies that repeated applies show no diff when only inline logging is configured.
func TestAccS3Bucket_InlineFeatures_singleLogging(t *testing.T) {
	testAccBucketInlineFeature(t, func(bucketName string) string {
		return testAccBucketConfig_inlineLogging(bucketName, "access-logs/")
	}, func(resourceName string) resource.TestCheckFunc {
		return resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr(resourceName, "logging.#", "1"),
			resource.TestCheckResourceAttr(resourceName, "logging.0.target_prefix", "access-logs/"),
		)
	})
}

// TC-S3-BUCKET-10 Inline Features Idempotence - single lifecycle
// TestAccS3Bucket_InlineFeatures_singleLifecycle verifies that repeated applies show no diff when only an inline lifecycle is configured.
func TestAccS3Bucket_InlineFeatures_singleLifecycle(t *testing.T) {
	testAccBucketInlineFeature(t, testAccBucketConfig_inlineLifecycle, func(resourceName string) resource.TestCheckFunc {
		return resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr(resourceName, "lifecycle_rule.#", "1"),
			resource.TestCheckResourceAttr(resourceName, "lifecycle_rule.0.expiration.0.days", "1"),
		)
	})
}

// TC-S3-BUCKET-10 Inline Features Idempotence - single encryption
// TestAccS3Bucket_InlineFeatures_singleEncryption verifies that repeated applies show no diff when only inline encryption is configured.
func TestAccS3Bucket_InlineFeatures_singleEncryption(t *testing.T) {
	testAccBucketInlineFeature(t, testAccBucketConfig_inlineEncryption, func(resourceName string) resource.TestCheckFunc {
		return resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr(resourceName, "server_side_encryption_configuration.#", "1"),
			resource.TestCheckResourceAttr(resourceName, "server_side_encryption_configuration.0.rule.0.apply_server_side_encryption_by_default.0.sse_algorithm", "AES256"),
		)
	})
}

// TC-S3-BUCKET-10 Inline Features Idempotence - single ACL
// TestAccS3Bucket_InlineFeatures_singleACL verifies that repeated applies show no diff when only the ACL is configured.
func TestAccS3Bucket_InlineFeatures_singleACL(t *testing.T) {
	testAccBucketInlineFeature(t, func(bucketName string) string {
		return testAccBucketConfig_acl(bucketName, "public-read")
	}, func(resourceName string) resource.TestCheckFunc {
		return resource.TestCheckResourceAttr(resourceName, "acl", "public-read")
	})
}

// TC-S3-BUCKET-10 Inline Features Idempotence - single object lock
// TestAccS3Bucket_InlineFeatures_singleObjectLock verifies that repeated applies show no diff when only an inline object lock is configured.
func TestAccS3Bucket_InlineFeatures_singleObjectLock(t *testing.T) {
	testAccBucketInlineFeature(t, testAccBucketConfig_inlineObjectLock, func(resourceName string) resource.TestCheckFunc {
		return resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr(resourceName, "object_lock_configuration.#", "1"),
			resource.TestCheckResourceAttr(resourceName, "object_lock_configuration.0.object_lock_enabled", "Enabled"),
			resource.TestCheckResourceAttr(resourceName, "object_lock_configuration.0.rule.0.default_retention.0.mode", "COMPLIANCE"),
			resource.TestCheckResourceAttr(resourceName, "object_lock_configuration.0.rule.0.default_retention.0.days", "1"),
		)
	})
}

// TC-S3-BUCKET-10 Inline Features Idempotence - combined features
// TestAccS3Bucket_InlineFeatures_combined verifies that the combined configuration of policy, logging, lifecycle, and encryption is stable.
func TestAccS3Bucket_InlineFeatures_combined(t *testing.T) {
	testAccBucketInlineFeatures(t, testAccBucketConfig_inlineFeatures)
}

// TC-S3-BUCKET-10 Inline Features Idempotence - remove one block
// TestAccS3Bucket_InlineFeatures_removeLogging verifies that the other inline blocks remain stable after logging is removed.
func TestAccS3Bucket_InlineFeatures_removeLogging(t *testing.T) {
	testAccBucketInlineFeatureTransition(t, func(bucket string) string {
		return testAccBucketConfig_inlineFeaturesWithout(bucket, "")
	}, func(bucket string) string {
		return testAccBucketConfig_inlineFeaturesWithout(bucket, "logging")
	},
		func(resourceName, bucketName string) resource.TestCheckFunc {
			return resource.ComposeAggregateTestCheckFunc(
				testAccCheckBucketInlinePolicy(resourceName, bucketName, "oos:*"),
				testAccCheckBucketInlineLogging(resourceName, "log/"),
				testAccCheckBucketInlineLifecycle(resourceName, "1"),
				testAccCheckBucketInlineObjectLock(resourceName, "Disabled"),
				testAccCheckBucketInlineEncryption(resourceName),
				resource.TestCheckResourceAttr(resourceName, "acl", "public-read"),
			)
		})
}

// TC-S3-BUCKET-10 Inline Features Idempotence - remove policy from configuration
func TestAccS3Bucket_InlineFeatures_removePolicy(t *testing.T) {
	testAccBucketInlineFeatureTransition(t,
		func(bucket string) string {
			return testAccBucketConfig_inlineFeaturesWithout(bucket, "")
		},
		func(bucket string) string {
			return testAccBucketConfig_inlineFeaturesWithout(bucket, "policy")
		},
		func(resourceName, bucketName string) resource.TestCheckFunc {
			return resource.ComposeAggregateTestCheckFunc(
				testAccCheckBucketInlinePolicy(resourceName, bucketName, "oos:*"),
				testAccCheckBucketInlineLogging(resourceName, "log/"),
				testAccCheckBucketInlineLifecycle(resourceName, "1"),
				testAccCheckBucketInlineObjectLock(resourceName, "Disabled"),
				testAccCheckBucketInlineEncryption(resourceName),
				resource.TestCheckResourceAttr(resourceName, "acl", "public-read"),
			)
		},
	)
}

// TC-S3-BUCKET-10 Inline Features Idempotence - remove lifecycle
func TestAccS3Bucket_InlineFeatures_removeLifecycle(t *testing.T) {
	testAccBucketInlineFeatureTransition(t,
		func(bucket string) string {
			return testAccBucketConfig_inlineFeaturesWithout(bucket, "")
		},
		func(bucket string) string {
			return testAccBucketConfig_inlineFeaturesWithout(bucket, "lifecycle")
		},
		func(resourceName, bucketName string) resource.TestCheckFunc {
			return resource.ComposeAggregateTestCheckFunc(
				testAccCheckBucketInlinePolicy(resourceName, bucketName, "oos:*"),
				testAccCheckBucketInlineLogging(resourceName, "log/"),
				testAccCheckBucketInlineLifecycle(resourceName, "1"),
				testAccCheckBucketInlineObjectLock(resourceName, "Disabled"),
				testAccCheckBucketInlineEncryption(resourceName),
				resource.TestCheckResourceAttr(resourceName, "acl", "public-read"),
			)
		},
	)
}

// TC-S3-BUCKET-10 Inline Features Idempotence - remove object lock
func TestAccS3Bucket_InlineFeatures_removeObjectLock(t *testing.T) {
	testAccBucketInlineFeatureTransition(t,
		func(bucket string) string {
			return testAccBucketConfig_inlineFeaturesWithout(bucket, "")
		},
		func(bucket string) string {
			return testAccBucketConfig_inlineFeaturesWithout(bucket, "object_lock")
		},
		func(resourceName, bucketName string) resource.TestCheckFunc {
			return resource.ComposeAggregateTestCheckFunc(
				testAccCheckBucketInlinePolicy(resourceName, bucketName, "oos:*"),
				testAccCheckBucketInlineLogging(resourceName, "log/"),
				testAccCheckBucketInlineLifecycle(resourceName, "1"),
				testAccCheckBucketInlineObjectLock(resourceName, "Disabled"),
				testAccCheckBucketInlineEncryption(resourceName),
				resource.TestCheckResourceAttr(resourceName, "acl", "public-read"),
			)
		},
	)
}

// TC-S3-BUCKET-10 Inline Features Idempotence - remove ACL
func TestAccS3Bucket_InlineFeatures_removeACL(t *testing.T) {
	testAccBucketInlineFeatureTransition(t,
		func(bucket string) string {
			return testAccBucketConfig_inlineFeaturesWithout(bucket, "")
		},
		func(bucket string) string {
			return testAccBucketConfig_inlineFeaturesWithout(bucket, "acl")
		},
		func(resourceName, bucketName string) resource.TestCheckFunc {
			return resource.ComposeAggregateTestCheckFunc(
				testAccCheckBucketInlinePolicy(resourceName, bucketName, "oos:*"),
				testAccCheckBucketInlineLogging(resourceName, "log/"),
				testAccCheckBucketInlineLifecycle(resourceName, "1"),
				testAccCheckBucketInlineObjectLock(resourceName, "Disabled"),
				testAccCheckBucketInlineEncryption(resourceName),
				resource.TestCheckResourceAttr(resourceName, "acl", "public-read"),
			)
		},
	)
}

// TC-S3-BUCKET-10 Inline Features Idempotence - remove encryption
func TestAccS3Bucket_InlineFeatures_removeEncryption(t *testing.T) {
	testAccBucketInlineFeatureTransition(t,
		func(bucket string) string {
			return testAccBucketConfig_inlineFeaturesWithout(bucket, "")
		},
		func(bucket string) string {
			return testAccBucketConfig_inlineFeaturesWithout(bucket, "encryption")
		},
		func(resourceName, bucketName string) resource.TestCheckFunc {
			return resource.ComposeAggregateTestCheckFunc(
				testAccCheckBucketInlinePolicy(resourceName, bucketName, "oos:*"),
				testAccCheckBucketInlineLogging(resourceName, "log/"),
				testAccCheckBucketInlineLifecycle(resourceName, "1"),
				testAccCheckBucketInlineObjectLock(resourceName, "Disabled"),
				testAccCheckBucketInlineEncryption(resourceName),
				resource.TestCheckResourceAttr(resourceName, "acl", "public-read"),
			)
		},
	)
}

// TC-S3-BUCKET-10 Inline Features Idempotence - stable plan after removing logging
func TestAccS3Bucket_InlineFeatures_stablePlanLogging(t *testing.T) {
	testAccBucketInlineFeatureStablePlan(t,
		func(bucket string) string {
			return testAccBucketConfig_inlineFeaturesWithout(bucket, "")
		},
		func(bucket string) string {
			return testAccBucketConfig_inlineFeaturesWithout(bucket, "logging")
		},
		func(resourceName, bucketName string) resource.TestCheckFunc {
			return resource.ComposeAggregateTestCheckFunc(
				testAccCheckBucketInlinePolicy(resourceName, bucketName, "oos:*"),
				testAccCheckBucketInlineLogging(resourceName, "log/"),
				testAccCheckBucketInlineLifecycle(resourceName, "1"),
				testAccCheckBucketInlineObjectLock(resourceName, "Disabled"),
				testAccCheckBucketInlineEncryption(resourceName),
				resource.TestCheckResourceAttr(resourceName, "acl", "public-read"),
			)
		},
	)
}

// TC-S3-BUCKET-10 Inline Features Idempotence - stable plan after removing policy
func TestAccS3Bucket_InlineFeatures_stablePlanPolicy(t *testing.T) {
	testAccBucketInlineFeatureStablePlan(t,
		func(bucket string) string {
			return testAccBucketConfig_inlineFeaturesWithout(bucket, "")
		},
		func(bucket string) string {
			return testAccBucketConfig_inlineFeaturesWithout(bucket, "policy")
		},
		func(resourceName, bucketName string) resource.TestCheckFunc {
			return resource.ComposeAggregateTestCheckFunc(
				testAccCheckBucketInlinePolicy(resourceName, bucketName, "oos:*"),
				testAccCheckBucketInlineLogging(resourceName, "log/"),
				testAccCheckBucketInlineLifecycle(resourceName, "1"),
				testAccCheckBucketInlineObjectLock(resourceName, "Disabled"),
				testAccCheckBucketInlineEncryption(resourceName),
				resource.TestCheckResourceAttr(resourceName, "acl", "public-read"),
			)
		},
	)
}

// TC-S3-BUCKET-10 Inline Features Idempotence - stable plan after removing lifecycle
func TestAccS3Bucket_InlineFeatures_stablePlanLifecycle(t *testing.T) {
	testAccBucketInlineFeatureStablePlan(t,
		func(bucket string) string {
			return testAccBucketConfig_inlineFeaturesWithout(bucket, "")
		},
		func(bucket string) string {
			return testAccBucketConfig_inlineFeaturesWithout(bucket, "lifecycle")
		},
		func(resourceName, bucketName string) resource.TestCheckFunc {
			return resource.ComposeAggregateTestCheckFunc(
				testAccCheckBucketInlinePolicy(resourceName, bucketName, "oos:*"),
				testAccCheckBucketInlineLogging(resourceName, "log/"),
				testAccCheckBucketInlineLifecycle(resourceName, "1"),
				testAccCheckBucketInlineObjectLock(resourceName, "Disabled"),
				testAccCheckBucketInlineEncryption(resourceName),
				resource.TestCheckResourceAttr(resourceName, "acl", "public-read"),
			)
		},
	)
}

// TC-S3-BUCKET-10 Inline Features Idempotence - stable plan after removing object lock
func TestAccS3Bucket_InlineFeatures_stablePlanObjectLock(t *testing.T) {
	testAccBucketInlineFeatureStablePlan(t,
		func(bucket string) string {
			return testAccBucketConfig_inlineFeaturesWithout(bucket, "")
		},
		func(bucket string) string {
			return testAccBucketConfig_inlineFeaturesWithout(bucket, "object_lock")
		},
		func(resourceName, bucketName string) resource.TestCheckFunc {
			return resource.ComposeAggregateTestCheckFunc(
				testAccCheckBucketInlinePolicy(resourceName, bucketName, "oos:*"),
				testAccCheckBucketInlineLogging(resourceName, "log/"),
				testAccCheckBucketInlineLifecycle(resourceName, "1"),
				testAccCheckBucketInlineObjectLock(resourceName, "Disabled"),
				testAccCheckBucketInlineEncryption(resourceName),
				resource.TestCheckResourceAttr(resourceName, "acl", "public-read"),
			)
		},
	)
}

// TC-S3-BUCKET-10 Inline Features Idempotence - stable plan after removing ACL
func TestAccS3Bucket_InlineFeatures_stablePlanACL(t *testing.T) {
	testAccBucketInlineFeatureStablePlan(t,
		func(bucket string) string {
			return testAccBucketConfig_inlineFeaturesWithout(bucket, "")
		},
		func(bucket string) string {
			return testAccBucketConfig_inlineFeaturesWithout(bucket, "acl")
		},
		func(resourceName, bucketName string) resource.TestCheckFunc {
			return resource.ComposeAggregateTestCheckFunc(
				testAccCheckBucketInlinePolicy(resourceName, bucketName, "oos:*"),
				testAccCheckBucketInlineLogging(resourceName, "log/"),
				testAccCheckBucketInlineLifecycle(resourceName, "1"),
				testAccCheckBucketInlineObjectLock(resourceName, "Disabled"),
				testAccCheckBucketInlineEncryption(resourceName),
				resource.TestCheckResourceAttr(resourceName, "acl", "public-read"),
			)
		},
	)
}

// TC-S3-BUCKET-10 Inline Features Idempotence - stable plan after removing encryption
func TestAccS3Bucket_InlineFeatures_stablePlanEncryption(t *testing.T) {
	testAccBucketInlineFeatureStablePlan(t,
		func(bucket string) string {
			return testAccBucketConfig_inlineFeaturesWithout(bucket, "")
		},
		func(bucket string) string {
			return testAccBucketConfig_inlineFeaturesWithout(bucket, "encryption")
		},
		func(resourceName, bucketName string) resource.TestCheckFunc {
			return resource.ComposeAggregateTestCheckFunc(
				testAccCheckBucketInlinePolicy(resourceName, bucketName, "oos:*"),
				testAccCheckBucketInlineLogging(resourceName, "log/"),
				testAccCheckBucketInlineLifecycle(resourceName, "1"),
				testAccCheckBucketInlineObjectLock(resourceName, "Disabled"),
				testAccCheckBucketInlineEncryption(resourceName),
				resource.TestCheckResourceAttr(resourceName, "acl", "public-read"),
			)
		},
	)
}

// TC-S3-BUCKET-10 Inline Features Idempotence - update one block
// TestAccS3Bucket_InlineFeatures_updateLogging verifies that the other inline blocks remain stable when only logging is updated.
func TestAccS3Bucket_InlineFeatures_updateLogging(t *testing.T) {
	testAccBucketInlineFeatureTransition(t, func(bucket string) string {
		return testAccBucketConfig_inlineFeaturesUpdate(bucket)
	}, func(bucket string) string {
		return testAccBucketConfig_inlineFeaturesUpdate(bucket, "logging")
	}, func(resourceName, bucketName string) resource.TestCheckFunc {
		return resource.ComposeAggregateTestCheckFunc(
			testAccCheckBucketInlinePolicy(resourceName, bucketName, "oos:*"),
			testAccCheckBucketInlineLogging(resourceName, "log/b/"),
			testAccCheckBucketInlineLifecycle(resourceName, "1"),
			testAccCheckBucketInlineObjectLock(resourceName, "Disabled"),
			testAccCheckBucketInlineEncryption(resourceName),
			resource.TestCheckResourceAttr(resourceName, "acl", "public-read"),
		)
	})
}

// TC-S3-BUCKET-10 Inline Features Idempotence - update policy
func TestAccS3Bucket_InlineFeatures_updatePolicy(t *testing.T) {
	testAccBucketInlineFeatureTransition(t,
		func(bucket string) string { return testAccBucketConfig_inlineFeaturesUpdate(bucket) },
		func(bucket string) string { return testAccBucketConfig_inlineFeaturesUpdate(bucket, "policy") },
		func(resourceName, bucketName string) resource.TestCheckFunc {
			return resource.ComposeAggregateTestCheckFunc(
				testAccCheckBucketInlinePolicy(resourceName, bucketName, "oos:GetObject"),
				testAccCheckBucketInlineLogging(resourceName, "log/"),
				testAccCheckBucketInlineLifecycle(resourceName, "1"),
				testAccCheckBucketInlineObjectLock(resourceName, "Disabled"),
				testAccCheckBucketInlineEncryption(resourceName),
				resource.TestCheckResourceAttr(resourceName, "acl", "public-read"),
			)
		},
	)
}

// TC-S3-BUCKET-10 Inline Features Idempotence - update lifecycle
func TestAccS3Bucket_InlineFeatures_updateLifecycle(t *testing.T) {
	testAccBucketInlineFeatureTransition(t,
		func(bucket string) string { return testAccBucketConfig_inlineFeaturesUpdate(bucket) },
		func(bucket string) string { return testAccBucketConfig_inlineFeaturesUpdate(bucket, "lifecycle") },
		func(resourceName, bucketName string) resource.TestCheckFunc {
			return resource.ComposeAggregateTestCheckFunc(
				testAccCheckBucketInlinePolicy(resourceName, bucketName, "oos:*"),
				testAccCheckBucketInlineLogging(resourceName, "log/"),
				testAccCheckBucketInlineLifecycle(resourceName, "2"),
				testAccCheckBucketInlineObjectLock(resourceName, "Disabled"),
				testAccCheckBucketInlineEncryption(resourceName),
				resource.TestCheckResourceAttr(resourceName, "acl", "public-read"),
			)
		},
	)
}

// TC-S3-BUCKET-10 Inline Features Idempotence - update object lock
func TestAccS3Bucket_InlineFeatures_updateObjectLock(t *testing.T) {
	testAccBucketInlineFeatureTransition(t,
		func(bucket string) string { return testAccBucketConfig_inlineFeaturesUpdate(bucket) },
		func(bucket string) string { return testAccBucketConfig_inlineFeaturesUpdate(bucket, "object_lock") },
		func(resourceName, bucketName string) resource.TestCheckFunc {
			return resource.ComposeAggregateTestCheckFunc(
				testAccCheckBucketInlinePolicy(resourceName, bucketName, "oos:*"),
				testAccCheckBucketInlineLogging(resourceName, "log/"),
				testAccCheckBucketInlineLifecycle(resourceName, "1"),
				testAccCheckBucketInlineObjectLock(resourceName, "Enabled"),
				testAccCheckBucketInlineEncryption(resourceName),
				resource.TestCheckResourceAttr(resourceName, "acl", "public-read"),
			)
		},
	)
}

// TC-S3-BUCKET-10 Inline Features Idempotence - update ACL
func TestAccS3Bucket_InlineFeatures_updateACL(t *testing.T) {
	testAccBucketInlineFeatureTransition(t,
		func(bucket string) string { return testAccBucketConfig_inlineFeaturesUpdate(bucket) },
		func(bucket string) string { return testAccBucketConfig_inlineFeaturesUpdate(bucket, "acl") },
		func(resourceName, bucketName string) resource.TestCheckFunc {
			return resource.ComposeAggregateTestCheckFunc(
				testAccCheckBucketInlinePolicy(resourceName, bucketName, "oos:*"),
				testAccCheckBucketInlineLogging(resourceName, "log/"),
				testAccCheckBucketInlineLifecycle(resourceName, "1"),
				testAccCheckBucketInlineObjectLock(resourceName, "Disabled"),
				testAccCheckBucketInlineEncryption(resourceName),
				resource.TestCheckResourceAttr(resourceName, "acl", "public-read-write"),
			)
		},
	)
}

func testAccBucketInlineFeatureTransition(t *testing.T, initial, updated func(string) string, check func(string, string) resource.TestCheckFunc) {
	ctx := acctest.Context(t)
	bucketName := acctest.RandomWithPrefix(t, "tf-test-bucket")
	resourceName := "ctyun_oos_bucket.test"
	updatedConfig := updated(bucketName)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketDestroy(ctx, t),
		Steps: []resource.TestStep{
			{Config: initial(bucketName)},
			{Config: updatedConfig, Check: check(resourceName, bucketName)},
			{Config: updatedConfig, PlanOnly: true, ExpectNonEmptyPlan: false},
		},
	})
}

// Create the bucket with the full inline configuration.
// Apply with the configuration that removes the corresponding inline block, allowing one ExpectNonEmptyPlan: true to cover "a one-time diff may appear after remote default values are back-filled".
// Apply again with the same configuration and re-assert that the returned values are stable.
// Run PlanOnly with the same configuration, requiring ExpectNonEmptyPlan: false, verifying the same change is no longer proposed.
func testAccBucketInlineFeatureStablePlan(t *testing.T, initial, updated func(string) string, check func(string, string) resource.TestCheckFunc) {
	ctx := acctest.Context(t)
	bucketName := acctest.RandomWithPrefix(t, "tf-test-bucket")
	resourceName := "ctyun_oos_bucket.test"
	updatedConfig := updated(bucketName)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketDestroy(ctx, t),
		Steps: []resource.TestStep{
			{Config: initial(bucketName)},
			// {Config: updatedConfig, Check: check(resourceName, bucketName), ExpectNonEmptyPlan: true},
			{Config: updatedConfig, Check: check(resourceName, bucketName)},
			{Config: updatedConfig, PlanOnly: true, ExpectNonEmptyPlan: false},
		},
	})
}
func testAccBucketInlineFeature(t *testing.T, configFunc func(string) string, check func(string) resource.TestCheckFunc, extraSteps ...func(string) resource.TestStep) {
	ctx := acctest.Context(t)
	bucketName := acctest.RandomWithPrefix(t, "tf-test-bucket")
	resourceName := "ctyun_oos_bucket.test"
	config := configFunc(bucketName)
	steps := []resource.TestStep{
		{
			Config: config,
			Check: resource.ComposeAggregateTestCheckFunc(
				testAccCheckBucketExists(ctx, t, resourceName),
				check(resourceName),
			),
		},
	}

	for _, step := range extraSteps {
		steps = append(steps, step(bucketName))
	}

	steps = append(steps, resource.TestStep{
		Config:             config,
		PlanOnly:           true,
		ExpectNonEmptyPlan: false,
	})

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketDestroy(ctx, t),
		Steps:                    steps,
	})
}
func testAccBucketInlineFeatures(t *testing.T, configFunc func(string, string, bool) string) {
	ctx := acctest.Context(t)
	bucketName := acctest.RandomWithPrefix(t, "tf-test-bucket")
	resourceName := "ctyun_oos_bucket.test"
	config := configFunc(bucketName, "log/", true)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, names.AttrPolicy),
					resource.TestCheckResourceAttr(resourceName, "logging.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "lifecycle_rule.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "server_side_encryption_configuration.#", "1"),
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

// TC-S3-BUCKET-10 Inline Features Idempotence - removed encryption can be re-managed
// TestAccS3Bucket_InlineFeatures_removeEncryptionRetainsRemoteAndReadds verifies that after the inline server-side encryption is removed from the configuration, the remote configuration is retained and can be brought under management again.
func TestAccS3Bucket_InlineFeatures_removeEncryptionRetainsRemoteAndReadds(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, "tf-test-bucket")
	resourceName := "ctyun_oos_bucket.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketConfig_inlineEncryption(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "server_side_encryption_configuration.#", "1"),
					testAccCheckBucketInlineServerSideEncryptionConfigurationRetained(ctx, t, rName),
				),
			},
			{
				Config: testAccBucketConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketExists(ctx, t, resourceName),
					testAccCheckBucketInlineServerSideEncryptionConfigurationRetained(ctx, t, rName),
				),
			},
			{
				Config: testAccBucketConfig_inlineEncryption(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "server_side_encryption_configuration.#", "1"),
					testAccCheckBucketInlineServerSideEncryptionConfigurationRetained(ctx, t, rName),
				),
			},
		},
	})
}

// TestAccS3Bucket_ACLUpdate verifies that the bucket ACL can be updated between private and public-read and restored.
func TestAccS3Bucket_aclUpdate(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, "tf-test-bucket")
	resourceName := "ctyun_oos_bucket.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "acl", "private"),
				),
			},
			{
				Config: testAccBucketConfig_acl(rName, "public-read"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "acl", "public-read"),
				),
			},
			{
				Config: testAccBucketConfig_acl(rName, "private"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "acl", "private"),
				),
			},
		},
	})
}

func testAccCheckBucketDestroy(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "ctyun_oos_bucket" {
				continue
			}

			// S3 seems to be highly eventually consistent. Even if one connection reports that the queue is gone,
			// another connection may still report it as present.
			_, err := tfresource.RetryUntilNotFound(ctx, tfs3.BucketPropagationTimeout, func(ctx context.Context) (any, error) {
				return tfs3.FindOOSBucket(ctx, conn, rs.Primary.ID)
			})

			if errors.Is(err, tfresource.ErrFoundResource) {
				return fmt.Errorf("S3 Bucket %s still exists", rs.Primary.ID)
			}

			if err != nil {
				return err
			}

			continue
		}

		return nil
	}
}

func testAccCheckBucketInlinePolicy(resourceName, bucketName, action string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", resourceName)
		}

		var policy map[string]any
		if err := json.Unmarshal([]byte(rs.Primary.Attributes[names.AttrPolicy]), &policy); err != nil {
			return fmt.Errorf("decoding bucket policy: %w", err)
		}
		if policy["Version"] != "2012-10-17" || policy["Id"] != "*" {
			return fmt.Errorf("unexpected bucket policy version or ID: %#v", policy)
		}

		statements, ok := policy["Statement"].([]any)
		if !ok || len(statements) != 1 {
			return fmt.Errorf("unexpected bucket policy statements: %#v", policy["Statement"])
		}
		statement, ok := statements[0].(map[string]any)
		if !ok {
			return fmt.Errorf("unexpected bucket policy statement: %#v", statements[0])
		}
		if statement["Effect"] != "Allow" || statement["Action"] != action || statement["Resource"] != fmt.Sprintf("arn:ctyun:oos:::%s/*", bucketName) {
			return fmt.Errorf("unexpected bucket policy statement values: %#v", statement)
		}

		principal, ok := statement["Principal"].(map[string]any)
		if !ok || principal["CTYUN"] != "arn:ctyun:iam:::user/test" {
			return fmt.Errorf("unexpected bucket policy principal: %#v", statement["Principal"])
		}
		return nil
	}
}
func testAccCheckResourceNotInState(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if _, ok := s.RootModule().Resources[n]; ok {
			return fmt.Errorf("expected %s to be removed from state", n)
		}

		return nil
	}
}

func testAccCheckBucketRemoteACLDrift(ctx context.Context, t *testing.T, n, acl string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		_, err := acctest.ProviderMeta(ctx, t).S3Client(ctx).CreateBucket(ctx, &s3.CreateBucketInput{
			Bucket: aws.String(rs.Primary.ID),
			ACL:    types.BucketCannedACL(acl),
		})

		return err
	}
}
func testAccCheckBucketExists(ctx context.Context, t *testing.T, n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)

		_, err := tfs3.FindOOSBucket(ctx, conn, rs.Primary.ID)

		return err
	}
}

func testAccCheckBucketNameStable(n string, expected *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		bucketName := rs.Primary.Attributes[names.AttrBucket]
		if bucketName == "" {
			return fmt.Errorf("%s: bucket name is empty", n)
		}

		if *expected == "" {
			*expected = bucketName
			return nil
		}

		if bucketName != *expected {
			return fmt.Errorf("%s: bucket name changed from %q to %q", n, *expected, bucketName)
		}

		return nil
	}
}

func testAccCheckBucketACL(ctx context.Context, t *testing.T, bucket, want string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		o, err := tfs3.FindBucketACL(ctx, acctest.ProviderMeta(ctx, t).S3Client(ctx), bucket)
		if err != nil {
			return err
		}
		got := "private"
		for _, g := range o.Grants {
			if g.Grantee != nil && aws.ToString(g.Grantee.URI) == "http://acs.amazonaws.com/groups/global/AllUsers" {
				if string(g.Permission) == "READ" {
					got = "public-read"
				}
				if string(g.Permission) == "FULL_CONTROL" {
					got = "public-read-write"
				}
			}
		}
		if got != want {
			return fmt.Errorf("OOS Bucket (%s) ACL mismatch: got %q, want %q", bucket, got, want)
		}
		return nil
	}
}

func testAccCheckBucketInlineServerSideEncryptionConfigurationRetained(ctx context.Context, t *testing.T, bucket string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)

		_, err := tfs3.FindServerSideEncryptionConfiguration(ctx, conn, bucket)
		return err
	}
}

func testAccBucketConfig_basic(bucketName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}
`, bucketName)
}

func testAccBucketConfig_metadataLocation(bucketName, metadataLocation string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q

  metadata_location_constraint {
    location = %[2]q
  }
}
`, bucketName, metadataLocation)
}

func testAccBucketConfig_forceDestroy(bucketName string, forceDestroy bool) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket        = %[1]q
  force_destroy = %[2]t
}
`, bucketName, forceDestroy)
}

func testAccBucketConfig_namePrefix(namePrefix string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket_prefix = %[1]q
}
`, namePrefix)
}

func testAccBucketConfig_nameConflict(bucketName, namePrefix string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket        = %[1]q
  bucket_prefix = %[2]q
}
`, bucketName, namePrefix)
}

func testAccBucketConfig_nameGenerated() string {
	return `
resource "ctyun_oos_bucket" "test" {
}
`
}

func testAccBucketConfig_importNormalizedInlineFeatures(bucket string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "access_log" {
  bucket = "%[1]s-access-log"
}

resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
  acl    = "public-read-write"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid       = "PublicObjectRead"
        Effect    = "Allow"
        Principal = "*"
        Action    = ["oos:GetObject"]
        Resource  = "arn:ctyun:oos:::%[1]s/*"
      },
    ]
  })

  logging {
    target_bucket = ctyun_oos_bucket.access_log.bucket
    target_prefix = "access-logs/"
  }

  lifecycle_rule {
    id      = "expire-objects"
    enabled = true
    prefix  = ""

    expiration {
      days = 365
    }
  }

  server_side_encryption_configuration {
    rule {
      apply_server_side_encryption_by_default {
        sse_algorithm = "AES256"
      }
    }
  }

  object_lock_configuration {
    object_lock_enabled = "Enabled"

    rule {
      default_retention {
        mode = "COMPLIANCE"
        days = 60
      }
    }
  }
}
`, bucket)
}

func testAccBucketConfig_dataLocationOnly(bucketName, dataLocation string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q

  data_location_constraint {
    type              = "Specified"
    locations         = [%[2]q]
    schedule_strategy = "Allowed"
  }
}
`, bucketName, dataLocation)
}

func testAccBucketConfig_metadataAndDataLocation(bucketName, metadataLocation, dataLocation string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q

  metadata_location_constraint {
    location = %[2]q
  }

  data_location_constraint {
    type              = "Specified"
    locations         = [%[3]q]
    schedule_strategy = "Allowed"
  }
}
`, bucketName, metadataLocation, dataLocation)
}

func testAccBucketConfig_acl(bucketName, acl string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
  acl    = %[2]q
}
`, bucketName, acl)
}

func testAccBucketConfig_inlineObjectLock(bucketName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q

  object_lock_configuration {
    object_lock_enabled = "Enabled"

    rule {
      default_retention {
        mode = "COMPLIANCE"
        days = 1
      }
    }
  }
}
`, bucketName)
}

func testAccBucketConfig_inlineLifecycle(bucketName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q

  lifecycle_rule {
    id      = "inline"
    enabled = true
    prefix  = ""

    expiration {
      days = 1
    }
  }
}
`, bucketName)
}

func testAccBucketConfig_inlineEncryption(bucketName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q

  server_side_encryption_configuration {
    rule {
      apply_server_side_encryption_by_default {
        sse_algorithm = "AES256"
      }
    }
  }
}
`, bucketName)
}

func testAccBucketConfig_inlineFeatures(bucketName, targetPrefix string, logging bool) string {
	logBucket := ""
	loggingBlock := ""

	if logging {
		logBucket = fmt.Sprintf(`
resource "ctyun_oos_bucket" "log" {
  bucket = "%[1]s-log"
}
`, bucketName)

		loggingBlock = fmt.Sprintf(`

  logging {
    target_bucket = ctyun_oos_bucket.log.bucket
    target_prefix = %[1]q
  }`, targetPrefix)
	}

	return fmt.Sprintf(`%[2]s
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q

  policy = jsonencode({
    Version = "2012-10-17"
    Id      = "*"
    Statement = [
      {
        Sid    = "*"
        Effect = "Allow"
        Principal = {
          CTYUN = "arn:ctyun:iam:::user/test"
        }
        Action   = "oos:*"
        Resource = "arn:ctyun:oos:::%[1]s/*"
      },
    ]
  })

  lifecycle_rule {
    id      = "inline"
    enabled = true
    prefix  = ""

    expiration {
      days = 1
    }
  }

  server_side_encryption_configuration {
    rule {
      apply_server_side_encryption_by_default {
        sse_algorithm = "AES256"
      }
    }
  }
%[3]s
}
`, bucketName, logBucket, loggingBlock)
}

// testAccBucketConfig_inlineFeaturesWithout returns the full inline
// feature configuration while removing exactly one feature from local configuration.
// testAccBucketConfig_inlineFeaturesUpdate returns the complete inline feature configuration.
// When a feature is supplied, only that feature uses its update value.
func testAccBucketConfig_inlineFeaturesUpdate(bucketName string, feature ...string) string {
	config := testAccBucketConfig_inlineFeaturesWithout(bucketName, "")
	if len(feature) == 0 {
		return config
	}

	switch feature[0] {
	case "policy":
		return strings.Replace(config, `Action   = "oos:*"`, `Action   = "oos:GetObject"`, 1)
	case "logging":
		return strings.Replace(config, `target_prefix = "log/"`, `target_prefix = "log/b/"`, 1)
	case "lifecycle":
		return strings.Replace(config, `expiration { days = 1 }`, `expiration { days = 2 }`, 1)
	case "object_lock":
		return strings.Replace(config, `object_lock_enabled = "Disabled"`, `object_lock_enabled = "Enabled"`, 1)
	case "acl":
		return strings.Replace(config, `acl = "public-read"`, `acl = "public-read-write"`, 1)
	case "encryption":
		return config
	default:
		return config
	}
}
func testAccBucketConfig_inlineFeaturesWithout(bucketName, omittedFeature string) string {
	policyBlock := fmt.Sprintf(`
  policy = jsonencode({
    Version = "2012-10-17"
    Id      = "*"
    Statement = [{
      Sid    = "*"
      Effect = "Allow"
      Principal = { CTYUN = "arn:ctyun:iam:::user/test" }
      Action   = "oos:*"
      Resource = "arn:ctyun:oos:::%[1]s/*"
    }]
  })`, bucketName)
	loggingBlock := `
  logging {
    target_bucket = ctyun_oos_bucket.log.bucket
    target_prefix = "log/"
  }`
	lifecycleBlock := `
  lifecycle_rule {
    id      = "inline"
    enabled = true
    prefix  = ""

    expiration { days = 1 }
  }`
	encryptionBlock := `
  server_side_encryption_configuration {
    rule {
      apply_server_side_encryption_by_default { sse_algorithm = "AES256" }
    }
  }`
	objectlockBlock := `
  object_lock_configuration {
    object_lock_enabled = "Disabled"
    rule {
      default_retention {
        mode = "COMPLIANCE"
        days = 1
      }
    }
  }`
	aclBlock := `
  acl = "public-read"`

	if omittedFeature == "policy" {
		policyBlock = ""
	}
	if omittedFeature == "logging" {
		loggingBlock = ""
	}
	if omittedFeature == "lifecycle" {
		lifecycleBlock = ""
	}
	if omittedFeature == "encryption" {
		encryptionBlock = ""
	}
	if omittedFeature == "object_lock" {
		objectlockBlock = ""
	}
	if omittedFeature == "acl" {
		aclBlock = ""
	}

	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "log" {
  bucket = "%[1]s-log"
}

resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
%[2]s
%[3]s
%[4]s
%[5]s
%[6]s
%[7]s
}
`, bucketName, policyBlock, loggingBlock, lifecycleBlock, encryptionBlock, objectlockBlock, aclBlock)
}

func testAccCheckBucketInlineLogging(resourceName, targetPrefix string) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttr(resourceName, "logging.#", "1"),
		resource.TestCheckResourceAttr(resourceName, "logging.0.target_prefix", targetPrefix),
	)
}

func testAccCheckBucketInlineLifecycle(resourceName, expirationDays string) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttr(resourceName, "lifecycle_rule.#", "1"),
		resource.TestCheckResourceAttr(resourceName, "lifecycle_rule.0.id", "inline"),
		resource.TestCheckResourceAttr(resourceName, "lifecycle_rule.0.enabled", "true"),
		resource.TestCheckResourceAttr(resourceName, "lifecycle_rule.0.prefix", ""),
		resource.TestCheckResourceAttr(resourceName, "lifecycle_rule.0.expiration.0.days", expirationDays),
	)
}

func testAccCheckBucketInlineObjectLock(resourceName, objectLockEnabled string) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttr(resourceName, "object_lock_configuration.#", "1"),
		resource.TestCheckResourceAttr(resourceName, "object_lock_configuration.0.object_lock_enabled", objectLockEnabled),
		resource.TestCheckResourceAttr(resourceName, "object_lock_configuration.0.rule.0.default_retention.0.mode", "COMPLIANCE"),
		resource.TestCheckResourceAttr(resourceName, "object_lock_configuration.0.rule.0.default_retention.0.days", "1"),
	)
}

func testAccCheckBucketInlineEncryption(resourceName string) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttr(resourceName, "server_side_encryption_configuration.#", "1"),
		resource.TestCheckResourceAttr(resourceName, "server_side_encryption_configuration.0.rule.0.apply_server_side_encryption_by_default.0.sse_algorithm", "AES256"),
	)
}
func testAccBucketConfig_inlineLogging(bucketName, targetPrefix string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "log_bucket" {
  bucket = "%[1]s-log"
}

resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q

  logging {
    target_bucket = ctyun_oos_bucket.log_bucket.bucket
    target_prefix = %[2]q
  }
}
`, bucketName, targetPrefix)
}

func testAccBucketConfig_inlinePolicy(bucketName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q

  policy = jsonencode({
    Version = "2012-10-17"
    Id      = "*"
    Statement = [
      {
        Sid    = "*"
        Effect = "Allow"
        Principal = {
          CTYUN = "arn:ctyun:iam:::user/test"
        }
        Action   = "oos:*"
        Resource = "arn:ctyun:oos:::%[1]s/*"
      },
    ]
  })
}
`, bucketName)
}

func testAccBucketConfig_inlinePolicyEquivalent(bucketName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
  policy = <<POLICY
{
  "Statement": [
    {
      "Resource": "arn:ctyun:oos:::%[1]s/*",
      "Action": "oos:*",
      "Principal": {
        "CTYUN": "arn:ctyun:iam:::user/test"
      },
      "Effect": "Allow",
      "Sid": "*"
    }
  ],
  "Id": "*",
  "Version": "2012-10-17"
}
POLICY
}
`, bucketName)
}
