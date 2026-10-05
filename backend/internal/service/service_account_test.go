package service

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestServiceAccountPermissions(t *testing.T) {
	for _, role := range []string{"owner", "admin", "developer", "billing", "viewer"} {
		if !HasWorkspacePermission(role, "service_account.read") {
			t.Errorf("%s cannot read", role)
		}
		for _, permission := range []string{"service_account.create", "service_account.update", "service_account.disable", "service_account.credential.read", "service_account.credential.create", "service_account.credential.update", "service_account.credential.revoke", "service_account.credential.rotate"} {
			want := role == "owner" || role == "admin" || role == "developer"
			if HasWorkspacePermission(role, permission) != want {
				t.Errorf("%s %s", role, permission)
			}
		}
	}
}

func TestServiceAccountCredentialMetadataNeverContainsDigest(t *testing.T) {
	suffix := "abcd"
	key := &APIKey{ID: 1, Key: "sha256:private-digest", KeySuffix: &suffix}
	raw, err := json.Marshal(ServiceAccountCredentialFromKey(key))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "private-digest") || strings.Contains(string(raw), "\"key\":") {
		t.Fatalf("credential leaked: %s", raw)
	}
	if !strings.Contains(string(raw), "abcd") {
		t.Fatal("suffix absent")
	}
}

func TestServiceAccountEventSafety(t *testing.T) {
	for _, name := range ServiceAccountEventTypes() {
		if !IsWorkspaceVisibleEvent(name) {
			t.Errorf("event invisible: %s", name)
		}
		if _, err := NewDomainEvent(name, 1, 2, 3, "service_account", "4", DomainEventData{"service_account_id": 4, "credential_id": 5}); err != nil {
			t.Fatal(err)
		}
		if _, err := NewDomainEvent(name, 1, 2, 3, "service_account", "4", DomainEventData{"secret": "raw"}); err == nil {
			t.Fatal("accepted secret")
		}
	}
}
