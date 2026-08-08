package main

import "testing"

func TestAccFixture_basic(t *testing.T) { acceptanceHelper() }

func TestUnitHelper(t *testing.T) { unitOnlyHelper() }

// deadTestHelper has no callers at all, but lives in a _test.go file so it is never
// reported: test files are go test's domain, not dead code.
func deadTestHelper() {}
