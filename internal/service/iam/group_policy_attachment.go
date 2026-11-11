// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package iam

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	awstypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/sdkdiag"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/retry"
	tfslices "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/slices"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/tfresource"
	inttypes "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/types"
	"github.com/hashicorp/aws-sdk-go-base/v2/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// @SDKResource("ctyun_iam_group_policy_attachment", name="Group Policy Attachment")
// @IdentityAttribute("group")
// @IdentityAttribute("policy_arn")
// @IdAttrFormat("{group}/{policy_arn}")
// @ImportIDHandler("groupPolicyAttachmentImportID")
// @Testing(existsType="github.com/aws/aws-sdk-go-v2/service/iam/types;types.AttachedPolicy", hasNoPreExistingResource=true)
func resourceGroupPolicyAttachment() *schema.Resource {
	return &schema.Resource{
		CreateWithoutTimeout: resourceGroupPolicyAttachmentCreate,
		ReadWithoutTimeout:   resourceGroupPolicyAttachmentRead,
		DeleteWithoutTimeout: resourceGroupPolicyAttachmentDelete,

		Schema: map[string]*schema.Schema{
			"group": {
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
			"policy_arn": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringLenBetween(20, 2048),
			},
		},
	}
}

func resourceGroupPolicyAttachmentCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	group := d.Get("group").(string)
	policyARN := d.Get("policy_arn").(string)

	if err := attachPolicyToGroup(ctx, conn, group, policyARN); err != nil {
		return sdkdiag.AppendFromErr(diags, err)
	}

	d.SetId(createGroupPolicyAttachmentImportID(d))

	return append(diags, resourceGroupPolicyAttachmentRead(ctx, d, meta)...)
}

func resourceGroupPolicyAttachmentRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	group := d.Get("group").(string)
	policyARN := d.Get("policy_arn").(string)
	// Human friendly ID for error messages since d.Id() is non-descriptive.
	id := fmt.Sprintf("%s:%s", group, policyARN)

	_, err := tfresource.RetryWhenNewResourceNotFound(ctx, propagationTimeout, func(ctx context.Context) (any, error) {
		return findAttachedGroupPolicyByTwoPartKey(ctx, conn, group, policyARN)
	}, d.IsNewResource())

	if !d.IsNewResource() && retry.NotFound(err) {
		log.Printf("[WARN] IAM Group Policy Attachment (%s) not found, removing from state", id)
		d.SetId("")
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading IAM Group Policy Attachment (%s): %s", id, err)
	}

	return diags
}

func resourceGroupPolicyAttachmentDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	if err := detachPolicyFromGroup(ctx, conn, d.Get("group").(string), d.Get("policy_arn").(string)); err != nil {
		return sdkdiag.AppendFromErr(diags, err)
	}

	return diags
}

func groupPolicyAttachmentID(groupName, policyARN string) string {
	return fmt.Sprintf("%s/%s", groupName, policyARN)
}

func createGroupPolicyAttachmentImportID(d *schema.ResourceData) string {
	return (groupPolicyAttachmentImportID{}).Create(d)
}

type groupPolicyAttachmentImportID struct{}

func (v groupPolicyAttachmentImportID) Create(d *schema.ResourceData) string {
	return v.create(d.Get("group").(string), d.Get("policy_arn").(string))
}

func (groupPolicyAttachmentImportID) create(groupName, policyARN string) string {
	return groupPolicyAttachmentID(groupName, policyARN)
}

func (groupPolicyAttachmentImportID) Parse(id string) (string, map[string]any, error) {
	parts := strings.SplitN(id, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", nil, fmt.Errorf("unexpected format for Import ID (%q), expected <group-name>/<policy_arn>", id)
	}

	result := map[string]any{
		"group":      parts[0],
		"policy_arn": parts[1],
	}
	return id, result, nil
}

func attachPolicyToGroup(ctx context.Context, conn *iam.Client, group, policyARN string) error {
	_, err := conn.AttachGroupPolicy(ctx, &iam.AttachGroupPolicyInput{
		GroupName: aws.String(group),
		PolicyArn: aws.String(policyARN),
	})

	if err != nil {
		return fmt.Errorf("attaching IAM Policy (%s) to IAM Group (%s): %w", policyARN, group, err)
	}

	return nil
}

func detachPolicyFromGroup(ctx context.Context, conn *iam.Client, group, policyARN string) error {
	_, err := conn.DetachGroupPolicy(ctx, &iam.DetachGroupPolicyInput{
		GroupName: aws.String(group),
		PolicyArn: aws.String(policyARN),
	})

	if errs.IsA[*awstypes.NoSuchEntityException](err) || tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("detaching IAM Policy (%s) from IAM Group (%s): %w", policyARN, group, err)
	}

	return nil
}

func findAttachedGroupPolicyByTwoPartKey(ctx context.Context, conn *iam.Client, groupName, policyARN string) (*awstypes.AttachedPolicy, error) {
	input := iam.ListAttachedGroupPoliciesInput{
		GroupName: aws.String(groupName),
	}

	return findAttachedGroupPolicy(ctx, conn, &input, func(v *awstypes.AttachedPolicy) bool {
		return aws.ToString(v.PolicyArn) == policyARN
	})
}

func findAttachedGroupPolicy(ctx context.Context, conn *iam.Client, input *iam.ListAttachedGroupPoliciesInput, filter tfslices.Predicate[*awstypes.AttachedPolicy]) (*awstypes.AttachedPolicy, error) {
	output, err := findAttachedGroupPolicies(ctx, conn, input, filter)

	if err != nil {
		return nil, err
	}

	return tfresource.AssertSingleValueResult(output)
}

func findAttachedGroupPolicies(ctx context.Context, conn *iam.Client, input *iam.ListAttachedGroupPoliciesInput, filter tfslices.Predicate[*awstypes.AttachedPolicy]) ([]awstypes.AttachedPolicy, error) {
	var output []awstypes.AttachedPolicy

	pages := iam.NewListAttachedGroupPoliciesPaginator(conn, input)
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx, withOOSIAMListAttachedPoliciesResponse())

		if errs.IsA[*awstypes.NoSuchEntityException](err) || tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
			return nil, &retry.NotFoundError{
				LastError: err,
			}
		}

		if err != nil {
			return nil, err
		}

		for _, v := range page.AttachedPolicies {
			if p := &v; !inttypes.IsZero(p) && filter(p) {
				output = append(output, v)
			}
		}
	}

	return output, nil
}
