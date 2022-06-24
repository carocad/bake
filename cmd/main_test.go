//go:build integration_test

package main

import (
	"context"
	"os"
	"testing"
)

func Test_do(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{oldArgs[0], "build"}

	_, err := do(context.Background())
	if err != nil {
		t.Fatal(err)
	}
}
