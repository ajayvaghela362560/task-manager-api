package models

import (
	"strings"
	"testing"
)

func strPtr(s string) *string { return &s }

func TestTaskInputValidate(t *testing.T) {
	cases := []struct {
		name      string
		in        TaskInput
		create    bool
		wantField string // "" means the input must be valid
	}{
		{"create with only a title is valid", TaskInput{Title: strPtr("Write report")}, true, ""},
		{"create requires title", TaskInput{}, true, "title"},
		{"blank title rejected", TaskInput{Title: strPtr("   ")}, true, "title"},
		{"overlong title rejected", TaskInput{Title: strPtr(strings.Repeat("x", MaxTitleLen+1))}, true, "title"},
		{"invalid status rejected", TaskInput{Title: strPtr("ok"), Status: strPtr("archived")}, true, "status"},
		{"invalid priority rejected", TaskInput{Title: strPtr("ok"), Priority: strPtr("urgent")}, true, "priority"},
		{"invalid due date rejected", TaskInput{Title: strPtr("ok"), DueDate: strPtr("tomorrow")}, true, "dueDate"},
		{"valid due date accepted", TaskInput{Title: strPtr("ok"), DueDate: strPtr("2026-06-15T17:00:00Z")}, true, ""},
		{"patch with no fields is valid", TaskInput{}, false, ""},
		{"patch cannot blank the title", TaskInput{Title: strPtr("")}, false, "title"},
		{"patch can clear the due date", TaskInput{DueDate: strPtr("")}, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := tc.in.Validate(tc.create)
			if tc.wantField == "" {
				if len(errs) != 0 {
					t.Fatalf("expected valid input, got errors: %v", errs)
				}
				return
			}
			if _, ok := errs[tc.wantField]; !ok {
				t.Fatalf("expected an error on field %q, got: %v", tc.wantField, errs)
			}
		})
	}
}

func TestSignupInputValidate(t *testing.T) {
	valid := SignupInput{Name: "Ada", Email: "ada@example.com", Password: "secret-password"}
	if errs := valid.Validate(); len(errs) != 0 {
		t.Fatalf("expected valid signup, got errors: %v", errs)
	}

	cases := []struct {
		name      string
		in        SignupInput
		wantField string
	}{
		{"missing name", SignupInput{Email: "a@b.co", Password: "longenough"}, "name"},
		{"bad email", SignupInput{Name: "Ada", Email: "not-an-email", Password: "longenough"}, "email"},
		{"short password", SignupInput{Name: "Ada", Email: "a@b.co", Password: "short"}, "password"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, ok := tc.in.Validate()[tc.wantField]; !ok {
				t.Fatalf("expected an error on field %q, got: %v", tc.wantField, tc.in.Validate())
			}
		})
	}
}
