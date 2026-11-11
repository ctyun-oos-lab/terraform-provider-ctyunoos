// Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package iam

import (
	"context"
	"encoding/xml"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/sdkdiag"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/retry"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/tfresource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

const accountLoginSecurityPolicyID = "iam-account-login-security-policy"

// @SDKResource("ctyun_iam_account_login_security_policy", name="Account Login Security Policy")
func resourceAccountLoginSecurityPolicy() *schema.Resource {
	return &schema.Resource{
		CreateWithoutTimeout: resourceAccountLoginSecurityPolicyUpdate,
		ReadWithoutTimeout:   resourceAccountLoginSecurityPolicyRead,
		UpdateWithoutTimeout: resourceAccountLoginSecurityPolicyUpdate,
		DeleteWithoutTimeout: resourceAccountLoginSecurityPolicyDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"allow_single_users_simultaneous_login": {
				Type:     schema.TypeBool,
				Optional: true,
				Computed: true,
			},
			"lockout_duration": {
				Type:         schema.TypeInt,
				Optional:     true,
				Computed:     true,
				ValidateFunc: validation.IntBetween(15, 60),
			},
			"login_failed_times": {
				Type:         schema.TypeInt,
				Optional:     true,
				Computed:     true,
				ValidateFunc: validation.IntBetween(5, 10),
			},
			"period_with_login_failures": {
				Type:         schema.TypeInt,
				Optional:     true,
				Computed:     true,
				ValidateFunc: validation.IntBetween(15, 60),
			},
			"session_duration": {
				Type:         schema.TypeInt,
				Optional:     true,
				Computed:     true,
				ValidateFunc: validation.IntBetween(10, 30),
			},
		},
	}
}

func resourceAccountLoginSecurityPolicyUpdate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics

	if err := updateAccountLoginSecurityPolicy(ctx, meta.(*conns.AWSClient).IAMClient(ctx), accountLoginSecurityPolicyUpdateValues(d)); err != nil {
		return sdkdiag.AppendErrorf(diags, "updating IAM Account Login Security Policy: %s", err)
	}

	if d.IsNewResource() {
		d.SetId(accountLoginSecurityPolicyID)
	}

	return append(diags, resourceAccountLoginSecurityPolicyRead(ctx, d, meta)...)
}

func resourceAccountLoginSecurityPolicyRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics

	policy, err := findAccountLoginSecurityPolicy(ctx, meta.(*conns.AWSClient).IAMClient(ctx))
	if !d.IsNewResource() && retry.NotFound(err) {
		log.Printf("[WARN] IAM Account Login Security Policy (%s) not found, removing from state", d.Id())
		d.SetId("")
		return diags
	}
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading IAM Account Login Security Policy (%s): %s", d.Id(), err)
	}

	d.Set("allow_single_users_simultaneous_login", policy.AllowSingleUsersSimultaneousLogin)
	d.Set("lockout_duration", policy.LockoutDuration)
	d.Set("login_failed_times", policy.LoginFailedTimes)
	d.Set("period_with_login_failures", policy.PeriodWithLoginFailures)
	d.Set("session_duration", policy.SessionDuration)

	return diags
}

func resourceAccountLoginSecurityPolicyDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics

	log.Printf("[DEBUG] Deleting IAM Account Login Security Policy: %s", d.Id())
	err := deleteAccountLoginSecurityPolicy(ctx, meta.(*conns.AWSClient).IAMClient(ctx))
	if retry.NotFound(err) {
		return diags
	}
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "deleting IAM Account Login Security Policy (%s): %s", d.Id(), err)
	}

	return diags
}

type accountLoginSecurityPolicyInput struct {
	AllowSingleUsersSimultaneousLogin bool
	LockoutDuration                   int
	LoginFailedTimes                  int
	PeriodWithLoginFailures           int
	SessionDuration                   int
}

func accountLoginSecurityPolicyUpdateValues(d *schema.ResourceData) url.Values {
	values := url.Values{}

	if value, ok := d.GetOkExists("allow_single_users_simultaneous_login"); ok {
		values.Set("AllowSingleUsersSimultaneousLogin", strconvBool(value.(bool)))
	}
	if value, ok := d.GetOkExists("lockout_duration"); ok {
		values.Set("LockoutDuration", strconv.Itoa(value.(int)))
	}
	if value, ok := d.GetOkExists("login_failed_times"); ok {
		values.Set("LoginFailedTimes", strconv.Itoa(value.(int)))
	}
	if value, ok := d.GetOkExists("period_with_login_failures"); ok {
		values.Set("PeriodWithLoginFailures", strconv.Itoa(value.(int)))
	}
	if value, ok := d.GetOkExists("session_duration"); ok {
		values.Set("SessionDuration", strconv.Itoa(value.(int)))
	}

	return values
}

func updateAccountLoginSecurityPolicy(ctx context.Context, conn *iam.Client, values url.Values) error {
	_, err := oosIAMQuery(ctx, conn, "UpdateAccountLoginSecurityPolicy", values)

	return err
}

func findAccountLoginSecurityPolicy(ctx context.Context, conn *iam.Client) (*accountLoginSecurityPolicyInput, error) {
	b, err := oosIAMQuery(ctx, conn, "GetAccountLoginSecurityPolicy", nil)
	if err != nil {
		return nil, err
	}

	var parsed getAccountLoginSecurityPolicyResponse
	if err := xml.Unmarshal(b, &parsed); err != nil {
		return nil, err
	}

	policy := parsed.GetAccountLoginSecurityPolicyResult.LoginSecurityPolicy
	if policy == nil {
		return nil, tfresource.NewEmptyResultError()
	}

	return policy, nil
}

func deleteAccountLoginSecurityPolicy(ctx context.Context, conn *iam.Client) error {
	_, err := oosIAMQuery(ctx, conn, "DeleteAccountLoginSecurityPolicy", nil)

	return err
}

type getAccountLoginSecurityPolicyResponse struct {
	GetAccountLoginSecurityPolicyResult getAccountLoginSecurityPolicyResult `xml:"GetAccountLoginSecurityPolicyResult"`
}

type getAccountLoginSecurityPolicyResult struct {
	LoginSecurityPolicy *accountLoginSecurityPolicyInput `xml:"LoginSecurityPolicy"`
}

func (p *accountLoginSecurityPolicyInput) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	type rawPolicy struct {
		AllowSingleUsersSimultaneousLogin string `xml:"AllowSingleUsersSimultaneousLogin"`
		LockoutDuration                   int    `xml:"LockoutDuration"`
		LoginFailedTimes                  int    `xml:"LoginFailedTimes"`
		PeriodWithLoginFailures           int    `xml:"PeriodWithLoginFailures"`
		SessionDuration                   int    `xml:"SessionDuration"`
	}

	var raw rawPolicy
	if err := d.DecodeElement(&raw, &start); err != nil {
		return err
	}

	p.AllowSingleUsersSimultaneousLogin = strings.EqualFold(raw.AllowSingleUsersSimultaneousLogin, "true")
	p.LockoutDuration = raw.LockoutDuration
	p.LoginFailedTimes = raw.LoginFailedTimes
	p.PeriodWithLoginFailures = raw.PeriodWithLoginFailures
	p.SessionDuration = raw.SessionDuration

	return nil
}

func oosIAMQuery(ctx context.Context, conn *iam.Client, action string, values url.Values) ([]byte, error) {
	if values == nil {
		values = url.Values{}
	}

	var b []byte
	optFns := []func(*iam.Options){withOOSIAMQueryBody(action, values)}

	switch action {
	case "GetAccountLoginSecurityPolicy":
		optFns = append(optFns, withOOSIAMQueryResponse(oosIAMQueryOutput(action), &b))
		_, err := conn.GetAccountPasswordPolicy(ctx, &iam.GetAccountPasswordPolicyInput{}, optFns...)
		if err != nil {
			return nil, err
		}
	case "UpdateAccountLoginSecurityPolicy":
		_, err := conn.UpdateAccountPasswordPolicy(ctx, &iam.UpdateAccountPasswordPolicyInput{}, optFns...)
		if err != nil {
			return nil, err
		}
	case "DeleteAccountLoginSecurityPolicy":
		_, err := conn.DeleteAccountPasswordPolicy(ctx, &iam.DeleteAccountPasswordPolicyInput{}, optFns...)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported OOS IAM action %q", action)
	}

	return b, nil
}

func oosIAMQueryOutput(action string) any {
	switch action {
	case "GetAccountLoginSecurityPolicy":
		return &iam.GetAccountPasswordPolicyOutput{}
	case "UpdateAccountLoginSecurityPolicy":
		return &iam.UpdateAccountPasswordPolicyOutput{}
	case "DeleteAccountLoginSecurityPolicy":
		return &iam.DeleteAccountPasswordPolicyOutput{}
	default:
		return nil
	}
}

func strconvBool(v bool) string {
	if v {
		return "true"
	}

	return "false"
}
