// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package pgpkeys

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

func TestFetchKeybasePubkeys(t *testing.T) {
	t.Parallel()

	type testKey struct {
		username    string
		bundle      string
		fingerprint string
	}

	testKeys := make([]testKey, 2)
	for i, username := range []string{"jefferai", "hashicorp"} {
		entity, err := openpgp.NewEntity(username, "", username+"@example.com", nil)
		if err != nil {
			t.Fatalf("creating test key for %s: %v", username, err)
		}

		var bundle bytes.Buffer
		armorWriter, err := armor.Encode(&bundle, openpgp.PublicKeyType, nil)
		if err != nil {
			t.Fatalf("encoding test key for %s: %v", username, err)
		}
		if err := entity.Serialize(armorWriter); err != nil {
			t.Fatalf("serializing test key for %s: %v", username, err)
		}
		if err := armorWriter.Close(); err != nil {
			t.Fatalf("closing test key armor for %s: %v", username, err)
		}

		testKeys[i] = testKey{
			username:    username,
			bundle:      bundle.String(),
			fingerprint: hex.EncodeToString(entity.PrimaryKey.Fingerprint[:]),
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/_/api/1.0/user/lookup.json"; got != want {
			t.Errorf("request path = %q, want %q", got, want)
		}
		if got, want := r.URL.Query().Get("usernames"), "jefferai,hashicorp"; got != want {
			t.Errorf("usernames = %q, want %q", got, want)
		}
		if got, want := r.URL.Query().Get("fields"), "public_keys"; got != want {
			t.Errorf("fields = %q, want %q", got, want)
		}

		response := map[string]any{
			"status": map[string]string{"name": "OK"},
			"them": []map[string]any{
				{"public_keys": map[string]any{"primary": map[string]string{"bundle": testKeys[0].bundle}}},
				{"public_keys": map[string]any{"primary": map[string]string{"bundle": testKeys[1].bundle}}},
			},
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("writing mock Keybase response: %v", err)
		}
	}))
	defer server.Close()

	testset := []string{"keybase:jefferai", "keybase:hashicorp"}
	ret, err := fetchKeybasePubkeys(server.Client(), server.URL+"/_/api/1.0/user/lookup.json", testset)
	if err != nil {
		t.Fatalf("fetching test Keybase public keys: %v", err)
	}

	for _, testKey := range testKeys {
		data, err := base64.StdEncoding.DecodeString(ret[kbPrefix+testKey.username])
		if err != nil {
			t.Fatalf("decoding key for user %s: %v", testKey.username, err)
		}
		entity, err := openpgp.ReadEntity(packet.NewReader(bytes.NewBuffer(data)))
		if err != nil {
			t.Fatalf("parsing key for user %s: %v", testKey.username, err)
		}
		if got := hex.EncodeToString(entity.PrimaryKey.Fingerprint[:]); got != testKey.fingerprint {
			t.Fatalf("fingerprint for user %s = %q, want %q", testKey.username, got, testKey.fingerprint)
		}
	}
}
