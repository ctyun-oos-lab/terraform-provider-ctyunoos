// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package sdkv2

import (
	"fmt"
	"strings"
	"time"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/verify"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// validAssumeRoleDuration validates a string can be parsed as a valid time.Duration
// and is within a minimum of 15 minutes and maximum of 12 hours
func validAssumeRoleDuration(v any, k string) (ws []string, errors []error) {
	duration, err := time.ParseDuration(v.(string))
	if err != nil {
		errors = append(errors, fmt.Errorf("%q cannot be parsed as a duration: %w", k, err))
		return
	}

	if duration.Minutes() < 15 || duration.Hours() > 12 {
		errors = append(errors, fmt.Errorf("duration %q must be between 15 minutes (15m) and 12 hours (12h), inclusive", k))
	}

	return
}

var validAssumeRoleSessionName = validation.All(
	validation.StringLenBetween(2, 64),
	validation.StringMatch(regexache.MustCompile(`^[\w+=,.@-]+$`), ""),
)

var validAssumeRoleSourceIdentity = validation.All(
	validation.StringLenBetween(2, 64),
	validation.StringMatch(regexache.MustCompile(`^[\w+=,.@-]+$`), ""),
)

var validAssumeRoleExternalID = validation.All(
	validation.StringLenBetween(2, 1224),
	validation.StringMatch(regexache.MustCompile(`^[\w+=,.@:\/-]+$`), ""),
)

var validAssumeRolePolicy = validation.All(
	validation.StringLenBetween(1, 2048),
	validation.StringIsJSON,
)

func validAssumeRoleRoleARN(v any, k string) ([]string, []error) {
	if value, ok := v.(string); ok && value == "" {
		return nil, nil
	}

	return validation.All(
		validation.StringLenBetween(20, 2048),
		verify.ValidARNCheck(validAssumeRoleARN),
	)(v, k)
}

func validAssumeRoleARN(_ any, k string, parsedARN arn.ARN) ([]string, []error) {
	var errors []error

	if parsedARN.Partition != "ctyun" {
		errors = append(errors, fmt.Errorf("%q must be a CTYUN IAM role ARN", k))
	}

	if parsedARN.Service != "iam" || !strings.HasPrefix(parsedARN.Resource, "role/") {
		errors = append(errors, fmt.Errorf("%q must be an IAM role ARN", k))
	}

	return nil, errors
}
