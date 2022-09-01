//go:build integration_test

package main

import (
	"context"
	"fmt"
	"os"
	"testing"
)

/**
 * TestBootstrap checks that running bake on the current directory
 * wont result in an error; this is specially useful to test whether
 * or not bake is able to build itself
 */
func TestBootstrap(t *testing.T) {
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
