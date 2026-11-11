// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package iam

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	awstypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/create"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/sdkdiag"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/retry"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/tfresource"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/verify"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/aws-sdk-go-base/v2/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	sdkid "github.com/hashicorp/terraform-plugin-sdk/v2/helper/id"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/structure"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

const (
	policyNameMaxLen       = 128
	policyNamePrefixMaxLen = policyNameMaxLen - sdkid.UniqueIDSuffixLength
)

// @SDKResource("ctyun_iam_policy", name="Policy")
// @Testing(existsType="github.com/aws/aws-sdk-go-v2/service/iam/types;types.Policy")
// @ArnIdentity
// @Testing(hasNoPreExistingResource=true)
func resourcePolicy() *schema.Resource {
	return &schema.Resource{
		CreateWithoutTimeout: resourcePolicyCreate,
		ReadWithoutTimeout:   resourcePolicyRead,
		UpdateWithoutTimeout: resourcePolicyCreate,
		DeleteWithoutTimeout: resourcePolicyDelete,

		Schema: map[string]*schema.Schema{
			names.AttrARN: {
				Type:     schema.TypeString,
				Computed: true,
			},
			"attachment_count": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			names.AttrDescription: {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringLenBetween(0, 1000),
			},
			names.AttrName: {
				Type:          schema.TypeString,
				Optional:      true,
				Computed:      true,
				ForceNew:      true,
				ConflictsWith: []string{names.AttrNamePrefix},
				ValidateFunc:  validResourceName(policyNameMaxLen),
			},
			names.AttrNamePrefix: {
				Type:          schema.TypeString,
				Optional:      true,
				Computed:      true,
				ForceNew:      true,
				ConflictsWith: []string{names.AttrName},
				ValidateFunc:  validResourceName(policyNamePrefixMaxLen),
			},
			names.AttrPolicy: {
				Type:                  schema.TypeString,
				Required:              true,
				ValidateFunc:          validation.All(validation.StringLenBetween(1, 131072), verify.ValidIAMPolicyJSON),
				DiffSuppressFunc:      verify.SuppressEquivalentPolicyDiffs,
				DiffSuppressOnRefresh: true,
				StateFunc: func(v any) string {
					json, _ := structure.NormalizeJsonString(v)
					return json
				},
			},
			"policy_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func resourcePolicyCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	policy, err := structure.NormalizeJsonString(d.Get(names.AttrPolicy).(string))
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "policy (%s) is invalid JSON: %s", policy, err)
	}

	name := create.Name(ctx, d.Get(names.AttrName).(string), d.Get(names.AttrNamePrefix).(string))
	input := iam.CreatePolicyInput{
		Description:    aws.String(d.Get(names.AttrDescription).(string)),
		PolicyDocument: aws.String(policy),
		PolicyName:     aws.String(name),
	}

	output, err := conn.CreatePolicy(ctx, &input)

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "creating IAM Policy (%s): %s", name, err)
	}

	d.SetId(aws.ToString(output.Policy.Arn))

	return append(diags, resourcePolicyRead(ctx, d, meta)...)
}

func resourcePolicyRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	type policyWithVersion struct {
		policy        *awstypes.Policy
		policyVersion *awstypes.PolicyVersion
	}
	output, err := tfresource.RetryWhenNewResourceNotFound(ctx, propagationTimeout, func(ctx context.Context) (*policyWithVersion, error) {
		getPolicyOutput, err := getPolicyByARN(ctx, conn, d.Id())
		if err != nil {
			return nil, err
		}

		policyDocument, _ := getPolicyOutput.ResultMetadata.Get(oosIAMPolicyDocumentMetadataKey).(string)
		if policyDocument == "" {
			return nil, fmt.Errorf("GetPolicy response did not include policy document")
		}

		return &policyWithVersion{
			policy: getPolicyOutput.Policy,
			policyVersion: &awstypes.PolicyVersion{
				Document:  aws.String(policyDocument),
				VersionId: getPolicyOutput.Policy.DefaultVersionId,
			},
		}, nil
	}, d.IsNewResource())

	if !d.IsNewResource() && retry.NotFound(err) {
		log.Printf("[WARN] IAM Policy (%s) not found, removing from state", d.Id())
		d.SetId("")
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading IAM Policy (%s): %s", d.Id(), err)
	}

	resourcePolicyFlatten(output.policy, d)

	d.Set(names.AttrNamePrefix, create.NamePrefixFromName(aws.ToString(output.policy.PolicyName)))

	if err := resourcePolicyFlattenPolicyDocument(aws.ToString(output.policyVersion.Document), d); err != nil {
		return sdkdiag.AppendFromErr(diags, err)
	}

	return diags
}

func resourcePolicyDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	log.Printf("[INFO] Deleting IAM Policy: %s", d.Id())
	input := iam.DeletePolicyInput{
		PolicyArn: aws.String(d.Id()),
	}
	_, err := conn.DeletePolicy(ctx, &input)

	if errs.IsA[*awstypes.NoSuchEntityException](err) || tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "deleting IAM Policy (%s): %s", d.Id(), err)
	}

	return diags
}

func resourcePolicyFlatten(policy *awstypes.Policy, d *schema.ResourceData) {
	for k, v := range flattenPolicy(policy) {
		d.Set(k, v)
	}
}

func flattenPolicy(policy *awstypes.Policy) map[string]any {
	if policy == nil {
		return nil
	}

	return map[string]any{
		names.AttrARN:         aws.ToString(policy.Arn),
		"attachment_count":    int(aws.ToInt32(policy.AttachmentCount)),
		names.AttrDescription: aws.ToString(policy.Description),
		names.AttrName:        aws.ToString(policy.PolicyName),
		"policy_id":           aws.ToString(policy.PolicyId),
	}
}

func resourcePolicyFlattenPolicyDocument(policyDocument string, d *schema.ResourceData) error {
	policyDocument, err := url.QueryUnescape(policyDocument)
	if err != nil {
		return fmt.Errorf("parsing IAM Policy (%s) document: %w", d.Id(), err)
	}

	policyToSet, err := verify.PolicyToSet(d.Get(names.AttrPolicy).(string), policyDocument)
	if err != nil {
		return fmt.Errorf("parsing IAM Policy (%s) document: %w", d.Id(), err)
	}

	d.Set(names.AttrPolicy, policyToSet)

	return nil
}

func getPolicyByARN(ctx context.Context, conn *iam.Client, arn string) (*iam.GetPolicyOutput, error) {
	input := iam.GetPolicyInput{
		PolicyArn: aws.String(arn),
	}

	return getPolicy(ctx, conn, &input)
}

func findPolicyByARN(ctx context.Context, conn *iam.Client, arn string) (*awstypes.Policy, error) {
	policy, _, err := findPolicyWithDocumentByARN(ctx, conn, arn)
	return policy, err
}

// findPolicyWithDocumentByARN returns the managed policy and its default document
// (OOS GetPolicy embeds the document in response metadata).
func findPolicyWithDocumentByARN(ctx context.Context, conn *iam.Client, arn string) (*awstypes.Policy, string, error) {
	policy, policyDocument, _, err := findPolicyDetailsByARN(ctx, conn, arn)
	return policy, policyDocument, err
}

// findPolicyDetailsByARN returns policy, default document, and scope from GetPolicy
// (OOS embeds document and scope in response metadata).
func findPolicyDetailsByARN(ctx context.Context, conn *iam.Client, arn string) (*awstypes.Policy, string, string, error) {
	output, err := getPolicyByARN(ctx, conn, arn)
	if err != nil {
		return nil, "", "", err
	}

	policyDocument, _ := output.ResultMetadata.Get(oosIAMPolicyDocumentMetadataKey).(string)
	scope, _ := output.ResultMetadata.Get(oosIAMPolicyScopeMetadataKey).(string)
	return output.Policy, policyDocument, scope, nil
}

func getPolicy(ctx context.Context, conn *iam.Client, input *iam.GetPolicyInput) (*iam.GetPolicyOutput, error) {
	output, err := conn.GetPolicy(ctx, input, withOOSIAMGetPolicyResponse())

	if errs.IsA[*awstypes.NoSuchEntityException](err) || tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
		return nil, &retry.NotFoundError{
			LastError: err,
		}
	}

	if err != nil {
		return nil, err
	}

	if output == nil || output.Policy == nil {
		return nil, tfresource.NewEmptyResultError()
	}

	return output, nil
}
