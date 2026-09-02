package source_test

import (
	"archive/tar"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mkosiarc/gobsi/pkg/source"
)

func TestProcessExtraSrcDirs(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello"), 0o644)

	artifacts, err := source.ProcessExtraSrcDirs([]string{dir}, t.TempDir())
	if err != nil {
		t.Fatalf("ProcessExtraSrcDirs failed: %v", err)
	}

	if len(artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(artifacts))
	}

	a := artifacts[0]
	if a.DriverName != "extra_src_dir" {
		t.Errorf("expected driver extra_src_dir, got %s", a.DriverName)
	}
	name := a.Annotations["source.artifact.name"]
	if !strings.HasPrefix(name, "extra-src-") || !strings.HasSuffix(name, ".tar") {
		t.Errorf("expected name extra-src-<checksum>.tar, got %s", name)
	}
	if a.Annotations["source.artifact.mimetype"] != "application/x-tar" {
		t.Errorf("expected mimetype application/x-tar, got %s", a.Annotations["source.artifact.mimetype"])
	}
	if _, err := os.Stat(a.Path); err != nil {
		t.Errorf("tar file does not exist: %v", err)
	}
}

func TestProcessExtraSrcDirsMultiple(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	os.WriteFile(filepath.Join(dir1, "a.txt"), []byte("aaa"), 0o644)
	os.WriteFile(filepath.Join(dir2, "b.txt"), []byte("bbb"), 0o644)

	artifacts, err := source.ProcessExtraSrcDirs([]string{dir1, dir2}, t.TempDir())
	if err != nil {
		t.Fatalf("ProcessExtraSrcDirs failed: %v", err)
	}

	if len(artifacts) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(artifacts))
	}

	name0 := artifacts[0].Annotations["source.artifact.name"]
	name1 := artifacts[1].Annotations["source.artifact.name"]
	if !strings.HasPrefix(name0, "extra-src-") || !strings.HasSuffix(name0, ".tar") {
		t.Errorf("expected name extra-src-<checksum>.tar, got %s", name0)
	}
	if !strings.HasPrefix(name1, "extra-src-") || !strings.HasSuffix(name1, ".tar") {
		t.Errorf("expected name extra-src-<checksum>.tar, got %s", name1)
	}
	if name0 == name1 {
		t.Errorf("expected different names for different dirs, both got %s", name0)
	}
}

func TestCreateTarContents(t *testing.T) {
	srcDir := t.TempDir()
	os.MkdirAll(filepath.Join(srcDir, "subdir"), 0o755)
	os.WriteFile(filepath.Join(srcDir, "b.txt"), []byte("bbb"), 0o644)
	os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("aaa"), 0o644)
	os.WriteFile(filepath.Join(srcDir, "subdir", "c.txt"), []byte("ccc"), 0o644)

	artifacts, err := source.ProcessExtraSrcDirs([]string{srcDir}, t.TempDir())
	if err != nil {
		t.Fatalf("ProcessExtraSrcDirs failed: %v", err)
	}

	f, err := os.Open(artifacts[0].Path)
	if err != nil {
		t.Fatalf("opening tar: %v", err)
	}
	defer f.Close()

	tr := tar.NewReader(f)
	var names []string
	for {
		header, err := tr.Next()
		if err != nil {
			break
		}
		names = append(names, header.Name)

		if header.Uid != 0 || header.Gid != 0 {
			t.Errorf("entry %s has non-zero uid/gid: %d/%d", header.Name, header.Uid, header.Gid)
		}
		if !header.ModTime.Equal(time.Unix(0, 0)) {
			t.Errorf("entry %s has non-epoch modtime: %v", header.Name, header.ModTime)
		}

	}

	expected := []string{"a.txt", "b.txt", "subdir", "subdir/c.txt"}
	if len(names) != len(expected) {
		t.Fatalf("expected %d entries, got %d: %v", len(expected), len(names), names)
	}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("entry %d: expected %s, got %s", i, expected[i], name)
		}
	}
}

func TestProcessExtraSrcDirsEmpty(t *testing.T) {
	artifacts, err := source.ProcessExtraSrcDirs(nil, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(artifacts) != 0 {
		t.Errorf("expected 0 artifacts, got %d", len(artifacts))
	}
}
