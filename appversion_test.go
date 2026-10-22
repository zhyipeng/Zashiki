package main

import (
	"regexp"
	"testing"
)

func TestCurrentVersion(t *testing.T) {
	if !regexp.MustCompile(`^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$`).MatchString(currentVersion()) {
		t.Fatalf("invalid build version %q", currentVersion())
	}
}
