// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package iam

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	awstypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/sdkdiag"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/retry"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/tfresource"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/aws-sdk-go-base/v2/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// @SDKResource("ctyun_iam_group", name="Group")
func resourceGroup() *schema.Resource {
	return &schema.Resource{
		CreateWithoutTimeout: resourceGroupCreate,
		ReadWithoutTimeout:   resourceGroupRead,
		DeleteWithoutTimeout: resourceGroupDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			names.AttrARN: {
				Type:     schema.TypeString,
				Computed: true,
			},
			names.AttrName: {
				Type:     schema.TypeString,
				ForceNew: true,
				Required: true,
				ValidateFunc: validation.All(
					validation.StringLenBetween(1, 128),
					validation.StringMatch(
						regexache.MustCompile(`^[0-9A-Za-z=,.@\-_+]+$`),
						"must only contain alphanumeric characters, hyphens, underscores, commas, periods, @ symbols, plus and equals signs",
					),
				),
			},
			"unique_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},

		CustomizeDiff: func(_ context.Context, d *schema.ResourceDiff, meta any) error {
			if d.HasChange(names.AttrName) {
				return d.SetNewComputed(names.AttrARN)
			}

			return nil
		},
	}
}

func resourceGroupCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	name := d.Get(names.AttrName).(string)
	input := &iam.CreateGroupInput{
		GroupName: aws.String(name),
	}

	output, err := conn.CreateGroup(ctx, input)

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "creating IAM Group (%s): %s", name, err)
	}

	d.SetId(aws.ToString(output.Group.GroupName))

	_, err = tfresource.RetryWhenNotFound(ctx, propagationTimeout, func(ctx context.Context) (any, error) {
		return findGroupByName(ctx, conn, d.Id())
	})

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "waiting for IAM Group (%s) create: %s", d.Id(), err)
	}

	return append(diags, resourceGroupRead(ctx, d, meta)...)
}

func resourceGroupRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	group, err := findGroupByName(ctx, conn, d.Id())

	if !d.IsNewResource() && retry.NotFound(err) {
		log.Printf("[WARN] IAM Group (%s) not found, removing from state", d.Id())
		d.SetId("")
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading IAM Group (%s): %s", d.Id(), err)
	}

	d.Set(names.AttrARN, group.Arn)
	d.Set(names.AttrName, group.GroupName)
	d.Set("unique_id", group.GroupId)

	return diags
}

func resourceGroupDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	log.Printf("[DEBUG] Deleting IAM Group: %s", d.Id())
	_, err := conn.DeleteGroup(ctx, &iam.DeleteGroupInput{
		GroupName: aws.String(d.Id()),
	})

	if errs.IsA[*awstypes.NoSuchEntityException](err) || tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "deleting IAM Group (%s): %s", d.Id(), err)
	}

	return diags
}

func findGroupByName(ctx context.Context, conn *iam.Client, name string) (*awstypes.Group, error) {
	input := &iam.GetGroupInput{
		GroupName: aws.String(name),
	}

	output, err := conn.GetGroup(ctx, input, withOOSIAMGetGroupResponse())

	if errs.IsA[*awstypes.NoSuchEntityException](err) || tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
		return nil, &retry.NotFoundError{
			LastError: err,
		}
	}

	if err != nil {
		return nil, err
	}

	if output == nil || output.Group == nil {
		return nil, tfresource.NewEmptyResultError()
	}

	return output.Group, nil
}

func deleteGroupPolicyAttachments(ctx context.Context, conn *iam.Client, groupName string) error {
	var attachedPolicies []awstypes.AttachedPolicy
	input := &iam.ListAttachedGroupPoliciesInput{
		GroupName: aws.String(groupName),
	}

	pages := iam.NewListAttachedGroupPoliciesPaginator(conn, input)
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx, withOOSIAMListAttachedPoliciesResponse())

		if errs.IsA[*awstypes.NoSuchEntityException](err) || tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("listing IAM Group (%s) policy attachments for deletion: %w", groupName, err)
		}

		attachedPolicies = append(attachedPolicies, page.AttachedPolicies...)
	}

	for _, attachedPolicy := range attachedPolicies {
		input := &iam.DetachGroupPolicyInput{
			GroupName: aws.String(groupName),
			PolicyArn: attachedPolicy.PolicyArn,
		}

		_, err := conn.DetachGroupPolicy(ctx, input)

		if errs.IsA[*awstypes.NoSuchEntityException](err) || tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
			continue
		}

		if err != nil {
			return fmt.Errorf("detaching IAM Group (%s) policy (%s): %w", groupName, aws.ToString(attachedPolicy.PolicyArn), err)
		}
	}

	return nil
}
