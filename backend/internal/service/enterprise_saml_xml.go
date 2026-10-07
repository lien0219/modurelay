package service

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strings"
)

// This is a resource/ambiguity gate only. XML signature and canonicalization
// are performed by the maintained SAML/XML DSig libraries, on original bytes.
func validateSAMLXML(raw []byte, maxSize int) error {
	if len(raw) == 0 || len(raw) > maxSize {
		return errors.New("SAML XML size limit")
	}
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	decoder.Strict = true
	depth, roots, tokens := 0, 0, 0
	ids := map[string]bool{}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.New("invalid SAML XML")
		}
		tokens++
		if tokens > 16000 {
			return errors.New("SAML XML token limit")
		}
		switch item := token.(type) {
		case xml.Directive:
			return errors.New("SAML XML directives forbidden")
		case xml.ProcInst:
			if item.Target != "xml" || depth != 0 || roots != 0 {
				return errors.New("SAML XML processing instruction forbidden")
			}
		case xml.StartElement:
			depth++
			if depth == 1 {
				roots++
			}
			if depth > 64 || roots > 1 || len(item.Attr) > 64 {
				return errors.New("SAML XML structure limit")
			}
			attributes := map[xml.Name]bool{}
			for _, attr := range item.Attr {
				if attributes[attr.Name] || len(attr.Value) > 8192 {
					return errors.New("ambiguous SAML XML attributes")
				}
				attributes[attr.Name] = true
				if attr.Name.Local == "ID" || attr.Name.Local == "Id" || attr.Name.Local == "id" {
					if attr.Value == "" || len(attr.Value) > 1024 || ids[attr.Value] {
						return errors.New("duplicate or invalid SAML XML ID")
					}
					ids[attr.Value] = true
				}
				if attr.Name.Local == "Algorithm" && strings.Contains(attr.Value, "sha1") && (item.Name.Local == "SignatureMethod" || item.Name.Local == "DigestMethod") {
					return errors.New("weak SAML signature algorithm")
				}
			}
		case xml.EndElement:
			depth--
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(item)) != "" {
				return errors.New("invalid XML document")
			}
		}
	}
	if depth != 0 || roots != 1 {
		return errors.New("invalid XML document")
	}
	return nil
}
