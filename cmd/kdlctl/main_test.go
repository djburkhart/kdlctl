package main

import "testing"

func TestMainCallsExecuteCLI(t *testing.T) {
	original := executeCLI
	t.Cleanup(func() {
		executeCLI = original
	})

	called := false
	executeCLI = func() {
		called = true
	}

	main()

	if !called {
		t.Fatal("expected executeCLI to be called")
	}
}
