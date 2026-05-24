package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallSkills(t *testing.T) {
	dir := t.TempDir()
	if err := installSkills([]string{dir}); err != nil {
		t.Fatalf("installSkills: %v", err)
	}

	// Every bundled skill should land as <name>/SKILL.md with its embedded content.
	for _, name := range []string{"audit-go-project", "audit-go-module", "find-go-package"} {
		dst := filepath.Join(dir, name, "SKILL.md")
		got, err := os.ReadFile(dst)
		if err != nil {
			t.Errorf("%s: %v", dst, err)
			continue
		}
		want, err := skillsFS.ReadFile("skills/" + name + "/SKILL.md")
		if err != nil {
			t.Fatalf("embedded skill %s missing: %v", name, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s: installed content does not match embedded skill", dst)
		}
	}

	// The top-level skills/README.md is documentation, not a skill: don't install it.
	if _, err := os.Stat(filepath.Join(dir, "README.md")); !os.IsNotExist(err) {
		t.Errorf("README.md should not be installed; stat err = %v", err)
	}
}

func TestInstallSkillsSkipsExistingWithoutForce(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "audit-go-project", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("custom"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Default run must not clobber an existing file.
	if err := installSkills([]string{dir}); err != nil {
		t.Fatalf("installSkills: %v", err)
	}
	if got, _ := os.ReadFile(dst); string(got) != "custom" {
		t.Errorf("existing file overwritten without -force: %q", got)
	}

	// -force replaces it with the embedded content.
	if err := installSkills([]string{"-force", dir}); err != nil {
		t.Fatalf("installSkills -force: %v", err)
	}
	want, _ := skillsFS.ReadFile("skills/audit-go-project/SKILL.md")
	if got, _ := os.ReadFile(dst); !bytes.Equal(got, want) {
		t.Errorf("-force did not overwrite with embedded content")
	}
}
