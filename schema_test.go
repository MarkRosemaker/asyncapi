package asyncapi_test

import (
	"testing"

	"github.com/MarkRosemaker/asyncapi"
)

func TestSchema_Validate_Errors(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		schema *asyncapi.Schema
		want   string
	}{
		"unknown type": {
			&asyncapi.Schema{Type: asyncapi.DataTypes{"struct"}},
			`type ("struct") is invalid, must be one of: "integer", "number", "string", ` +
				`"array", "boolean", "object", "null"`,
		},
		"unknown type among others": {
			&asyncapi.Schema{Type: asyncapi.DataTypes{asyncapi.TypeString, "struct"}},
			`type[1] ("struct") is invalid, must be one of: "integer", "number", "string", ` +
				`"array", "boolean", "object", "null"`,
		},
		"minimum greater than maximum": {
			&asyncapi.Schema{
				Type: asyncapi.DataTypes{asyncapi.TypeInteger},
				Min:  new(10.0), Max: new(5.0),
			},
			"minimum (10) is invalid: minimum is greater than maximum (10 > 5)",
		},
		"minLength greater than maxLength": {
			&asyncapi.Schema{
				Type:      asyncapi.DataTypes{asyncapi.TypeString},
				MinLength: 10, MaxLength: new(uint(5)),
			},
			"minLength (10) is invalid: minLength is greater than maxLength (10 > 5)",
		},
		"minItems greater than maxItems": {
			&asyncapi.Schema{
				Type:     asyncapi.DataTypes{asyncapi.TypeArray},
				MinItems: 10, MaxItems: new(uint(5)),
			},
			"minItems (10) is invalid: minItems is greater than maxItems (10 > 5)",
		},
		"minProperties greater than maxProperties": {
			&asyncapi.Schema{
				Type:          asyncapi.DataTypes{asyncapi.TypeObject},
				MinProperties: 10, MaxProperties: new(uint(5)),
			},
			"minProperties (10) is invalid: minProperties is greater than maxProperties (10 > 5)",
		},
		"multipleOf zero": {
			&asyncapi.Schema{
				Type:       asyncapi.DataTypes{asyncapi.TypeNumber},
				MultipleOf: new(0.0),
			},
			"multipleOf (0) is invalid: must be greater than zero",
		},
		"discriminator that is not a property": {
			&asyncapi.Schema{
				Type:          asyncapi.DataTypes{asyncapi.TypeObject},
				Discriminator: "petType",
			},
			`discriminator ("petType") is invalid: property does not exist`,
		},
		"discriminator that is not required": {
			&asyncapi.Schema{
				Type: asyncapi.DataTypes{asyncapi.TypeObject},
				Properties: asyncapi.Schemas{
					"petType": {Value: &asyncapi.AnySchema{Schema: &asyncapi.Schema{
						Type: asyncapi.DataTypes{asyncapi.TypeString},
					}}},
				},
				Discriminator: "petType",
			},
			`discriminator ("petType") is invalid: property must be required`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := tc.schema.Validate()
			if err == nil {
				t.Fatal("expected error")
			}

			if err.Error() != tc.want {
				t.Fatalf("got: %v, want: %v", err, tc.want)
			}
		})
	}
}

func TestSchema_Validate_Discriminator(t *testing.T) {
	t.Parallel()

	s := &asyncapi.Schema{
		Type: asyncapi.DataTypes{asyncapi.TypeObject},
		Properties: asyncapi.Schemas{
			"petType": {Value: &asyncapi.AnySchema{Schema: &asyncapi.Schema{
				Type: asyncapi.DataTypes{asyncapi.TypeString},
			}}},
		},
		Required:      []string{"petType"},
		Discriminator: "petType",
	}

	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestSchema_Composition(t *testing.T) {
	t.Parallel()

	doc, err := asyncapi.LoadFromFile("examples/v3.1/anyof.yaml")
	if err != nil {
		t.Fatal(err)
	}

	if err := doc.Validate(); err != nil {
		t.Fatal(err)
	}

	payload := doc.Components.Messages["testMessages"].Value.Payload.Value.Schema
	if got, want := len(payload.AnyOf), 2; got != want {
		t.Fatalf("got: %d schemas, want: %d", got, want)
	}

	// the references of the composition were resolved
	if payload.AnyOf[0].Value != doc.Components.Schemas["objectWithKey"].Value {
		t.Fatal("the first schema was not resolved")
	}

	if payload.AnyOf[1].Value != doc.Components.Schemas["objectWithKey2"].Value {
		t.Fatal("the second schema was not resolved")
	}
}

// TestSchema_ContentSchema checks that contentSchema round-trips through the
// same AnySchemaRef machinery a payload uses — a protobuf schema needs the
// same "which format, then the schema in that format" shape a message's
// payload does, so it reuses AnySchema rather than inventing a second one.
func TestSchema_ContentSchema(t *testing.T) {
	t.Parallel()

	s := loadSchema(t, `{"type":"string","contentEncoding":"base64",`+
		`"contentMediaType":"application/vnd.google.protobuf;version=3",`+
		`"contentSchema":{"schemaFormat":"application/vnd.google.protobuf;version=3",`+
		`"schema":"syntax = \"proto3\";\n\nmessage Foo { string id = 1; }"}}`)

	cs := s.Schema.ContentSchema
	if cs == nil {
		t.Fatal("expected a content schema")
	}

	if got, want := cs.Value.SchemaFormat, asyncapi.SchemaFormatProtobuf3; got != want {
		t.Fatalf("schemaFormat: got %v, want %v", got, want)
	}

	if got, want := string(cs.Value.Raw), `"syntax = \"proto3\";\n\nmessage Foo { string id = 1; }"`; got != want {
		t.Fatalf("raw: got %s, want %s", got, want)
	}
}

// TestSchema_ContentSchema_Ref checks that a $ref inside contentSchema
// resolves the same way [Schema.Items] or [Schema.Not] would — contentSchema
// is wired into [Schema] alongside them, sharing subSchemas, so a component
// schema can be reused as more than one field's contentSchema instead of
// repeating it inline.
func TestSchema_ContentSchema_Ref(t *testing.T) {
	t.Parallel()

	doc, err := asyncapi.LoadFromDataJSON([]byte(`{"asyncapi":"3.1.0",` +
		`"info":{"title":"foo","version":"1.0.0"},` +
		`"components":{"schemas":{` +
		`"protoDef":{"schemaFormat":"application/vnd.google.protobuf;version=3",` +
		`"schema":"syntax = \"proto3\";\n\nmessage Foo { string id = 1; }"},` +
		`"test":{"type":"string","contentEncoding":"base64",` +
		`"contentSchema":{"$ref":"#/components/schemas/protoDef"}}` +
		`}}}`))
	if err != nil {
		t.Fatal(err)
	}

	if err := doc.Validate(); err != nil {
		t.Fatal(err)
	}

	cs := doc.Components.Schemas["test"].Value.Schema.ContentSchema
	if cs.Value != doc.Components.Schemas["protoDef"].Value {
		t.Fatal("contentSchema's $ref was not resolved to the referenced component")
	}
}

func TestSchema_SortMaps(t *testing.T) {
	t.Parallel()

	s := &asyncapi.Schema{
		Type: asyncapi.DataTypes{asyncapi.TypeObject},
		Properties: asyncapi.Schemas{
			"c": {Value: &asyncapi.AnySchema{Schema: &asyncapi.Schema{}}},
			"a": {Value: &asyncapi.AnySchema{Schema: &asyncapi.Schema{}}},
			"b": {Value: &asyncapi.AnySchema{Schema: &asyncapi.Schema{}}},
		},
	}

	s.SortMaps()

	want := []string{"a", "b", "c"}

	i := 0
	for name := range s.Properties.ByIndex() {
		if name != want[i] {
			t.Fatalf("got: %v, want: %v", name, want[i])
		}

		i++
	}
}
