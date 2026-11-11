// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package iam

import (
	"context"
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
	tftags "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/tags"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/tfresource"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/aws-sdk-go-base/v2/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// @SDKResource("ctyun_iam_user", name="User")
// @IdentityAttribute("name")
// @MutableIdentity
// @Tags(identifierAttribute="id", resourceType="User")
// @Testing(existsType="github.com/aws/aws-sdk-go-v2/service/iam/types;types.User")
// @Testing(importIgnore="force_destroy")
// @Testing(plannableImportAction="NoOp")
// @Testing(hasNoPreExistingResource=true)
func resourceUser() *schema.Resource {
	tagsSchema := *tftags.TagsSchema()
	tagsSchema.ValidateDiagFunc = validUserTags

	return &schema.Resource{
		CreateWithoutTimeout: resourceUserCreate,
		ReadWithoutTimeout:   resourceUserRead,
		UpdateWithoutTimeout: resourceUserUpdate,
		DeleteWithoutTimeout: resourceUserDelete,

		Schema: map[string]*schema.Schema{
			names.AttrARN: {
				Type:     schema.TypeString,
				Computed: true,
			},
			names.AttrForceDestroy: {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Delete user even if it has non-Terraform-managed IAM access keys, login profile or MFA devices",
			},
			names.AttrName: {
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
			names.AttrTags: &tagsSchema,
			/*
				The UniqueID could be used as the Id(), but none of the API
				calls allow specifying a user by the UniqueID: they require the
				name. The only way to locate a user by UniqueID is to list them
				all and that would make this provider unnecessarily complex
				and inefficient. Still, there are other reasons one might want
				the UniqueID, so we can make it available.
			*/
			"unique_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func resourceUserCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	name := d.Get(names.AttrName).(string)
	input := iam.CreateUserInput{
		Tags:     getTagsIn(ctx),
		UserName: aws.String(name),
	}

	output, err := conn.CreateUser(ctx, &input)

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "creating IAM User (%s): %s", name, err)
	}

	if output == nil || output.User == nil || aws.ToString(output.User.UserName) == "" {
		return sdkdiag.AppendErrorf(diags, "creating IAM User (%s): empty result", name)
	}

	d.SetId(aws.ToString(output.User.UserName))

	return append(diags, resourceUserRead(ctx, d, meta)...)
}

func resourceUserRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	user, err := tfresource.RetryWhenNewResourceNotFound(ctx, propagationTimeout, func(ctx context.Context) (*awstypes.User, error) {
		return findUserByName(ctx, conn, d.Id())
	}, d.IsNewResource())

	if !d.IsNewResource() && retry.NotFound(err) {
		log.Printf("[WARN] IAM User (%s) not found, removing from state", d.Id())
		d.SetId("")
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading IAM User (%s): %s", d.Id(), err)
	}

	resourceUserFlatten(user, d)
	setTagsOut(ctx, user.Tags)

	return diags
}

func resourceUserUpdate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	return append(diags, resourceUserRead(ctx, d, meta)...)
}

func resourceUserDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)
	forceDestroy := d.Get(names.AttrForceDestroy).(bool)
	if forceDestroy {
		// DeleteUser with force_destroy may remove access keys on the OOS server side, so
		// serialize it with Terraform-managed access key create/delete operations for
		// the same user.
		unlock := lockAccessKeys(d.Id())
		defer unlock()
	}

	log.Println("[DEBUG] Deleting IAM User:", d.Id())
	input := iam.DeleteUserInput{
		UserName: aws.String(d.Id()),
	}
	_, err := conn.DeleteUser(ctx, &input, withOOSIAMDeleteUserForceDestroy(forceDestroy))

	if err != nil && (errs.IsA[*awstypes.NoSuchEntityException](err) || tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound)) {
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "deleting IAM User (%s): %s", d.Id(), err)
	}

	return diags
}

func findUserByName(ctx context.Context, conn *iam.Client, name string) (*awstypes.User, error) {
	input := iam.GetUserInput{
		UserName: aws.String(name),
	}

	return findUser(ctx, conn, &input)
}

func findUser(ctx context.Context, conn *iam.Client, input *iam.GetUserInput) (*awstypes.User, error) {
	output, err := conn.GetUser(ctx, input)

	if err != nil && (errs.IsA[*awstypes.NoSuchEntityException](err) || tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound)) {
		return nil, &retry.NotFoundError{
			LastError: err,
		}
	}

	if err != nil {
		return nil, err
	}

	if output == nil || output.User == nil {
		return nil, tfresource.NewEmptyResultError()
	}

	return output.User, nil
}

func userTags(ctx context.Context, conn *iam.Client, identifier string, optFns ...func(*iam.Options)) ([]awstypes.Tag, error) {
	input := iam.ListUserTagsInput{
		UserName: aws.String(identifier),
	}
	var output []awstypes.Tag

	pages := iam.NewListUserTagsPaginator(conn, &input)
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx, optFns...)

		if err != nil {
			return nil, err
		}

		output = append(output, page.Tags...)
	}

	return output, nil
}

func resourceUserFlatten(user *awstypes.User, d *schema.ResourceData) {
	d.Set(names.AttrARN, user.Arn)
	d.Set(names.AttrName, user.UserName)
	d.Set("unique_id", user.UserId)
}
