package service

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSCIMFilterAndPaginationRejectInjectionAndUnboundedInput(t *testing.T) {
	for _, resource := range []string{"User", "Group"} {
		query, err := ParseSCIMListQuery(url.Values{"filter": {`externalId eq "O'Brian\"value"`}, "startIndex": {"2"}, "count": {"0"}}, resource)
		require.NoError(t, err)
		require.Equal(t, "externalid", query.Attribute)
		require.Equal(t, "O'Brian\"value", query.Value)
		require.Equal(t, 2, query.StartIndex)
		require.Zero(t, query.Count)
	}
	for _, filter := range []string{`userName eq "x" or id pr`, `userName co "x"`, `workspace_id eq "1"`, `id eq "x"; DROP TABLE users`, `members[value eq "x"]`, strings.Repeat("x", 1025)} {
		_, err := ParseSCIMListQuery(url.Values{"filter": {filter}}, "User")
		require.Error(t, err, filter)
	}
	for _, query := range []url.Values{{"count": {"1000000"}}, {"count": {"-1"}}, {"count": {"1", "2"}}, {"startIndex": {"0"}}, {"startIndex": {"999999999"}}, {"sortBy": {"id"}}} {
		_, err := ParseSCIMListQuery(query, "User")
		require.Error(t, err)
	}
}

func TestSCIMUserPatchCommonClientShapesAndRequiredAttributes(t *testing.T) {
	user := &SCIMUser{ID: "opaque-user", UserName: "person@example.com", Active: true, ExternalID: "external", Emails: []SCIMEmail{{Value: "person@example.com", Type: "work", Primary: true}}, Name: SCIMName{GivenName: "Before"}}
	patch := &SCIMPatchRequest{Schemas: []string{SCIMPatchSchema}, Operations: []SCIMPatchOperation{
		{Op: "Replace", Value: json.RawMessage(`{"active":false,"displayName":"Person"}`)},
		{Op: "replace", Path: "name.givenName", Value: json.RawMessage(`"After"`)},
		{Op: "add", Path: "emails", Value: json.RawMessage(`[{"value":"alternate@example.com","type":"other"}]`)},
		{Op: "remove", Path: `emails[type eq "other"]`},
		{Op: "remove", Path: "externalId"},
	}}
	input, err := ApplySCIMUserPatch(user, patch)
	require.NoError(t, err)
	require.False(t, *input.Active)
	require.Equal(t, "After", input.Name.GivenName)
	require.Equal(t, "Person", input.DisplayName)
	require.Empty(t, input.ExternalID)
	require.Equal(t, user.Emails, input.Emails)
	require.Equal(t, "Before", user.Name.GivenName, "PATCH must not change the caller's snapshot")
	for _, operation := range []SCIMPatchOperation{
		{Op: "replace", Path: "workspace_id", Value: json.RawMessage(`2`)},
		{Op: "replace", Path: "id", Value: json.RawMessage(`"another"`)},
		{Op: "replace", Value: json.RawMessage(`{"role":"owner"}`)},
		{Op: "remove", Path: "userName"},
		{Op: "replace", Path: "active", Value: json.RawMessage(`"true"`)},
	} {
		_, err = ApplySCIMUserPatch(user, &SCIMPatchRequest{Schemas: []string{SCIMPatchSchema}, Operations: []SCIMPatchOperation{operation}})
		require.Error(t, err)
	}
}

func TestSCIMGroupPatchIdempotentMemberReferencesAndBounds(t *testing.T) {
	a, b, c, absent := strings.Repeat("a", 43), strings.Repeat("b", 43), strings.Repeat("c", 43), strings.Repeat("d", 43)
	group := &SCIMGroup{ID: strings.Repeat("g", 43), DisplayName: "Engineers", Members: []SCIMMember{{Value: a}, {Value: b}}}
	patch := &SCIMPatchRequest{Schemas: []string{SCIMPatchSchema}, Operations: []SCIMPatchOperation{{Op: "add", Path: "members", Value: json.RawMessage(`[{"value":"` + b + `"},{"value":"` + c + `"}]`)}, {Op: "remove", Path: `members[value eq "` + a + `"]`}, {Op: "remove", Path: `members[value eq "` + absent + `"]`}}}
	input, err := ApplySCIMGroupPatch(group, patch)
	require.NoError(t, err)
	require.Equal(t, []SCIMMember{{Value: b}, {Value: c}}, input.Members)
	group.Members = input.Members
	repeated, err := ApplySCIMGroupPatch(group, patch)
	require.NoError(t, err)
	require.Equal(t, input.Members, repeated.Members)
	for _, operation := range []SCIMPatchOperation{{Op: "replace", Path: "id", Value: json.RawMessage(`"other"`)}, {Op: "remove", Path: `members[value eq "a" or value pr]`}, {Op: "add", Path: "members", Value: json.RawMessage(`[{"value":""}]`)}} {
		_, err := ApplySCIMGroupPatch(group, &SCIMPatchRequest{Schemas: []string{SCIMPatchSchema}, Operations: []SCIMPatchOperation{operation}})
		require.Error(t, err)
	}
	_, err = ApplySCIMGroupPatch(group, &SCIMPatchRequest{Schemas: []string{SCIMPatchSchema}, Operations: make([]SCIMPatchOperation, 33)})
	require.Error(t, err)
}

func TestSCIMBoundedJSONAndVersionPreconditions(t *testing.T) {
	for _, raw := range []string{`{"a":1,"a":2}`, strings.Repeat("[", 17) + "0" + strings.Repeat("]", 17), strings.Repeat(" ", SCIMMaxPayloadBytes+1), `{"x":1} {"y":2}`} {
		require.Error(t, ValidateSCIMJSON([]byte(raw)))
	}
	require.NoError(t, ValidateSCIMJSON([]byte(`{"schemas":[],"active":true}`)))
	for _, input := range []string{`W/"42"`, `"42"`} {
		value, err := ParseSCIMIfMatch(input)
		require.NoError(t, err)
		require.EqualValues(t, 42, value)
	}
	for _, input := range []string{`W/"0"`, `"42", "43"`, "42", "*", `W/"999999999999999999999"`} {
		_, err := ParseSCIMIfMatch(input)
		require.Error(t, err)
	}
}

func TestSCIMUserEmailAddRetryPreservesComplexValuesAndOrder(t *testing.T) {
	primary := SCIMEmail{Value: "person@example.com", Type: "work", Primary: true}
	secondary := SCIMEmail{Value: "alternate@example.com", Type: "other", Display: "Alternate"}
	newEmail := SCIMEmail{Value: "new@example.com", Type: "home"}
	for _, pathless := range []bool{false, true} {
		for _, tc := range []struct {
			name string
			add  []SCIMEmail
			want []SCIMEmail
		}{
			{"secondary", []SCIMEmail{secondary}, []SCIMEmail{primary, secondary}},
			{"primary", []SCIMEmail{primary}, []SCIMEmail{primary, secondary}},
			{"normalized secondary", []SCIMEmail{{Value: " ALTERNATE@EXAMPLE.COM ", Type: "other", Display: "Alternate"}}, []SCIMEmail{primary, secondary}},
			{"normalized primary", []SCIMEmail{{Value: " PERSON@EXAMPLE.COM ", Type: "work", Primary: true}}, []SCIMEmail{primary, secondary}},
			{"incoming duplicates and new value", []SCIMEmail{secondary, newEmail, {Value: " NEW@EXAMPLE.COM ", Type: "home"}, primary}, []SCIMEmail{primary, secondary, newEmail}},
			{"distinct type", []SCIMEmail{{Value: secondary.Value, Type: "home", Display: secondary.Display}}, []SCIMEmail{primary, secondary, {Value: secondary.Value, Type: "home", Display: secondary.Display}}},
			{"distinct display", []SCIMEmail{{Value: secondary.Value, Type: secondary.Type, Display: "New label"}}, []SCIMEmail{primary, secondary, {Value: secondary.Value, Type: secondary.Type, Display: "New label"}}},
			{"distinct primary flag", []SCIMEmail{{Value: primary.Value, Type: primary.Type}}, []SCIMEmail{primary, secondary, {Value: primary.Value, Type: primary.Type}}},
			{"type case remains distinct", []SCIMEmail{{Value: secondary.Value, Type: "OTHER", Display: secondary.Display}}, []SCIMEmail{primary, secondary, {Value: secondary.Value, Type: "OTHER", Display: secondary.Display}}},
		} {
			t.Run(fmt.Sprintf("pathless=%t/%s", pathless, tc.name), func(t *testing.T) {
				user := &SCIMUser{UserName: primary.Value, Active: true, Emails: []SCIMEmail{primary, secondary}}
				patch := scimEmailAddPatch(t, tc.add, pathless)
				input, err := ApplySCIMUserPatch(user, patch)
				require.NoError(t, err)
				require.Equal(t, tc.want, input.Emails)
				require.Equal(t, []SCIMEmail{primary, secondary}, user.Emails, "caller snapshot is unchanged")
				user.Emails = input.Emails
				for range 25 {
					repeated, err := ApplySCIMUserPatch(user, patch)
					require.NoError(t, err)
					require.Equal(t, tc.want, repeated.Emails)
					user.Emails = repeated.Emails
				}
			})
		}
	}
}

func scimEmailAddPatch(t *testing.T, emails []SCIMEmail, pathless bool) *SCIMPatchRequest {
	t.Helper()
	var value any = emails
	path := "emails"
	if pathless {
		path = ""
		value = map[string]any{"emails": emails}
	}
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	return &SCIMPatchRequest{Schemas: []string{SCIMPatchSchema}, Operations: []SCIMPatchOperation{{Op: "add", Path: path, Value: raw}}}
}

func TestSCIMUserEmailAddRetryRetainsBoundsAndValidation(t *testing.T) {
	user := &SCIMUser{UserName: "person@example.com", Active: true, Emails: []SCIMEmail{{Value: "person@example.com", Primary: true}}}
	for i := 1; i < 20; i++ {
		user.Emails = append(user.Emails, SCIMEmail{Value: fmt.Sprintf("alternate%d@example.com", i), Type: "other"})
	}
	for _, pathless := range []bool{false, true} {
		t.Run(fmt.Sprintf("pathless=%t", pathless), func(t *testing.T) {
			input, err := ApplySCIMUserPatch(user, scimEmailAddPatch(t, user.Emails, pathless))
			require.NoError(t, err)
			require.Equal(t, user.Emails, input.Emails, "replay at the stored 20-email bound is a no-op")
			for _, add := range [][]SCIMEmail{
				{{Value: "distinct21@example.com"}},
				append(append([]SCIMEmail{}, user.Emails...), user.Emails[0]),
			} {
				_, err := ApplySCIMUserPatch(user, scimEmailAddPatch(t, add, pathless))
				require.Error(t, err, "stored and raw incoming limits remain enforced")
				var typed *SCIMError
				require.ErrorAs(t, err, &typed)
				require.Equal(t, "invalidValue", typed.Type)
			}
		})
	}
	user.Emails = user.Emails[:1]
	for _, pathless := range []bool{false, true} {
		for _, add := range [][]SCIMEmail{
			{{Value: "invalid"}},
			{{Value: user.Emails[0].Value, Primary: true, Display: "different"}},
			{{Value: user.Emails[0].Value, Primary: true, Type: "different"}},
			{{Value: "new@example.com", Primary: true}},
			{{Value: user.Emails[0].Value, Primary: true, Type: strings.Repeat("x", 81)}},
			{{Value: user.Emails[0].Value, Primary: true, Display: strings.Repeat("x", 513)}},
		} {
			_, err := ApplySCIMUserPatch(user, scimEmailAddPatch(t, add, pathless))
			require.Error(t, err, "distinct conflicting primaries and malformed attributes must not be collapsed")
		}
	}
}
