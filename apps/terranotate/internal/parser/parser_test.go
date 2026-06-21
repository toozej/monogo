package parser

import (
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
