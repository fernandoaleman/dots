package filesystem

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// A zero-length v2 array must survive publication; v1 must omit the field.
func (s snapshot) MarshalJSON() ([]byte, error) {
	type wire snapshot
	if s.Version == 2 {
		return json.Marshal(struct {
			wire
			Imports []importIntent `json:"imports"`
		}{wire(s), s.Imports})
	}
	return json.Marshal(wire(s))
}

// strictJSON first rejects duplicate/null values, then dispatches by version
// and structural position. Shared field names never select an object schema.
func strictJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	var value func(int) error
	value = func(depth int) error {
		if depth > 32 { // All supported schemas have a fixed, much shallower depth.
			return fmt.Errorf("ownership JSON nesting exceeds schema depth")
		}
		token, err := d.Token()
		if err != nil {
			return err
		}
		if token == nil {
			return fmt.Errorf("null state field")
		}
		if delimiter, ok := token.(json.Delim); ok {
			keys := make(map[string]bool)
			for d.More() {
				if delimiter == '{' {
					key, err := d.Token()
					if err != nil {
						return err
					}
					name := key.(string)
					if keys[name] {
						return fmt.Errorf("duplicate key %q", name)
					}
					keys[name] = true
				}
				if err := value(depth + 1); err != nil {
					return err
				}
			}
			_, err = d.Token()
		}
		return err
	}
	if err := value(0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("trailing or invalid JSON")
	}
	var header struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return err
	}
	if header.Version != 1 && header.Version != 2 {
		return fmt.Errorf("unsupported ownership registry version %d", header.Version)
	}
	return registryShape(data, fmt.Sprintf("v%d", header.Version))
}

// Scalar types and semantic constraints are checked by typed decoding and
// validateSnapshot. This pass enforces exact keys and complete nested evidence.
func registryShape(data json.RawMessage, kind string) error {
	if strings.HasPrefix(kind, "[]") {
		var elements []json.RawMessage
		if err := json.Unmarshal(data, &elements); err != nil {
			return err
		}
		for _, element := range elements {
			if err := registryShape(element, strings.TrimPrefix(kind, "[]")); err != nil {
				return err
			}
		}
		return nil
	}
	var required string
	children := map[string]string{}
	switch kind {
	case "v1", "v2":
		required = "version revision unfinished records"
		children["records"] = "[]record"
		if kind == "v2" {
			required += " imports"
			children["imports"] = "[]intent"
		}
	case "record":
		required = "profile target destination text phase parent routes leaf"
		children = map[string]string{"parent": "identity", "routes": "[]route", "leaf": "fingerprint"}
	case "intent":
		required = "id phase profile target root input source input_binding source_anchor original_file digest"
		children = map[string]string{"input_binding": "binding", "source_anchor": "anchor", "original_file": "file", "digest": "digest", "source_binding": "binding", "relocated_file": "file"}
	case "identity":
		required = "device inode"
	case "fingerprint":
		required = "id mode size modified_seconds modified_nanoseconds changed_seconds changed_nanoseconds"
		children["id"] = "identity"
	case "route":
		required = "parent name entry type text target"
		children = map[string]string{"parent": "identity", "entry": "identity", "target": "identity"}
	case "binding", "anchor":
		required = "parent routes"
		children = map[string]string{"parent": "identity", "routes": "[]route"}
		if kind == "anchor" {
			required += " missing"
		}
	case "file":
		required = "fingerprint uid gid nlink"
		children["fingerprint"] = "fingerprint"
	case "digest":
		required = "algorithm value"
	default:
		return fmt.Errorf("unknown registry schema position")
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return err
	}
	allowed := make(map[string]bool)
	for _, field := range strings.Fields(required) {
		if _, ok := object[field]; !ok {
			return fmt.Errorf("missing %s field %q", kind, field)
		}
		allowed[field] = true
	}
	if kind == "intent" {
		allowed["source_binding"], allowed["relocated_file"] = true, true
	}
	fields := make([]string, 0, len(object))
	for field := range object {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	for _, field := range fields {
		raw := object[field]
		if !allowed[field] {
			return fmt.Errorf("unknown %s field %q", kind, field)
		}
		if child := children[field]; child != "" {
			if err := registryShape(raw, child); err != nil {
				return err
			}
		}
	}
	return nil
}
