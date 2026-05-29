package envfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAssignment(t *testing.T) {
	name, value, err := ParseAssignment(" API_KEY =secret=value")
	if err != nil {
		t.Fatalf("parse assignment: %v", err)
	}
	if name != "API_KEY" || value != "secret=value" {
		t.Fatalf("assignment = %q, %q", name, value)
	}

	if _, _, err := ParseAssignment("1BAD=value"); err == nil {
		t.Fatal("invalid variable name should fail")
	}
	if _, _, err := ParseAssignment("MISSING_EQUALS"); err == nil {
		t.Fatal("assignment without equals should fail")
	}
}

func TestReadAndWriteEnvFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")

	if err := Write(path, map[string]string{
		"BETA":  "two words",
		"ALPHA": "one",
	}); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read raw env file: %v", err)
	}
	if !strings.HasPrefix(string(content), "ALPHA=\"one\"\nBETA=\"two words\"\n") {
		t.Fatalf("env file was not sorted and quoted: %q", content)
	}

	variables, err := Read(path)
	if err != nil {
		t.Fatalf("read env file: %v", err)
	}
	if variables["ALPHA"] != "one" || variables["BETA"] != "two words" {
		t.Fatalf("variables = %#v", variables)
	}
}

func TestReadMissingEnvFileReturnsEmptyMap(t *testing.T) {
	variables, err := Read(filepath.Join(t.TempDir(), "missing.env"))
	if err != nil {
		t.Fatalf("read missing env file: %v", err)
	}
	if len(variables) != 0 {
		t.Fatalf("variables = %#v, want empty map", variables)
	}
}
