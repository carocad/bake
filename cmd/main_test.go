//go:build integration_test

package main

import (
	"context"
	"fmt"
	"os"
	"testing"
)

func Test_do(t *testing.T) {
	oldArgs := os.Args
	newArgs := append([]string{oldArgs[0]}, oldArgs[3:]...)
	defer func() { os.Args = oldArgs }()
	fmt.Printf("running with %v instead of %v\n", newArgs, oldArgs)
	os.Args = newArgs

	_, err := do(context.Background())
	if err != nil {
		t.Fatal(err)
	}
}
