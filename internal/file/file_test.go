package file

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExistingFileFindsFilesOnly(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "config.yml")
	if err := os.WriteFile(path, []byte("services: {}\n"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	found, ok := ExistingFile(path)
	if !ok || !filepath.IsAbs(found) {
		t.Fatalf("existing file = %q, %v", found, ok)
	}

	if _, ok := ExistingFile(directory); ok {
		t.Fatal("directory should not be reported as a file")
	}
}

func TestCopyCopiesFileContent(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "source")
	destination := filepath.Join(directory, "destination")
	if err := os.WriteFile(source, []byte("hello"), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	if err := Copy(source, destination); err != nil {
		t.Fatalf("copy file: %v", err)
	}
	content, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("read destination: %v", err)
	}
	if string(content) != "hello" {
		t.Fatalf("destination content = %q", content)
	}
}

func TestRemoveAllInsideDeletesOnlyWithinBaseDirectory(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "repo")
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatalf("create target: %v", err)
	}

	if err := RemoveAllInside(base, target); err != nil {
		t.Fatalf("remove inside base: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target still exists or stat failed unexpectedly: %v", err)
	}

	outside := t.TempDir()
	if err := RemoveAllInside(base, outside); err == nil {
		t.Fatal("remove outside base should fail")
	}
}
