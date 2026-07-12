package parser

import (
	"reflect"
	"testing"

	"github.com/spf13/afero"
)

func TestParseFile_Simple(t *testing.T) {
	fs := afero.NewMemMapFs()
	content := `
resource "aws_instance" "example" {
  # @metadata owner:team-a
  ami = "ami-123456"
}
`
	filename := "main.tf"
	_ = afero.WriteFile(fs, filename, []byte(content), 0644)

	prefixes := []string{"@metadata"}
	p := NewCommentParser(fs, prefixes)

	resources, err := p.ParseFile(filename)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("Expected 1 resource, got %d", len(resources))
	}

	res := resources[0]
	if res.Type != "aws_instance" || res.Name != "example" {
		t.Errorf("Unexpected resource: %s.%s", res.Type, res.Name)
	}

	if len(res.InlineComments) != 1 {
		t.Fatalf("Expected 1 inline comment, got %d", len(res.InlineComments))
	}

	comment := res.InlineComments[0]
	if comment.Prefix != "@metadata" {
		t.Errorf("Expected prefix @metadata, got %s", comment.Prefix)
	}

	if owner, ok := comment.Fields["owner"]; !ok || owner != "team-a" {
		t.Errorf("Expected owner:team-a, got %v", owner)
	}
}

func TestParseFile_NestedFields(t *testing.T) {
	fs := afero.NewMemMapFs()
	content := `
resource "test_resource" "nested" {
  # @config contact.email:user@example.com contact.slack:@user
  attribute = "value"
}
`
	filename := "nested.tf"
	_ = afero.WriteFile(fs, filename, []byte(content), 0644)

	prefixes := []string{"@config"}
	p := NewCommentParser(fs, prefixes)

	resources, err := p.ParseFile(filename)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("Expected 1 resource, got %d", len(resources))
	}

	res := resources[0]
	val := res.GetNestedField("@config", "contact.email")
	if val != "user@example.com" {
		t.Errorf("Expected contact.email to be user@example.com, got %v", val)
	}
}

func TestParseFile_FileNotFound(t *testing.T) {
	fs := afero.NewMemMapFs()
	p := NewCommentParser(fs, []string{"@metadata"})

	_, err := p.ParseFile("nonexistent.tf")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}

func TestParseFile_SplitsAdjacentPrefixesAndAssociatesWholeBlock(t *testing.T) {
	fs := afero.NewMemMapFs()
	content := `# @metadata owner:platform
# description:Primary web application
# @validation approved:true
# rule:required
# explanatory prose
# continues here

resource "aws_instance" "example" {
  ami = "ami-123456"
}
`
	if err := afero.WriteFile(fs, "main.tf", []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	resources, err := NewCommentParser(fs, []string{"@metadata", "@validation"}).ParseFile("main.tf")
	if err != nil {
		t.Fatal(err)
	}
	if len(resources) != 1 || len(resources[0].PrecedingComments) != 2 {
		t.Fatalf("unexpected parsed resources: %+v", resources)
	}
	if resources[0].PrecedingComments[0].Prefix != "@metadata" || resources[0].PrecedingComments[1].Prefix != "@validation" {
		t.Fatalf("adjacent prefixes were not split: %+v", resources[0].PrecedingComments)
	}
	if got := resources[0].PrecedingComments[0].Fields["description"]; got != "Primary web application" {
		t.Fatalf("description = %#v, want full spaced value", got)
	}
}

func TestParseFile_AssignsPrecedingCommentOnlyToNearestResource(t *testing.T) {
	fs := afero.NewMemMapFs()
	content := `# @metadata owner:first-only
resource "test" "first" {}

resource "test" "second" {}
`
	if err := afero.WriteFile(fs, "main.tf", []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	resources, err := NewCommentParser(fs, []string{"@metadata"}).ParseFile("main.tf")
	if err != nil {
		t.Fatal(err)
	}
	if len(resources) != 2 {
		t.Fatalf("got %d resources", len(resources))
	}
	if len(resources[0].PrecedingComments) != 1 || len(resources[1].PrecedingComments) != 0 {
		t.Fatalf("comment association leaked across resources: %+v", resources)
	}
}

func TestParseFile_RequiresExactPrefixBoundary(t *testing.T) {
	fs := afero.NewMemMapFs()
	content := `# @metadata-typo owner:wrong
resource "test" "example" {}
`
	if err := afero.WriteFile(fs, "main.tf", []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	resources, err := NewCommentParser(fs, []string{"@metadata"}).ParseFile("main.tf")
	if err != nil {
		t.Fatal(err)
	}
	if len(resources[0].PrecedingComments) != 0 {
		t.Fatalf("typoed prefix was accepted: %+v", resources[0].PrecedingComments)
	}
}

func TestParseValueConsumesWholeNumber(t *testing.T) {
	p := NewCommentParser(afero.NewMemMapFs(), nil)
	tests := map[string]interface{}{
		"99.9":  float64(99.9),
		"-12":   -12,
		"-1.25": float64(-1.25),
		"12x":   "12x",
	}
	for input, want := range tests {
		if got := p.parseValue(input); got != want {
			t.Errorf("parseValue(%q) = %#v, want %#v", input, got, want)
		}
	}
}

func TestParseFile_PreservesQuotedKeyLikeText(t *testing.T) {
	fs := afero.NewMemMapFs()
	content := `# @docs description:"Deploy owner: service" format:markdown
resource "test" "example" {}
`
	if err := afero.WriteFile(fs, "main.tf", []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	resources, err := NewCommentParser(fs, []string{"@docs"}).ParseFile("main.tf")
	if err != nil {
		t.Fatal(err)
	}
	comment := resources[0].PrecedingComments[0]
	if got := comment.Fields["description"]; got != "Deploy owner: service" {
		t.Fatalf("description = %#v", got)
	}
	if got := comment.Fields["format"]; got != "markdown" {
		t.Fatalf("format = %#v", got)
	}
	if _, exists := comment.Fields["owner"]; exists {
		t.Fatalf("quoted text was parsed as a field: %#v", comment.Fields)
	}
}

func TestParseFile_ApostropheDoesNotHideFollowingField(t *testing.T) {
	fs := afero.NewMemMapFs()
	content := `# @docs description:John's service owner:platform
resource "test" "example" {}
`
	if err := afero.WriteFile(fs, "main.tf", []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	resources, err := NewCommentParser(fs, []string{"@docs"}).ParseFile("main.tf")
	if err != nil {
		t.Fatal(err)
	}
	comment := resources[0].PrecedingComments[0]
	if got := comment.Fields["description"]; got != "John's service" {
		t.Fatalf("description = %#v", got)
	}
	if got := comment.Fields["owner"]; got != "platform" {
		t.Fatalf("owner = %#v", got)
	}
}

func TestParseValueRejectsNonFiniteNumbersAndParsesEmptyArray(t *testing.T) {
	p := NewCommentParser(afero.NewMemMapFs(), nil)
	for _, input := range []string{"NaN", "Inf", "-Inf"} {
		if got := p.parseValue(input); got != input {
			t.Errorf("parseValue(%q) = %#v, want string", input, got)
		}
	}
	if got := p.parseValue("[]"); !reflect.DeepEqual(got, []interface{}{}) {
		t.Errorf("parseValue(\"[]\") = %#v, want empty array", got)
	}
}
