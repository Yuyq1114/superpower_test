package servicehealth

import (
	"context"
	"errors"
	"testing"
)

func TestServerTransitionsAndDependencyState(t *testing.T) {
	ok := true
	s := New(func(context.Context) error {
		if ok {
			return nil
		}
		return errors.New("down")
	})
	if s.Serving(t.Context()) {
		t.Fatal("startup ready")
	}
	s.SetServing(true)
	if !s.Serving(t.Context()) {
		t.Fatal("expected serving")
	}
	ok = false
	if s.Serving(t.Context()) {
		t.Fatal("dependency failure ready")
	}
	ok = true
	s.SetServing(false)
	if s.Serving(t.Context()) {
		t.Fatal("shutdown ready")
	}
}
