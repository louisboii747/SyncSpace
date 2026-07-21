package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestHandleCommandVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	handled, code := handleCommand([]string{"--version"}, &stdout, &stderr)
	if !handled || code != 0 {
		t.Fatalf("handled=%v code=%d", handled, code)
	}
	if got := stdout.String(); !strings.Contains(got, "SyncSpace "+buildVersion) {
		t.Fatalf("version output=%q", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr=%q", stderr.String())
	}
}

func TestHandleCommandAllowsServerStartupWithoutArguments(t *testing.T) {
	handled, code := handleCommand(nil, &bytes.Buffer{}, &bytes.Buffer{})
	if handled || code != 0 {
		t.Fatalf("handled=%v code=%d", handled, code)
	}
}

func TestHandleCommandRejectsUnknownArguments(t *testing.T) {
	var stderr bytes.Buffer
	handled, code := handleCommand([]string{"--listen-publicly"}, &bytes.Buffer{}, &stderr)
	if !handled || code != 2 || !strings.Contains(stderr.String(), "unsupported argument") {
		t.Fatalf("handled=%v code=%d stderr=%q", handled, code, stderr.String())
	}
}
