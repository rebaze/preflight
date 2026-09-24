package preflight

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
)

type Profile struct {
	SchemaVersion         int               `json:"schemaVersion"`
	ID                    string            `json:"id"`
	SourcePrefixes        []string          `json:"sourcePrefixes"`
	SourceFiles           []string          `json:"sourceFiles"`
	NodeVersion           string            `json:"nodeVersion"`
	NPMVersion            string            `json:"npmVersion"`
	TestWorkspace         string            `json:"testWorkspace"`
	RequiredOverrides     map[string]string `json:"requiredOverrides"`
	RequiredJobs          []string          `json:"requiredJobs"`
	ProtectedWorkflowKeys []string          `json:"protectedWorkflowKeys"`
	Owner                 string            `json:"owner"`
	DeferredControls      []string          `json:"deferredControls"`
}

// DecodeStrict rejects duplicate keys (at any depth), unknown fields, omitted
// required struct fields, null fixed records/arrays, and trailing JSON values.
// Fields explicitly tagged omitempty are optional. It never calls external code.
func DecodeStrict(data []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	if err := checkJSONValue(d); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON value")
		}
		return err
	}
	d = json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("trailing JSON value")
	}
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	typ := reflect.TypeOf(v)
	if typ == nil || typ.Kind() != reflect.Pointer {
		return fmt.Errorf("decode destination must be a non-nil pointer")
	}
	if err := checkRequired(raw, typ.Elem(), "$"); err != nil {
		return err
	}
	return nil
}
func checkJSONValue(d *json.Decoder) error {
	tok, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok {
				return fmt.Errorf("invalid object key")
			}
			if seen[name] {
				return fmt.Errorf("duplicate JSON key %q", name)
			}
			seen[name] = true
			if err := checkJSONValue(d); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err := checkJSONValue(d); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delim)
	}
	_, err = d.Token()
	return err
}
func checkRequired(raw any, typ reflect.Type, path string) error {
	if raw == nil && typ.Kind() != reflect.Pointer && typ.Kind() != reflect.Interface {
		return fmt.Errorf("%s: null is not allowed", path)
	}
	for typ.Kind() == reflect.Pointer {
		if raw == nil {
			return nil
		}
		typ = typ.Elem()
	}
	switch typ.Kind() {
	case reflect.Struct:
		obj, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("%s: expected object", path)
		}
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			if f.PkgPath != "" {
				continue
			}
			tag := strings.Split(f.Tag.Get("json"), ",")
			key := tag[0]
			if key == "-" {
				continue
			}
			if key == "" {
				key = f.Name
			}
			value, exists := obj[key]
			optional := false
			for _, opt := range tag[1:] {
				if opt == "omitempty" {
					optional = true
				}
			}
			if !exists {
				if optional {
					continue
				}
				return fmt.Errorf("%s.%s: mandatory field missing", path, key)
			}
			if value == nil && f.Type.Kind() != reflect.Pointer {
				return fmt.Errorf("%s.%s: null is not allowed", path, key)
			}
			if err := checkRequired(value, f.Type, path+"."+key); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		a, ok := raw.([]any)
		if !ok {
			return fmt.Errorf("%s: expected array", path)
		}
		for i, v := range a {
			if err := checkRequired(v, typ.Elem(), fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	case reflect.Map:
		obj, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("%s: expected object", path)
		}
		for k, v := range obj {
			if err := checkRequired(v, typ.Elem(), path+"."+k); err != nil {
				return err
			}
		}
	}
	return nil
}
func LoadProfile(path string) (Profile, error) {
	var p Profile
	f, err := os.Open(path)
	if err != nil {
		return p, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 1024*1024+1))
	if err != nil {
		return p, err
	}
	if len(b) > 1024*1024 {
		return p, fmt.Errorf("profile exceeds 1 MiB")
	}
	if err := DecodeStrict(b, &p); err != nil {
		return p, fmt.Errorf("profile: %w", err)
	}
	if err := p.Validate(); err != nil {
		return p, err
	}
	return p, nil
}

// Validate restricts v1 to the reviewed pilot profile and runtime.
func (p Profile) Validate() error {
	if p.SchemaVersion != 1 {
		return fmt.Errorf("unsupported profile schema version %d", p.SchemaVersion)
	}
	if p.ID != "invoicex-frontend" {
		return fmt.Errorf("unsupported profile %q", p.ID)
	}
	if p.NodeVersion != "24.18.0" || p.NPMVersion != "11.17.0" {
		return fmt.Errorf("profile requires Node 24.18.0 and npm 11.17.0")
	}
	if p.TestWorkspace != "@clarula/einfache-erechnung-frontend" {
		return fmt.Errorf("unsupported test workspace")
	}
	expected := map[string][]string{"sourcePrefixes": {"applications/frontend/"}, "sourceFiles": {".github/workflows/ci.yml"}, "requiredJobs": {"changes", "frontend-eer-run", "frontend-operator-run", "build-and-test"}, "protectedWorkflowKeys": {"on", "permissions"}, "deferredControls": {"artifact.verification"}}
	actual := map[string][]string{"sourcePrefixes": p.SourcePrefixes, "sourceFiles": p.SourceFiles, "requiredJobs": p.RequiredJobs, "protectedWorkflowKeys": p.ProtectedWorkflowKeys, "deferredControls": p.DeferredControls}
	for k, want := range expected {
		if !sameStringSet(actual[k], want) {
			return fmt.Errorf("unsupported %s for fixed pilot profile", k)
		}
	}
	if len(p.RequiredOverrides) != 2 || p.RequiredOverrides["brace-expansion"] != "5.0.9" || p.RequiredOverrides["js-yaml"] != "4.3.1" {
		return fmt.Errorf("profile must preserve brace-expansion 5.0.9 and js-yaml 4.3.1 compatibility overrides")
	}
	if strings.TrimSpace(p.Owner) == "" {
		return fmt.Errorf("profile owner is required")
	}
	return nil
}
func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[string]bool{}
	for _, x := range a {
		if seen[x] {
			return false
		}
		seen[x] = true
	}
	for _, x := range b {
		if !seen[x] {
			return false
		}
	}
	return true
}
