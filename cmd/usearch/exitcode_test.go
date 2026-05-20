package main

import (
	"errors"
	"testing"
)

func TestClassifyError_Nil(t *testing.T) {
	if got := classifyError(nil); got != ExitSuccess {
		t.Errorf("classifyError(nil) = %d, want %d", got, ExitSuccess)
	}
}

func TestClassifyError_UserInput(t *testing.T) {
	err := errors.Join(errUserInput, errors.New("detail"))
	if got := classifyError(err); got != ExitUserError {
		t.Errorf("classifyError(userInput) = %d, want %d", got, ExitUserError)
	}
}

func TestClassifyError_System(t *testing.T) {
	err := errors.New("connection refused")
	if got := classifyError(err); got != ExitSystemError {
		t.Errorf("classifyError(systemErr) = %d, want %d", got, ExitSystemError)
	}
}

func TestExitCodeConstants(t *testing.T) {
	if ExitSuccess != 0 {
		t.Error("ExitSuccess must be 0")
	}
	if ExitUserError != 1 {
		t.Error("ExitUserError must be 1")
	}
	if ExitSystemError != 2 {
		t.Error("ExitSystemError must be 2")
	}
	if ExitPartial != 3 {
		t.Error("ExitPartial must be 3")
	}
}
