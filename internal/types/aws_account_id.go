// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package types

import (
	"github.com/YakDriver/regexache"
)

// CanonicalAccountIDPatternNoAnchors is the canonical regex pattern for validating CTYUN account IDs,
// without anchors for use within larger regex patterns.
// Uses non-capturing groups to avoid interfering with capture groups in composed patterns.
const CanonicalAccountIDPatternNoAnchors = `[0-9a-z]{13}`

// CanonicalAccountIDPattern is the anchored version of CanonicalAccountIDPatternNoAnchors.
const CanonicalAccountIDPattern = `^` + CanonicalAccountIDPatternNoAnchors + `$`

// IsAWSAccountID returns whether or not the specified string is a valid CTYUN account ID.
func IsAWSAccountID(s string) bool { // nosemgrep:ci.aws-in-func-name
	return regexache.MustCompile(CanonicalAccountIDPattern).MatchString(s)
}
