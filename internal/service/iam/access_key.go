// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package iam

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	awstypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/enum"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/sdkdiag"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/retry"
	tfslices "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/slices"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/tfresource"
	inttypes "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/aws-sdk-go-base/v2/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// @SDKResource("ctyun_iam_access_key", name="Access Key")
func resourceAccessKey() *schema.Resource {
	return &schema.Resource{
		CreateWithoutTimeout: resourceAccessKeyCreate,
		ReadWithoutTimeout:   resourceAccessKeyRead,
		UpdateWithoutTimeout: resourceAccessKeyUpdate,
		DeleteWithoutTimeout: resourceAccessKeyDelete,

		Importer: &schema.ResourceImporter{
			// ListAccessKeys requires UserName field in certain scenarios:
			//   ValidationError: Must specify userName when calling with non-User credentials
			// To prevent import from requiring this extra information, use GetAccessKeyLastUsed.
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta any) ([]*schema.ResourceData, error) {
				conn := meta.(*conns.AWSClient).IAMClient(ctx)

				input := &iam.GetAccessKeyLastUsedInput{
					AccessKeyId: aws.String(d.Id()),
				}

				output, err := conn.GetAccessKeyLastUsed(ctx, input)

				if err != nil {
					return nil, fmt.Errorf("fetching IAM Access Key (%s) username via GetAccessKeyLastUsed: %w", d.Id(), err)
				}

				if output == nil || output.UserName == nil {
					return nil, fmt.Errorf("fetching IAM Access Key (%s) username via GetAccessKeyLastUsed: empty response", d.Id())
				}

				d.Set("user", output.UserName)

				return []*schema.ResourceData{d}, nil
			},
		},

		Schema: map[string]*schema.Schema{
			"create_date": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"encrypted_secret": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"key_fingerprint": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"last_used_date": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"pgp_key": {
				Type:     schema.TypeString,
				ForceNew: true,
				Optional: true,
			},
			"secret": {
				Type:      schema.TypeString,
				Computed:  true,
				Sensitive: true,
			},
			"service_name": {
				Type:     schema.TypeString,
				Computed: true,
			},
			names.AttrStatus: {
				Type:             schema.TypeString,
				Optional:         true,
				Computed:         true,
				ValidateDiagFunc: enum.Validate[awstypes.StatusType](),
			},
			"user": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				ValidateFunc: validation.All(
					validation.StringLenBetween(1, 64),
					validation.StringMatch(
						regexache.MustCompile(`^[0-9A-Za-z=,.@\-_+]+$`),
						"must only contain alphanumeric characters, hyphens, underscores, commas, periods, @ symbols, plus and equals signs",
					),
				),
			},
		},
	}
}

func resourceAccessKeyCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	username := d.Get("user").(string)
	unlock := lockAccessKeys(username)
	defer unlock()

	request := &iam.CreateAccessKeyInput{
		UserName: aws.String(username),
	}

	var errConcurrentModification *awstypes.ConcurrentModificationException
	createResp, err := tfresource.RetryWhenAWSErrCodeEquals(ctx, propagationTimeout, func(ctx context.Context) (*iam.CreateAccessKeyOutput, error) {
		return conn.CreateAccessKey(ctx, request)
	}, errConcurrentModification.ErrorCode())
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "creating IAM Access Key (%s): %s", username, err)
	}

	d.SetId(aws.ToString(createResp.AccessKey.AccessKeyId))

	if createResp.AccessKey == nil || createResp.AccessKey.SecretAccessKey == nil {
		return sdkdiag.AppendErrorf(diags, "CreateAccessKey response did not contain a Secret Access Key as expected")
	}

	if v, ok := d.GetOk("pgp_key"); ok {
		pgpKey := v.(string)
		encryptionKey, err := retrieveGPGKey(pgpKey)
		if err != nil {
			return sdkdiag.AppendErrorf(diags, "creating IAM Access Key (%s): %s", username, err)
		}
		fingerprint, encrypted, err := encryptValue(encryptionKey, *createResp.AccessKey.SecretAccessKey, "IAM Access Key Secret")
		if err != nil {
			return sdkdiag.AppendErrorf(diags, "creating IAM Access Key (%s): %s", username, err)
		}

		d.Set("key_fingerprint", fingerprint)
		d.Set("encrypted_secret", encrypted)
	} else {
		d.Set("secret", createResp.AccessKey.SecretAccessKey)
	}

	if v, ok := d.GetOk(names.AttrStatus); ok && v.(string) == string(awstypes.StatusTypeInactive) {
		input := &iam.UpdateAccessKeyInput{
			AccessKeyId: aws.String(d.Id()),
			Status:      awstypes.StatusTypeInactive,
			UserName:    aws.String(d.Get("user").(string)),
		}

		_, err := conn.UpdateAccessKey(ctx, input)

		if err != nil {
			return sdkdiag.AppendErrorf(diags, "deactivating IAM Access Key (%s): %s", d.Id(), err)
		}

		createResp.AccessKey.Status = awstypes.StatusTypeInactive
	}

	resourceAccessKeyReadResult(d, &awstypes.AccessKeyMetadata{
		AccessKeyId: createResp.AccessKey.AccessKeyId,
		CreateDate:  createResp.AccessKey.CreateDate,
		Status:      createResp.AccessKey.Status,
		UserName:    createResp.AccessKey.UserName,
	})

	return diags
}

func resourceAccessKeyRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	username := d.Get("user").(string)
	key, err := findAccessKeyByTwoPartKey(ctx, conn, username, d.Id())

	if !d.IsNewResource() && retry.NotFound(err) {
		log.Printf("[WARN] IAM Access Key (%s) for User (%s) not found, removing from state", d.Id(), username)
		d.SetId("")
		return diags
	}
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading IAM Access Key (%s): %s", d.Id(), err)
	}

	d.SetId(aws.ToString(key.AccessKeyId))

	if key.CreateDate != nil {
		d.Set("create_date", aws.ToTime(key.CreateDate).Format(time.RFC3339))
	} else {
		d.Set("create_date", nil)
	}

	d.Set(names.AttrStatus, key.Status)
	d.Set("user", key.UserName)

	lastUsedOutput, err := conn.GetAccessKeyLastUsed(ctx, &iam.GetAccessKeyLastUsedInput{
		AccessKeyId: key.AccessKeyId,
	})
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading IAM Access Key (%s) last used: %s", d.Id(), err)
	}
	resourceAccessKeyLastUsedReadResult(d, lastUsedOutput)

	return diags
}

func resourceAccessKeyReadResult(d *schema.ResourceData, key *awstypes.AccessKeyMetadata) {
	d.SetId(aws.ToString(key.AccessKeyId))

	if key.CreateDate != nil {
		d.Set("create_date", aws.ToTime(key.CreateDate).Format(time.RFC3339))
	} else {
		d.Set("create_date", nil)
	}

	d.Set(names.AttrStatus, key.Status)
	d.Set("user", key.UserName)
}

func resourceAccessKeyLastUsedReadResult(d *schema.ResourceData, output *iam.GetAccessKeyLastUsedOutput) {
	if output == nil || output.AccessKeyLastUsed == nil {
		d.Set("last_used_date", nil)
		d.Set("service_name", nil)
		return
	}

	if output.AccessKeyLastUsed.LastUsedDate != nil {
		d.Set("last_used_date", aws.ToTime(output.AccessKeyLastUsed.LastUsedDate).Format(time.RFC3339))
	} else {
		d.Set("last_used_date", nil)
	}

	d.Set("service_name", aws.ToString(output.AccessKeyLastUsed.ServiceName))
}

func resourceAccessKeyUpdate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	if d.HasChange(names.AttrStatus) {
		if err := resourceAccessKeyStatusUpdate(ctx, conn, d); err != nil {
			return sdkdiag.AppendErrorf(diags, "updating IAM Access Key (%s): %s", d.Id(), err)
		}
	}

	return append(diags, resourceAccessKeyRead(ctx, d, meta)...)
}

func resourceAccessKeyDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)
	username := d.Get("user").(string)
	unlock := lockAccessKeys(username)
	defer unlock()

	log.Printf("[DEBUG] Deactivating IAM Access Key before delete: %s", d.Id())
	updateInput := &iam.UpdateAccessKeyInput{
		AccessKeyId: aws.String(d.Id()),
		Status:      awstypes.StatusTypeInactive,
		UserName:    aws.String(username),
	}

	_, err := conn.UpdateAccessKey(ctx, updateInput)

	if errs.IsA[*awstypes.NoSuchEntityException](err) || tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "deactivating IAM Access Key (%s) before delete: %s", d.Id(), err)
	}

	log.Printf("[DEBUG] Deleting IAM Access Key: %s", d.Id())
	input := iam.DeleteAccessKeyInput{
		AccessKeyId: aws.String(d.Id()),
		UserName:    aws.String(username),
	}
	var errConcurrentModification *awstypes.ConcurrentModificationException
	_, err = tfresource.RetryWhenAWSErrCodeEquals(ctx, propagationTimeout, func(ctx context.Context) (*iam.DeleteAccessKeyOutput, error) {
		return conn.DeleteAccessKey(ctx, &input)
	}, errConcurrentModification.ErrorCode())

	if errs.IsA[*awstypes.NoSuchEntityException](err) || tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "deleting IAM Access Key (%s): %s", d.Id(), err)
	}

	return diags
}

func resourceAccessKeyStatusUpdate(ctx context.Context, conn *iam.Client, d *schema.ResourceData) error {
	request := &iam.UpdateAccessKeyInput{
		AccessKeyId: aws.String(d.Id()),
		Status:      awstypes.StatusType(d.Get(names.AttrStatus).(string)),
		UserName:    aws.String(d.Get("user").(string)),
	}

	_, err := conn.UpdateAccessKey(ctx, request)
	return err
}

func findAccessKeyByTwoPartKey(ctx context.Context, conn *iam.Client, username, id string) (*awstypes.AccessKeyMetadata, error) {
	input := &iam.ListAccessKeysInput{
		UserName: aws.String(username),
	}

	return findAccessKey(ctx, conn, input, func(v *awstypes.AccessKeyMetadata) bool {
		return aws.ToString(v.AccessKeyId) == id
	})
}

func findAccessKeysByUser(ctx context.Context, conn *iam.Client, username string) ([]awstypes.AccessKeyMetadata, error) {
	input := &iam.ListAccessKeysInput{
		UserName: aws.String(username),
	}

	return findAccessKeys(ctx, conn, input, tfslices.PredicateTrue[*awstypes.AccessKeyMetadata]())
}

func findAccessKey(ctx context.Context, conn *iam.Client, input *iam.ListAccessKeysInput, filter tfslices.Predicate[*awstypes.AccessKeyMetadata]) (*awstypes.AccessKeyMetadata, error) {
	output, err := findAccessKeys(ctx, conn, input, filter)

	if err != nil {
		return nil, err
	}

	return tfresource.AssertSingleValueResult(output)
}

func findAccessKeys(ctx context.Context, conn *iam.Client, input *iam.ListAccessKeysInput, filter tfslices.Predicate[*awstypes.AccessKeyMetadata]) ([]awstypes.AccessKeyMetadata, error) {
	var output []awstypes.AccessKeyMetadata

	pages := iam.NewListAccessKeysPaginator(conn, input)
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)

		if errs.IsA[*awstypes.NoSuchEntityException](err) || tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
			return nil, &retry.NotFoundError{
				LastError: err,
			}
		}

		if err != nil {
			return nil, err
		}

		for _, v := range page.AccessKeyMetadata {
			if p := &v; !inttypes.IsZero(p) && filter(p) {
				output = append(output, v)
			}
		}
	}

	return output, nil
}
