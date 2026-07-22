package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	previous := buildVersion
	buildVersion = "1.2.3-test"
	defer func() { buildVersion = previous }()

	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	previousStdout := os.Stdout
	os.Stdout = write
	defer func() { os.Stdout = previousStdout }()
	if err = run([]string{"--version"}); err != nil {
		t.Fatal(err)
	}
	if err = write.Close(); err != nil {
		t.Fatal(err)
	}
	output, err := io.ReadAll(read)
	if err != nil {
		t.Fatal(err)
	}
	if actual := strings.TrimSpace(string(output)); actual != "1.2.3-test" {
		t.Fatalf("version output = %q", actual)
	}
}
