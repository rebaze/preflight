package preflight

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestDiscoverySchemaFieldsMatchStrictRecords(t *testing.T) {
	data, err := os.ReadFile("../../schemas/discovery-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Definitions map[string]struct {
			Type       string                     `json:"type"`
			Additional bool                       `json:"additionalProperties"`
			Required   []string                   `json:"required"`
			Properties map[string]json.RawMessage `json:"properties"`
		} `json:"$defs"`
	}
	if err = json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	seen := map[reflect.Type]bool{}
	var check func(reflect.Type)
	check = func(typ reflect.Type) {
		for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice {
			typ = typ.Elem()
		}
		if typ.Kind() != reflect.Struct || seen[typ] {
			return
		}
		seen[typ] = true
		def, ok := schema.Definitions[typ.Name()]
		if !ok {
			t.Errorf("schema lacks %s", typ.Name())
			return
		}
		if def.Type != "object" || def.Additional {
			t.Errorf("%s must be strict object", typ.Name())
		}
		required := map[string]bool{}
		for _, name := range def.Required {
			required[name] = true
		}
		fields := 0
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			tag := strings.Split(field.Tag.Get("json"), ",")
			if tag[0] == "-" {
				continue
			}
			fields++
			if _, ok := def.Properties[tag[0]]; !ok {
				t.Errorf("%s lacks %s", typ.Name(), tag[0])
			}
			optional := len(tag) > 1 && tag[1] == "omitempty"
			if required[tag[0]] == optional {
				t.Errorf("%s.%s required mismatch", typ.Name(), tag[0])
			}
			check(field.Type)
		}
		if len(def.Properties) != fields {
			t.Errorf("%s has extra schema properties", typ.Name())
		}
	}
	check(reflect.TypeFor[Discovery]())
}
