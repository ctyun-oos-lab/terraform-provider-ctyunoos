// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package types

import "testing"

func TestIsAWSAccountID(t *testing.T) { // nosemgrep:ci.aws-in-func-name
	t.Parallel()

	for _, tc := range []struct {
		id    string
		valid bool
	}{
		{"10rc2arpn6306", true},
		{"32fefj64y54gc", true},
		{"123456789012", false},
		{"1234567890123", true},
		{"12345678901", false},
		{"", false},
		{"1234567890I2", false},
	} {
		ok := IsAWSAccountID(tc.id)
		if got, want := ok, tc.valid; got != want {
			t.Errorf("IsAWSAccountID(%q) = %v, want %v", tc.id, got, want)
		}
	}
}
