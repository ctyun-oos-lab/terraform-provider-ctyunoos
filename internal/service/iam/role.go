// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package iam

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	awstypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/create"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/sdkdiag"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/provider/sdkv2/importer"
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
	roleNameMaxLen       = 64
	roleNamePrefixMaxLen = roleNameMaxLen - sdkid.UniqueIDSuffixLength
)

// @SDKResource("ctyun_iam_role", name="Role")
// @IdentityAttribute("name")
// @CustomImport
// @Testing(existsType="github.com/aws/aws-sdk-go-v2/service/iam/types;types.Role")
// @Testing(hasNoPreExistingResource=true)
// @Testing(idAttrDuplicates="name")
func resourceRole() *schema.Resource {
	return &schema.Resource{
		CreateWithoutTimeout: resourceRoleCreate,
		ReadWithoutTimeout:   resourceRoleRead,
		UpdateWithoutTimeout: resourceRoleUpdate,
		DeleteWithoutTimeout: resourceRoleDelete,

		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, rd *schema.ResourceData, meta any) ([]*schema.ResourceData, error) {
				if err := importer.Import(ctx, rd, meta); err != nil {
					return nil, err
				}
				return []*schema.ResourceData{rd}, nil
			},
		},

		Schema: map[string]*schema.Schema{
			names.AttrARN: {
				Type:     schema.TypeString,
				Computed: true,
			},
			"assume_role_policy": {
				Type:     schema.TypeString,
				Required: true,
				ValidateFunc: validation.All(
					validation.StringLenBetween(1, 131072),
					validRolePolicyDocumentCharacters,
					verify.ValidIAMPolicyJSON,
				),
				DiffSuppressFunc:      verify.SuppressEquivalentPolicyDiffs,
				DiffSuppressOnRefresh: true,
				StateFunc: func(v any) string {
					json, _ := structure.NormalizeJsonString(v)
					return json
				},
			},
			"create_date": {
				Type:     schema.TypeString,
				Computed: true,
			},
			names.AttrDescription: {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringLenBetween(0, 1000),
			},
			"max_session_duration": {
				Type:         schema.TypeInt,
				Optional:     true,
				Computed:     true,
				ValidateFunc: validation.IntBetween(3600, 43200),
			},
			names.AttrName: {
				Type:          schema.TypeString,
				Optional:      true,
				Computed:      true,
				ForceNew:      true,
				ConflictsWith: []string{names.AttrNamePrefix},
				ValidateFunc:  validResourceName(roleNameMaxLen),
			},
			names.AttrNamePrefix: {
				Type:          schema.TypeString,
				Optional:      true,
				Computed:      true,
				ForceNew:      true,
				ConflictsWith: []string{names.AttrName},
				ValidateFunc:  validResourceName(roleNamePrefixMaxLen),
			},
			"unique_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func resourceRoleCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	assumeRolePolicy, err := structure.NormalizeJsonString(d.Get("assume_role_policy").(string))
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "assume_role_policy (%s) is invalid JSON: %s", assumeRolePolicy, err)
	}

	name := create.Name(ctx, d.Get(names.AttrName).(string), d.Get(names.AttrNamePrefix).(string))
	input := iam.CreateRoleInput{
		AssumeRolePolicyDocument: aws.String(assumeRolePolicy),
		RoleName:                 aws.String(name),
	}

	if v, ok := d.GetOk(names.AttrDescription); ok {
		input.Description = aws.String(v.(string))
	}

	if v, ok := d.GetOk("max_session_duration"); ok {
		input.MaxSessionDuration = aws.Int32(int32(v.(int)))
	}

	output, err := retryCreateRole(ctx, conn, &input)

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "creating IAM Role (%s): %s", name, err)
	}

	roleName := aws.ToString(output.Role.RoleName)

	d.SetId(roleName)

	return append(diags, resourceRoleRead(ctx, d, meta)...)
}

func resourceRoleRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	role, err := tfresource.RetryWhenNewResourceNotFound(ctx, propagationTimeout, func(ctx context.Context) (*awstypes.Role, error) {
		return findRoleByName(ctx, conn, d.Id())
	}, d.IsNewResource())

	if !d.IsNewResource() && retry.NotFound(err) {
		log.Printf("[WARN] IAM Role (%s) not found, removing from state", d.Id())
		d.SetId("")
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading IAM Role (%s): %s", d.Id(), err)
	}

	// occasionally, immediately after a role is created, AWS will give an ARN like AROAQ7SSZBKHREXAMPLE (unique ID)
	if role, err = waitRoleARNIsNotUniqueID(ctx, conn, d.Id(), role); err != nil {
		return sdkdiag.AppendErrorf(diags, "reading IAM Role (%s): waiting for valid ARN: %s", d.Id(), err)
	}

	diags = append(diags, resourceRoleFlatten(role, d)...)
	if diags.HasError() {
		return diags
	}

	return diags
}

func isOOSIAMUpdateRoleEmptyResult(err error) bool {
	if err == nil {
		return false
	}

	message := err.Error()
	return strings.Contains(message, "StatusCode: 200") &&
		strings.Contains(message, "UpdateRoleResult node not found")
}

func resourceRoleUpdate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	if d.HasChange("assume_role_policy") {
		assumeRolePolicy, err := structure.NormalizeJsonString(d.Get("assume_role_policy").(string))
		if err != nil {
			return sdkdiag.AppendErrorf(diags, "assume_role_policy (%s) is invalid JSON: %s", assumeRolePolicy, err)
		}

		input := iam.UpdateAssumeRolePolicyInput{
			RoleName:       aws.String(d.Id()),
			PolicyDocument: aws.String(assumeRolePolicy),
		}

		_, err = tfresource.RetryWhen(ctx, propagationTimeout,
			func(ctx context.Context) (any, error) {
				return conn.UpdateAssumeRolePolicy(ctx, &input)
			},
			func(err error) (bool, error) {
				if errs.IsAErrorMessageContains[*awstypes.MalformedPolicyDocumentException](err, "Invalid principal in policy") {
					return true, err
				}

				return false, err
			},
		)

		if err != nil {
			return sdkdiag.AppendErrorf(diags, "updating IAM Role (%s) assume role policy: %s", d.Id(), err)
		}
	}

	if d.HasChange(names.AttrDescription) || d.HasChange("max_session_duration") {
		input := iam.UpdateRoleInput{
			RoleName: aws.String(d.Id()),
		}

		if d.HasChange(names.AttrDescription) {
			input.Description = aws.String(d.Get(names.AttrDescription).(string))
		}

		if d.HasChange("max_session_duration") {
			input.MaxSessionDuration = aws.Int32(int32(d.Get("max_session_duration").(int)))
		}

		_, err := conn.UpdateRole(ctx, &input)

		if err != nil && !isOOSIAMUpdateRoleEmptyResult(err) {
			return sdkdiag.AppendErrorf(diags, "updating IAM Role (%s): %s", d.Id(), err)
		}
	}

	return append(diags, resourceRoleRead(ctx, d, meta)...)
}

func resourceRoleDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	err := deleteRole(ctx, conn, d.Id())

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "deleting IAM Role (%s): %s", d.Id(), err)
	}

	return diags
}

func deleteRole(ctx context.Context, conn *iam.Client, roleName string) error {
	policyARNs, err := findRoleAttachedPolicies(ctx, conn, roleName)
	switch {
	case retry.NotFound(err):
	case err != nil:
		return fmt.Errorf("reading IAM Policies attached to Role (%s): %w", roleName, err)
	default:
		if err := deleteRolePolicyAttachments(ctx, conn, roleName, policyARNs); err != nil {
			return err
		}
	}

	input := iam.DeleteRoleInput{
		RoleName: aws.String(roleName),
	}

	_, err = conn.DeleteRole(ctx, &input)

	if errs.IsA[*awstypes.NoSuchEntityException](err) || tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
		return nil
	}

	return err
}

func retryCreateRole(ctx context.Context, conn *iam.Client, input *iam.CreateRoleInput) (*iam.CreateRoleOutput, error) {
	outputRaw, err := tfresource.RetryWhen(ctx, propagationTimeout,
		func(ctx context.Context) (any, error) {
			return conn.CreateRole(ctx, input)
		},
		func(err error) (bool, error) {
			if errs.IsAErrorMessageContains[*awstypes.MalformedPolicyDocumentException](err, "Invalid principal in policy") {
				return true, err
			}

			return false, err
		},
	)

	if err != nil {
		return nil, err
	}

	output, ok := outputRaw.(*iam.CreateRoleOutput)
	if !ok || output == nil || aws.ToString(output.Role.RoleName) == "" {
		return nil, fmt.Errorf("create IAM role (%s) returned an empty result", aws.ToString(input.RoleName))
	}

	return output, err
}

func findRoleByName(ctx context.Context, conn *iam.Client, name string) (*awstypes.Role, error) {
	input := iam.GetRoleInput{
		RoleName: aws.String(name),
	}

	return findRole(ctx, conn, &input)
}

func findRole(ctx context.Context, conn *iam.Client, input *iam.GetRoleInput) (*awstypes.Role, error) {
	output, err := conn.GetRole(ctx, input)

	if errs.IsA[*awstypes.NoSuchEntityException](err) || tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
		return nil, &retry.NotFoundError{
			LastError: err,
		}
	}

	if err != nil {
		return nil, err
	}

	if output == nil || output.Role == nil {
		return nil, tfresource.NewEmptyResultError()
	}

	return output.Role, nil
}

const (
	roleARNIsUniqueIDState = "uniqueid"
	roleNotFoundState      = "notfound"
)

func statusRoleCreate(conn *iam.Client, id string) retry.StateRefreshFunc {
	return func(ctx context.Context) (any, string, error) {
		role, err := findRoleByName(ctx, conn, id)

		if retry.NotFound(err) {
			return nil, roleNotFoundState, nil
		}

		if err != nil {
			return nil, "", err
		}

		if arn.IsARN(aws.ToString(role.Arn)) {
			return role, names.AttrARN, nil
		}

		return role, roleARNIsUniqueIDState, nil
	}
}

func waitRoleARNIsNotUniqueID(ctx context.Context, conn *iam.Client, id string, role *awstypes.Role) (*awstypes.Role, error) {
	if arn.IsARN(aws.ToString(role.Arn)) {
		return role, nil
	}

	stateConf := &retry.StateChangeConf{
		Pending:                   []string{roleARNIsUniqueIDState, roleNotFoundState},
		Target:                    []string{names.AttrARN},
		Refresh:                   statusRoleCreate(conn, id),
		Timeout:                   propagationTimeout,
		NotFoundChecks:            10,
		ContinuousTargetOccurence: 5,
		Delay:                     10 * time.Second,
	}

	outputRaw, err := stateConf.WaitForStateContext(ctx)

	if output, ok := outputRaw.(*awstypes.Role); ok {
		return output, err
	}

	return nil, err
}

func resourceRoleFlatten(role *awstypes.Role, d *schema.ResourceData) diag.Diagnostics {
	var diags diag.Diagnostics

	d.Set(names.AttrARN, role.Arn)
	d.Set("create_date", policyTimeString(role.CreateDate))
	d.Set(names.AttrDescription, role.Description)
	d.Set("max_session_duration", role.MaxSessionDuration)
	d.Set(names.AttrName, role.RoleName)
	d.Set(names.AttrNamePrefix, create.NamePrefixFromName(aws.ToString(role.RoleName)))
	d.Set("unique_id", role.RoleId)

	assumeRolePolicy, err := url.QueryUnescape(aws.ToString(role.AssumeRolePolicyDocument))
	if err != nil {
		return sdkdiag.AppendFromErr(diags, err)
	}

	policyToSet, err := verify.PolicyToSet(d.Get("assume_role_policy").(string), assumeRolePolicy)
	if err != nil {
		return sdkdiag.AppendFromErr(diags, err)
	}

	d.Set("assume_role_policy", policyToSet)

	return diags
}

func findRoleAttachedPolicies(ctx context.Context, conn *iam.Client, roleName string) ([]string, error) {
	input := iam.ListAttachedRolePoliciesInput{
		RoleName: aws.String(roleName),
	}
	var output []string

	pages := iam.NewListAttachedRolePoliciesPaginator(conn, &input)
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

		for _, v := range page.AttachedPolicies {
			if aws.ToString(v.PolicyArn) != "" {
				output = append(output, aws.ToString(v.PolicyArn))
			}
		}
	}

	return output, nil
}

func deleteRolePolicyAttachments(ctx context.Context, conn *iam.Client, roleName string, policyARNs []string) error {
	var errsList []error

	for _, policyARN := range policyARNs {
		input := iam.DetachRolePolicyInput{
			PolicyArn: aws.String(policyARN),
			RoleName:  aws.String(roleName),
		}

		_, err := conn.DetachRolePolicy(ctx, &input)

		if errs.IsA[*awstypes.NoSuchEntityException](err) || tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
			continue
		}

		if err != nil {
			errsList = append(errsList, fmt.Errorf("detaching IAM Policy (%s) from Role (%s): %w", policyARN, roleName, err))
		}
	}

	return errors.Join(errsList...)
}
