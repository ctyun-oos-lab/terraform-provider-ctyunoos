// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package iam

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"strconv"

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

// @SDKResource("ctyun_iam_user_login_profile", name="User Login Profile")
func resourceUserLoginProfile() *schema.Resource {
	return &schema.Resource{
		CreateWithoutTimeout: resourceUserLoginProfileCreate,
		ReadWithoutTimeout:   resourceUserLoginProfileRead,
		DeleteWithoutTimeout: resourceUserLoginProfileDelete,

		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta any) ([]*schema.ResourceData, error) {
				return []*schema.ResourceData{d}, nil
			},
		},

		Schema: map[string]*schema.Schema{
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
			"password_reset_required": {
				Type:     schema.TypeBool,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},
			names.AttrPassword: {
				Type:      schema.TypeString,
				Computed:  true,
				Sensitive: true,
			},
		},
	}
}

const (
	charLower   = "abcdefghijklmnopqrstuvwxyz"
	charUpper   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	charNumbers = "0123456789"
	charSymbols = "!@#$%^&*()_+-=[]{}|'"
)

func defaultAccountPasswordPolicy() *awstypes.PasswordPolicy {
	return &awstypes.PasswordPolicy{
		AllowUsersToChangePassword: true,
		HardExpiry:                 aws.Bool(false),
		MaxPasswordAge:             aws.Int32(0),
		MinimumPasswordLength:      aws.Int32(8),
		PasswordReusePrevention:    aws.Int32(0),
		RequireLowercaseCharacters: true,
		RequireNumbers:             true,
		RequireSymbols:             false,
		RequireUppercaseCharacters: false,
		ExpirePasswords:            false,
	}
}

func accountPasswordPolicyOrDefault(policy *awstypes.PasswordPolicy) *awstypes.PasswordPolicy {
	if isEmptyAccountPasswordPolicy(policy) {
		return defaultAccountPasswordPolicy()
	}

	return policy
}

func isEmptyAccountPasswordPolicy(policy *awstypes.PasswordPolicy) bool {
	return policy == nil ||
		!policy.AllowUsersToChangePassword &&
			!policy.ExpirePasswords &&
			policy.HardExpiry == nil &&
			policy.MaxPasswordAge == nil &&
			policy.MinimumPasswordLength == nil &&
			policy.PasswordReusePrevention == nil &&
			!policy.RequireLowercaseCharacters &&
			!policy.RequireNumbers &&
			!policy.RequireSymbols &&
			!policy.RequireUppercaseCharacters
}

// generatePassword generates a random password matching the OOS default policy.
func generatePassword(length int) (string, error) {
	return generatePasswordWithLengthAndPolicy(length, nil)
}

func generatePasswordWithPolicy(policy *awstypes.PasswordPolicy) (string, error) {
	policy = accountPasswordPolicyOrDefault(policy)
	length := int(aws.ToInt32(policy.MinimumPasswordLength))
	if length <= 0 {
		length = int(aws.ToInt32(defaultAccountPasswordPolicy().MinimumPasswordLength))
	}

	return generatePasswordWithLengthAndPolicy(length, policy)
}

func generatePasswordWithLengthAndPolicy(length int, policy *awstypes.PasswordPolicy) (string, error) {
	if policy != nil && aws.ToInt32(policy.MinimumPasswordLength) > int32(length) {
		length = int(aws.ToInt32(policy.MinimumPasswordLength))
	}

	requiredCharsets, charset := passwordPolicyCharsets(policy)
	if length < len(requiredCharsets) {
		length = len(requiredCharsets)
	}

	result := make([]byte, length)
	charsetSize := big.NewInt(int64(len(charset)))

	for range 100000 {
		for i, requiredCharset := range requiredCharsets {
			c, err := randomCharacter(requiredCharset)
			if err != nil {
				return "", err
			}
			result[i] = c
		}

		for i := range result {
			if i < len(requiredCharsets) {
				continue
			}

			r, err := rand.Int(rand.Reader, charsetSize)
			if err != nil {
				return "", err
			}
			if !r.IsInt64() {
				return "", errors.New("rand.Int() not representable as an Int64")
			}

			result[i] = charset[r.Int64()]
		}

		if !checkPwdPolicyWithPolicy(result, policy) {
			continue
		}

		return string(result), nil
	}

	return "", errors.New("failed to generate acceptable password")
}

func passwordPolicyCharsets(policy *awstypes.PasswordPolicy) ([]string, string) {
	if policy == nil {
		return []string{charLower, charNumbers}, charLower + charNumbers
	}

	var requiredCharsets []string
	var charset string
	if policy.RequireLowercaseCharacters {
		requiredCharsets = append(requiredCharsets, charLower)
		charset += charLower
	}
	if policy.RequireUppercaseCharacters {
		requiredCharsets = append(requiredCharsets, charUpper)
		charset += charUpper
	}
	if policy.RequireNumbers {
		requiredCharsets = append(requiredCharsets, charNumbers)
		charset += charNumbers
	}
	if policy.RequireSymbols {
		requiredCharsets = append(requiredCharsets, charSymbols)
		charset += charSymbols
	}
	if charset == "" {
		charset = charLower + charNumbers
	}

	return requiredCharsets, charset
}

func randomCharacter(charset string) (byte, error) {
	r, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
	if err != nil {
		return 0, err
	}
	if !r.IsInt64() {
		return 0, errors.New("rand.Int() not representable as an Int64")
	}

	return charset[r.Int64()], nil
}

// Check the generated password contains lowercase letters and numbers only.
func checkPwdPolicy(pass []byte) bool {
	return checkPwdPolicyWithPolicy(pass, nil)
}

func checkPwdPolicyWithPolicy(pass []byte, policy *awstypes.PasswordPolicy) bool {
	if policy == nil {
		return bytes.ContainsAny(pass, charLower) &&
			bytes.ContainsAny(pass, charNumbers) &&
			!bytes.ContainsAny(pass, charUpper+charSymbols)
	}

	if policy.MinimumPasswordLength != nil && len(pass) < int(aws.ToInt32(policy.MinimumPasswordLength)) {
		return false
	}
	if policy.RequireLowercaseCharacters && !bytes.ContainsAny(pass, charLower) {
		return false
	}
	if policy.RequireUppercaseCharacters && !bytes.ContainsAny(pass, charUpper) {
		return false
	}
	if policy.RequireNumbers && !bytes.ContainsAny(pass, charNumbers) {
		return false
	}
	if policy.RequireSymbols && !bytes.ContainsAny(pass, charSymbols) {
		return false
	}

	return true
}

func resourceUserLoginProfileCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)
	username := d.Get("user").(string)

	policy, err := findAccountPasswordPolicy(ctx, conn)
	if retry.NotFound(err) {
		policy = defaultAccountPasswordPolicy()
	} else if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading IAM Account Password Policy for %q: %s", username, err)
	}

	initialPassword, err := generatePasswordWithPolicy(policy)
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "creating IAM User Login Profile for %q: %s", username, err)
	}

	createResp, err := conn.CreateLoginProfile(ctx, &iam.CreateLoginProfileInput{}, withOOSIAMQueryBody("CreateLoginProfile", userLoginProfileCreateValues(d, initialPassword)))
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "creating IAM User Login Profile for %q: %s", username, err)
	}

	d.SetId(aws.ToString(createResp.LoginProfile.UserName))

	d.Set(names.AttrPassword, initialPassword)

	return append(diags, resourceUserLoginProfileRead(ctx, d, meta)...)
}

func userLoginProfileCreateValues(d *schema.ResourceData, password string) url.Values {
	values := url.Values{}
	values.Set("UserName", d.Get("user").(string))
	values.Set("Password", password)
	values.Set("PasswordResetRequired", strconv.FormatBool(d.Get("password_reset_required").(bool)))

	return values
}

func resourceUserLoginProfileRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	input := &iam.GetLoginProfileInput{
		UserName: aws.String(d.Id()),
	}

	var output *iam.GetLoginProfileOutput

	err := tfresource.Retry(ctx, propagationTimeout, func(ctx context.Context) *tfresource.RetryError {
		var err error

		output, err = conn.GetLoginProfile(ctx, input)

		if d.IsNewResource() && (errs.IsA[*awstypes.NoSuchEntityException](err) || tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound)) {
			return tfresource.RetryableError(err)
		}

		if err != nil {
			return tfresource.NonRetryableError(err)
		}

		return nil
	})

	if !d.IsNewResource() && (errs.IsA[*awstypes.NoSuchEntityException](err) || tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound)) {
		log.Printf("[WARN] IAM User Login Profile (%s) not found, removing from state", d.Id())
		d.SetId("")
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading IAM User Login Profile (%s): %s", d.Id(), err)
	}

	if output == nil || output.LoginProfile == nil {
		return sdkdiag.AppendErrorf(diags, "reading IAM User Login Profile (%s): empty response", d.Id())
	}

	d.Set("user", output.LoginProfile.UserName)
	d.Set("password_reset_required", output.LoginProfile.PasswordResetRequired)
	return diags
}

func resourceUserLoginProfileDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	input := &iam.DeleteLoginProfileInput{
		UserName: aws.String(d.Id()),
	}

	log.Printf("[DEBUG] Deleting IAM User Login Profile (%s): %v", d.Id(), input)
	// Handle IAM eventual consistency
	err := tfresource.Retry(ctx, propagationTimeout, func(ctx context.Context) *tfresource.RetryError {
		_, err := conn.DeleteLoginProfile(ctx, input)

		var nse *awstypes.NoSuchEntityException
		if tfawserr.ErrCodeEquals(err, nse.ErrorCode()) || tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
			return nil
		}

		// EntityTemporarilyUnmodifiable: Login Profile for User XXX cannot be modified while login profile is being created.
		var etu *awstypes.EntityTemporarilyUnmodifiableException
		if tfawserr.ErrCodeEquals(err, etu.ErrorCode()) {
			return tfresource.RetryableError(err)
		}

		if err != nil {
			return tfresource.NonRetryableError(err)
		}

		return nil
	})

	if errs.IsA[*awstypes.NoSuchEntityException](err) || tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "deleting IAM User Login Profile (%s): %s", d.Id(), err)
	}

	return diags
}
