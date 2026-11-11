// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package iam

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	awstypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/sdkdiag"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/retry"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/tfresource"
	"github.com/hashicorp/aws-sdk-go-base/v2/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// @SDKResource("ctyun_iam_account_password_policy", name="Account Password Policy")
func resourceAccountPasswordPolicy() *schema.Resource {
	return &schema.Resource{
		CreateWithoutTimeout: resourceAccountPasswordPolicyUpdate,
		ReadWithoutTimeout:   resourceAccountPasswordPolicyRead,
		UpdateWithoutTimeout: resourceAccountPasswordPolicyUpdate,
		DeleteWithoutTimeout: resourceAccountPasswordPolicyDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"allow_users_to_change_password": {
				Type:     schema.TypeBool,
				Optional: true,
				Computed: true,
			},
			"expire_passwords": {
				Type:     schema.TypeBool,
				Computed: true,
			},
			"hard_expiry": {
				Type:     schema.TypeBool,
				Optional: true,
				Computed: true,
			},
			"max_password_age": {
				Type:         schema.TypeInt,
				Optional:     true,
				Computed:     true,
				ValidateFunc: validation.IntBetween(0, 1095),
			},
			"minimum_password_length": {
				Type:         schema.TypeInt,
				Optional:     true,
				Computed:     true,
				ValidateFunc: validation.IntBetween(8, 128),
			},
			"password_reuse_prevention": {
				Type:         schema.TypeInt,
				Optional:     true,
				Computed:     true,
				ValidateFunc: validation.IntBetween(0, 24),
			},
			"require_lowercase_characters": {
				Type:     schema.TypeBool,
				Optional: true,
				Computed: true,
			},
			"require_numbers": {
				Type:     schema.TypeBool,
				Optional: true,
				Computed: true,
			},
			"require_symbols": {
				Type:     schema.TypeBool,
				Optional: true,
				Computed: true,
			},
			"require_uppercase_characters": {
				Type:     schema.TypeBool,
				Optional: true,
				Computed: true,
			},
		},
	}
}

func resourceAccountPasswordPolicyUpdate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	_, err := conn.UpdateAccountPasswordPolicy(ctx, &iam.UpdateAccountPasswordPolicyInput{}, withOOSIAMQueryBody("UpdateAccountPasswordPolicy", accountPasswordPolicyUpdateValues(d)))

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "updating IAM Account Password Policy: %s", err)
	}

	if d.IsNewResource() {
		d.SetId("iam-account-password-policy")
	}

	return append(diags, resourceAccountPasswordPolicyRead(ctx, d, meta)...)
}

func accountPasswordPolicyUpdateValues(d *schema.ResourceData) url.Values {
	values := url.Values{}

	if value, ok := d.GetOkExists("allow_users_to_change_password"); ok {
		values.Set("AllowUsersToChangePassword", strconv.FormatBool(value.(bool)))
	}
	if value, ok := d.GetOkExists("hard_expiry"); ok {
		values.Set("HardExpiry", strconv.FormatBool(value.(bool)))
	}
	if value, ok := d.GetOkExists("max_password_age"); ok {
		values.Set("MaxPasswordAge", strconv.Itoa(value.(int)))
	}
	if value, ok := d.GetOkExists("minimum_password_length"); ok {
		values.Set("MinimumPasswordLength", strconv.Itoa(value.(int)))
	}
	if value, ok := d.GetOkExists("password_reuse_prevention"); ok {
		values.Set("PasswordReusePrevention", strconv.Itoa(value.(int)))
	}
	if value, ok := d.GetOkExists("require_lowercase_characters"); ok {
		values.Set("RequireLowercaseCharacters", strconv.FormatBool(value.(bool)))
	}
	if value, ok := d.GetOkExists("require_numbers"); ok {
		values.Set("RequireNumbers", strconv.FormatBool(value.(bool)))
	}
	if value, ok := d.GetOkExists("require_symbols"); ok {
		values.Set("RequireSymbols", strconv.FormatBool(value.(bool)))
	}
	if value, ok := d.GetOkExists("require_uppercase_characters"); ok {
		values.Set("RequireUppercaseCharacters", strconv.FormatBool(value.(bool)))
	}

	return values
}

func resourceAccountPasswordPolicyRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	policy, err := findAccountPasswordPolicy(ctx, conn)

	if !d.IsNewResource() && retry.NotFound(err) {
		log.Printf("[WARN] IAM Account Password Policy (%s) not found, removing from state", d.Id())
		d.SetId("")
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading IAM Account Password Policy (%s): %s", d.Id(), err)
	}

	d.Set("allow_users_to_change_password", policy.AllowUsersToChangePassword)
	d.Set("expire_passwords", policy.ExpirePasswords)
	d.Set("hard_expiry", policy.HardExpiry)
	d.Set("max_password_age", policy.MaxPasswordAge)
	d.Set("minimum_password_length", policy.MinimumPasswordLength)
	d.Set("password_reuse_prevention", policy.PasswordReusePrevention)
	d.Set("require_lowercase_characters", policy.RequireLowercaseCharacters)
	d.Set("require_numbers", policy.RequireNumbers)
	d.Set("require_symbols", policy.RequireSymbols)
	d.Set("require_uppercase_characters", policy.RequireUppercaseCharacters)

	return diags
}

func resourceAccountPasswordPolicyDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	log.Printf("[DEBUG] Deleting IAM Account Password Policy: %s", d.Id())
	_, err := conn.DeleteAccountPasswordPolicy(ctx, &iam.DeleteAccountPasswordPolicyInput{})

	if errs.IsA[*awstypes.NoSuchEntityException](err) || tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "deleting IAM Account Password Policy (%s): %s", d.Id(), err)
	}

	return diags
}

func findAccountPasswordPolicy(ctx context.Context, conn *iam.Client) (*awstypes.PasswordPolicy, error) {
	input := &iam.GetAccountPasswordPolicyInput{}

	output, err := conn.GetAccountPasswordPolicy(ctx, input)

	if errs.IsA[*awstypes.NoSuchEntityException](err) || tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
		return nil, &retry.NotFoundError{
			LastError: err,
		}
	}

	if err != nil {
		return nil, err
	}

	if output == nil || output.PasswordPolicy == nil {
		return nil, tfresource.NewEmptyResultError()
	}

	return output.PasswordPolicy, nil
}
