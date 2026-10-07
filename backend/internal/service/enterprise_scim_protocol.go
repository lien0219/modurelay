package service

import (
	"bytes"
	"encoding/json"
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const SCIMMaxPayloadBytes = 256 << 10

var (
	scimEqualityFilter = regexp.MustCompile(`(?i)^([a-z][a-z0-9]*)\s+eq\s+(".*")$`)
	scimMemberSelector = regexp.MustCompile(`(?i)^members\[value\s+eq\s+(".*")\]$`)
	scimEmailSelector  = regexp.MustCompile(`(?i)^emails\[(type|value|primary)\s+eq\s+(".*"|true|false)\](?:\.(value|type|primary|display))?$`)
	scimRevision       = regexp.MustCompile(`^(?:W/)?"([1-9][0-9]*)"$`)
)

func scimInvalidValue(detail string) error { return NewSCIMError(400, "invalidValue", detail) }
func scimInvalidPath() error {
	return NewSCIMError(400, "invalidPath", "unsupported or immutable attribute path")
}

// ParseSCIMListQuery deliberately supports only parameterized equality predicates.
// Attribute names never come from the request as SQL identifiers.
func ParseSCIMListQuery(values url.Values, resource string) (SCIMListQuery, error) {
	q := SCIMListQuery{StartIndex: 1, Count: 100}
	for key, entries := range values {
		if len(entries) != 1 {
			return q, scimInvalidValue("ambiguous query parameter")
		}
		switch key {
		case "filter", "startIndex", "count":
		default:
			return q, scimInvalidValue("unsupported query parameter")
		}
	}
	for _, item := range []struct {
		name     string
		target   *int
		min, max int
	}{
		{"startIndex", &q.StartIndex, 1, 100001}, {"count", &q.Count, 0, 100},
	} {
		if v, ok := values[item.name]; ok {
			if len(v[0]) > 7 || v[0] == "" || strings.Trim(v[0], "0123456789") != "" {
				return q, scimInvalidValue("invalid pagination")
			}
			n, err := strconv.Atoi(v[0])
			if err != nil || n < item.min || n > item.max {
				return q, scimInvalidValue("invalid pagination")
			}
			*item.target = n
		}
	}
	if raw, present := values["filter"]; present {
		if len(raw[0]) > 1024 {
			return q, NewSCIMError(400, "invalidFilter", "unsupported filter")
		}
		match := scimEqualityFilter.FindStringSubmatch(strings.TrimSpace(raw[0]))
		if len(match) != 3 {
			return q, NewSCIMError(400, "invalidFilter", "unsupported filter")
		}
		attribute := strings.ToLower(match[1])
		allowed := attribute == "id" || attribute == "externalid" || resource == "User" && attribute == "username" || resource == "Group" && attribute == "displayname"
		if !allowed || json.Unmarshal([]byte(match[2]), &q.Value) != nil || !scimStringValid(q.Value, 512) {
			return q, NewSCIMError(400, "invalidFilter", "unsupported filter")
		}
		q.Attribute = attribute
	}
	return q, nil
}

func ParseSCIMIfMatch(raw string) (int64, error) {
	if raw == "" {
		return 0, nil
	}
	match := scimRevision.FindStringSubmatch(strings.TrimSpace(raw))
	if len(match) != 2 {
		return 0, scimInvalidValue("invalid version precondition")
	}
	n, err := strconv.ParseInt(match[1], 10, 64)
	if err != nil || n <= 0 {
		return 0, scimInvalidValue("invalid version precondition")
	}
	return n, nil
}

// ValidateSCIMJSON bounds work and rejects duplicate attributes (including case
// variants), so validation and decoding cannot disagree about a supplied value.
func ValidateSCIMJSON(raw []byte) error {
	if len(raw) == 0 || len(raw) > SCIMMaxPayloadBytes {
		return scimInvalidValue("invalid payload size")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	tokens := 0
	var read func(int) error
	read = func(depth int) error {
		if depth > 16 {
			return scimInvalidValue("payload nesting limit exceeded")
		}
		t, err := d.Token()
		tokens++
		if err != nil || tokens > 16000 {
			return scimInvalidValue("invalid or excessive JSON")
		}
		delim, container := t.(json.Delim)
		if !container {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				key, e := d.Token()
				tokens++
				name, ok := key.(string)
				if e != nil || !ok || tokens > 16000 || seen[strings.ToLower(name)] {
					return scimInvalidValue("duplicate or invalid JSON attribute")
				}
				seen[strings.ToLower(name)] = true
				if e = read(depth + 1); e != nil {
					return e
				}
			}
		case '[':
			for d.More() {
				if e := read(depth + 1); e != nil {
					return e
				}
			}
		default:
			return scimInvalidValue("invalid JSON")
		}
		_, err = d.Token()
		tokens++
		if err != nil || tokens > 16000 {
			return scimInvalidValue("invalid JSON")
		}
		return nil
	}
	if err := read(0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return scimInvalidValue("trailing JSON value")
	}
	return nil
}

func scimDecodeValue(raw json.RawMessage, target any) error {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || len(raw) == 0 {
		return scimInvalidValue("attribute value required")
	}
	if err := ValidateSCIMJSON(raw); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return scimInvalidValue("invalid attribute value")
	}
	return nil
}

func validateSCIMPatch(patch *SCIMPatchRequest) error {
	if patch == nil || len(patch.Schemas) != 1 || patch.Schemas[0] != SCIMPatchSchema || len(patch.Operations) < 1 || len(patch.Operations) > 32 {
		return scimInvalidValue("invalid PATCH request")
	}
	return nil
}

func ApplySCIMUserPatch(current *SCIMUser, patch *SCIMPatchRequest) (*SCIMUserInput, error) {
	if current == nil {
		return nil, NewSCIMError(404, "", "resource not found")
	}
	if err := validateSCIMPatch(patch); err != nil {
		return nil, err
	}
	active := current.Active
	in := &SCIMUserInput{Schemas: []string{SCIMUserSchema}, ID: current.ID, ExternalID: current.ExternalID, UserName: current.UserName, Active: &active, Name: current.Name, DisplayName: current.DisplayName, Emails: append([]SCIMEmail{}, current.Emails...)}
	for _, operation := range patch.Operations {
		op := strings.ToLower(operation.Op)
		if op != "add" && op != "replace" && op != "remove" {
			return nil, scimInvalidValue("unsupported PATCH operation")
		}
		if operation.Path == "" {
			if op == "remove" {
				return nil, scimInvalidPath()
			}
			var attrs map[string]json.RawMessage
			if err := scimDecodeValue(operation.Value, &attrs); err != nil {
				return nil, err
			}
			if len(attrs) == 0 {
				return nil, scimInvalidValue("attributes required")
			}
			for path, value := range attrs {
				if strings.ContainsAny(path, ".[]") {
					return nil, scimInvalidPath()
				}
				if err := patchSCIMUserAttribute(in, op, path, value); err != nil {
					return nil, err
				}
			}
		} else if err := patchSCIMUserAttribute(in, op, operation.Path, operation.Value); err != nil {
			return nil, err
		}
	}
	if _, err := ValidateSCIMUserInput(in); err != nil {
		return nil, err
	}
	return in, nil
}

func patchSCIMUserAttribute(in *SCIMUserInput, op, path string, value json.RawMessage) error {
	path = strings.ToLower(strings.TrimSpace(path))
	remove := op == "remove"
	if strings.HasPrefix(path, "emails[") {
		return patchSCIMEmails(in, op, path, value)
	}
	switch path {
	case "active":
		active := true
		if !remove {
			if err := scimDecodeValue(value, &active); err != nil {
				return err
			}
		}
		in.Active = &active
	case "username":
		if remove {
			return scimInvalidValue("userName is required")
		}
		return scimDecodeValue(value, &in.UserName)
	case "externalid":
		if remove {
			in.ExternalID = ""
		} else {
			return scimDecodeValue(value, &in.ExternalID)
		}
	case "displayname":
		if remove {
			in.DisplayName = ""
		} else {
			return scimDecodeValue(value, &in.DisplayName)
		}
	case "name":
		if remove {
			in.Name = SCIMName{}
		} else {
			var attrs map[string]json.RawMessage
			if err := scimDecodeValue(value, &attrs); err != nil {
				return err
			}
			if op == "replace" {
				in.Name = SCIMName{}
			}
			for name, v := range attrs {
				if err := patchSCIMUserAttribute(in, "replace", "name."+name, v); err != nil {
					return err
				}
			}
		}
	case "name.givenname", "name.familyname":
		target := &in.Name.GivenName
		if path == "name.familyname" {
			target = &in.Name.FamilyName
		}
		if remove {
			*target = ""
		} else {
			return scimDecodeValue(value, target)
		}
	case "emails":
		if remove {
			in.Emails = nil
		} else {
			var emails []SCIMEmail
			if err := scimDecodeValue(value, &emails); err != nil {
				return err
			}
			if len(emails) > 20 {
				return scimInvalidValue("email limit exceeded")
			}
			if op == "add" {
				// Match repository normalization only for Value. Type, Primary and
				// Display remain part of complex-value equality so distinct
				// attributes are preserved, in their original array order.
				seen := make(map[SCIMEmail]bool, len(in.Emails)+len(emails))
				for _, email := range in.Emails {
					email.Value = strings.ToLower(strings.TrimSpace(email.Value))
					seen[email] = true
				}
				for _, email := range emails {
					key := email
					key.Value = strings.ToLower(strings.TrimSpace(key.Value))
					if !seen[key] {
						in.Emails = append(in.Emails, email)
						seen[key] = true
					}
				}
			} else {
				in.Emails = emails
			}
		}
	default:
		return scimInvalidPath()
	}
	return nil
}

func patchSCIMEmails(in *SCIMUserInput, op, path string, value json.RawMessage) error {
	match := scimEmailSelector.FindStringSubmatch(path)
	if len(match) != 4 {
		return scimInvalidPath()
	}
	selector, selected, subpath := strings.ToLower(match[1]), "", strings.ToLower(match[3])
	if selector == "primary" {
		if match[2] != "true" && match[2] != "false" {
			return scimInvalidPath()
		}
		selected = match[2]
	} else if json.Unmarshal([]byte(match[2]), &selected) != nil {
		return scimInvalidPath()
	}
	found := false
	result := make([]SCIMEmail, 0, len(in.Emails))
	for _, email := range in.Emails {
		matches := selector == "type" && strings.EqualFold(email.Type, selected) || selector == "value" && strings.EqualFold(email.Value, selected) || selector == "primary" && strconv.FormatBool(email.Primary) == selected
		if matches {
			found = true
			if op == "remove" && subpath == "" {
				continue
			}
			if subpath == "" {
				var update SCIMEmail
				if err := scimDecodeValue(value, &update); err != nil {
					return err
				}
				email = update
			} else {
				switch subpath {
				case "primary":
					if op == "remove" {
						email.Primary = false
					} else if err := scimDecodeValue(value, &email.Primary); err != nil {
						return err
					}
				case "value", "type", "display":
					target := &email.Value
					if subpath == "type" {
						target = &email.Type
					}
					if subpath == "display" {
						target = &email.Display
					}
					if op == "remove" {
						*target = ""
					} else if err := scimDecodeValue(value, target); err != nil {
						return err
					}
				}
			}
		}
		result = append(result, email)
	}
	if !found && op != "remove" {
		return NewSCIMError(400, "noTarget", "email selector has no target")
	}
	in.Emails = result
	return nil
}

func ApplySCIMGroupPatch(current *SCIMGroup, patch *SCIMPatchRequest) (*SCIMGroupInput, error) {
	if current == nil {
		return nil, NewSCIMError(404, "", "resource not found")
	}
	if err := validateSCIMPatch(patch); err != nil {
		return nil, err
	}
	in := &SCIMGroupInput{Schemas: []string{SCIMGroupSchema}, ID: current.ID, ExternalID: current.ExternalID, DisplayName: current.DisplayName, Members: append([]SCIMMember{}, current.Members...)}
	for _, operation := range patch.Operations {
		op := strings.ToLower(operation.Op)
		if op != "add" && op != "replace" && op != "remove" {
			return nil, scimInvalidValue("unsupported PATCH operation")
		}
		if operation.Path == "" {
			if op == "remove" {
				return nil, scimInvalidPath()
			}
			var attrs map[string]json.RawMessage
			if err := scimDecodeValue(operation.Value, &attrs); err != nil {
				return nil, err
			}
			if len(attrs) == 0 {
				return nil, scimInvalidValue("attributes required")
			}
			for path, v := range attrs {
				if strings.ContainsAny(path, ".[]") {
					return nil, scimInvalidPath()
				}
				if err := patchSCIMGroupAttribute(in, op, path, v); err != nil {
					return nil, err
				}
			}
		} else if err := patchSCIMGroupAttribute(in, op, operation.Path, operation.Value); err != nil {
			return nil, err
		}
	}
	if err := ValidateSCIMGroupInput(in); err != nil {
		return nil, err
	}
	return in, nil
}

func patchSCIMGroupAttribute(in *SCIMGroupInput, op, path string, value json.RawMessage) error {
	lower := strings.ToLower(strings.TrimSpace(path))
	switch lower {
	case "displayname":
		if op == "remove" {
			return scimInvalidValue("displayName is required")
		}
		return scimDecodeValue(value, &in.DisplayName)
	case "externalid":
		if op == "remove" {
			in.ExternalID = ""
			return nil
		}
		return scimDecodeValue(value, &in.ExternalID)
	case "members":
		if op == "remove" {
			in.Members = []SCIMMember{}
			return nil
		}
		var members []SCIMMember
		if err := scimDecodeValue(value, &members); err != nil {
			return err
		}
		if len(members) > 200 {
			return scimInvalidValue("operation member limit exceeded")
		}
		if op == "replace" {
			in.Members = []SCIMMember{}
		}
		seen := map[string]bool{}
		for _, member := range in.Members {
			seen[member.Value] = true
		}
		for _, member := range members {
			if !ValidSCIMResourceID(member.Value) {
				return scimInvalidValue("invalid group member")
			}
			if !seen[member.Value] {
				in.Members = append(in.Members, member)
				seen[member.Value] = true
			}
		}
		if len(in.Members) > 5000 {
			return scimInvalidValue("group member limit exceeded")
		}
		return nil
	default:
		match := scimMemberSelector.FindStringSubmatch(strings.TrimSpace(path))
		if len(match) != 2 || op != "remove" {
			return scimInvalidPath()
		}
		var id string
		if json.Unmarshal([]byte(match[1]), &id) != nil || !ValidSCIMResourceID(id) {
			return scimInvalidPath()
		}
		members := make([]SCIMMember, 0, len(in.Members))
		for _, m := range in.Members {
			if m.Value != id {
				members = append(members, m)
			}
		}
		in.Members = members
		return nil
	}
}
