package main

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"
)

func TestRootDockerfileGoImageMatchesModuleDirective(t *testing.T) {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
	goMod, err := os.ReadFile(filepath.Join(repoRoot, "backend", "go.mod"))
	if err != nil {
		t.Fatalf("read backend/go.mod: %v", err)
	}
	goVersionMatch := regexp.MustCompile(`(?m)^go ([0-9]+\.[0-9]+\.[0-9]+)\s*$`).FindSubmatch(goMod)
	if len(goVersionMatch) != 2 {
		t.Fatal("backend/go.mod must contain a full go patch version")
	}
	dockerVersionPattern := regexp.MustCompile(`(?m)^(?:ARG GOLANG_IMAGE=|FROM )golang:([0-9]+\.[0-9]+\.[0-9]+)-alpine\s*$`)
	for _, relativePath := range []string{"Dockerfile", "backend/Dockerfile"} {
		dockerfile, err := os.ReadFile(filepath.Join(repoRoot, relativePath))
		if err != nil {
			t.Fatalf("read %s: %v", relativePath, err)
		}
		dockerVersionMatch := dockerVersionPattern.FindSubmatch(dockerfile)
		if len(dockerVersionMatch) != 2 {
			t.Fatalf("%s must pin Go to a full alpine patch version", relativePath)
		}
		if string(dockerVersionMatch[1]) != string(goVersionMatch[1]) {
			t.Fatalf("%s Go image %s must match backend/go.mod go %s", relativePath, dockerVersionMatch[1], goVersionMatch[1])
		}
	}
}
