// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package s3

import (
	"context"
	"fmt"
	"strings"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/sdkdiag"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/retry"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/sweep"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/sweep/awsv2"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/tfresource"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	sdkid "github.com/hashicorp/terraform-plugin-sdk/v2/helper/id"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func RegisterSweepers() {
	awsv2.Register("ctyun_oos_bucket", sweepBuckets)

	// OOS does not support directory buckets; the related sweepers are disabled.
	// awsv2.Register("aws_s3_directory_bucket", sweepDirectoryBuckets)
	// awsv2.Register("aws_s3_object_directory_bucket", sweepDirectoryBucketObjects)
}

const logKeyBucketName = "bucket_name"

func sweepBuckets(ctx context.Context, client *conns.AWSClient) ([]sweep.Sweepable, error) {
	conn := client.S3Client(ctx)
	r := resourceBucket()

	var sweepables []sweep.Sweepable

	input := s3.ListBucketsInput{}
	pages := s3.NewListBucketsPaginator(conn, &input)
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, bucket := range page.Buckets {
			bucketName := aws.ToString(bucket.Name)
			ctx = tflog.SetField(ctx, logKeyBucketName, bucketName)
			if !bucketNameFilter(ctx, bucket) {
				continue
			}

			var objectLockEnabled bool
			objLockConfig, err := findObjectLockConfiguration(ctx, conn, bucketName)
			if !retry.NotFound(err) {
				if err != nil {
					tflog.Warn(ctx, "Reading S3 Bucket Object Lock Configuration", map[string]any{
						"error": err.Error(),
					})
					continue
				}
				objectLockEnabled = objLockConfig.ObjectLockEnabled == string(types.ObjectLockEnabledEnabled)
			}

			d := r.Data(nil)
			d.SetId(bucketName)

			sweepables = append(sweepables, bucketSweeper{
				conn:     conn,
				bucket:   bucketName,
				locked:   objectLockEnabled,
				client:   client,
				resource: r,
				data:     d,
			})
		}
	}

	return sweepables, nil
}

type bucketSweeper struct {
	conn     *s3.Client
	bucket   string
	locked   bool
	client   *conns.AWSClient
	resource *schema.Resource
	data     *schema.ResourceData
}

func (bs bucketSweeper) Delete(ctx context.Context, optFns ...tfresource.OptionsFunc) error {
	tflog.Info(ctx, "Emptying S3 General Purpose Bucket")
	n, err := emptyBucket(ctx, bs.conn, bs.bucket, bs.locked)
	if err != nil {
		return fmt.Errorf("deleting S3 Bucket (%s) objects: %w", bs.bucket, err)
	}
	tflog.Info(ctx, "Deleted Objects from S3 General Purpose Bucket", map[string]any{
		"object_count": n,
	})

	return deleteResource(ctx, bs.resource, bs.data, bs.client)
}

func deleteResource(ctx context.Context, resource *schema.Resource, d *schema.ResourceData, meta *conns.AWSClient) error {
	if resource.DeleteContext != nil {
		return sdkdiag.DiagnosticsError(resource.DeleteContext(ctx, d, meta))
	}
	if resource.DeleteWithoutTimeout != nil {
		return sdkdiag.DiagnosticsError(resource.DeleteWithoutTimeout(ctx, d, meta))
	}

	return fmt.Errorf("resource does not implement DeleteContext/DeleteWithoutTimeout")
}

func bucketNameFilter(ctx context.Context, bucket types.Bucket) bool {
	name := aws.ToString(bucket.Name)

	prefixes := []string{
		"tf-acc",
		"tf-object-test",
		"tf-test",
		"tftest.applicationversion",
		"terraform-remote-s3-test",
		"aws-security-data-lake-", // Orphaned by aws_securitylake_data_lake.
		"resource-test-terraform",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}

	defaultNameRegexp := regexache.MustCompile(fmt.Sprintf(`^%s\d+$`, sdkid.UniqueIdPrefix))
	if defaultNameRegexp.MatchString(name) {
		return true
	}

	tflog.Info(ctx, "Skipping resource", map[string]any{
		"skip_reason": "no match on prefix list",
	})
	return false
}
