package models

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MaxTitleLen       = 200
	MaxDescriptionLen = 5000
	MaxNameLen        = 100
	MinPasswordLen    = 8
	MaxPasswordLen    = 72 // bcrypt operates on at most 72 bytes
)

var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

func ValidStatus(s string) bool {
	return s == StatusTodo || s == StatusInProgress || s == StatusDone
}

func ValidPriority(p string) bool {
	return p == PriorityLow || p == PriorityMedium || p == PriorityHigh
}

// TaskInput is the JSON body accepted by the create and update endpoints.
// Pointer fields distinguish "absent" from "empty" so PATCH can apply
// partial updates; an empty dueDate string clears the due date.
type TaskInput struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
	Priority    *string `json:"priority"`
	DueDate     *string `json:"dueDate"`
}

// Validate returns a map of field name to problem; an empty map means the
// input is valid. When create is true the title must be present.
func (in TaskInput) Validate(create bool) map[string]string {
	errs := map[string]string{}
	if create && in.Title == nil {
		errs["title"] = "title is required"
	}
	if in.Title != nil {
		t := strings.TrimSpace(*in.Title)
		switch {
		case t == "":
			errs["title"] = "title must not be empty"
		case utf8.RuneCountInString(t) > MaxTitleLen:
			errs["title"] = fmt.Sprintf("title must be at most %d characters", MaxTitleLen)
		}
	}
	if in.Description != nil && utf8.RuneCountInString(*in.Description) > MaxDescriptionLen {
		errs["description"] = fmt.Sprintf("description must be at most %d characters", MaxDescriptionLen)
	}
	if in.Status != nil && !ValidStatus(*in.Status) {
		errs["status"] = "status must be one of: todo, in_progress, done"
	}
	if in.Priority != nil && !ValidPriority(*in.Priority) {
		errs["priority"] = "priority must be one of: low, medium, high"
	}
	if in.DueDate != nil && *in.DueDate != "" {
		if _, err := time.Parse(time.RFC3339, *in.DueDate); err != nil {
			errs["dueDate"] = "dueDate must be a valid RFC3339 timestamp, e.g. 2026-06-15T17:00:00Z"
		}
	}
	return errs
}

type SignupInput struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (in SignupInput) Validate() map[string]string {
	errs := map[string]string{}
	switch {
	case strings.TrimSpace(in.Name) == "":
		errs["name"] = "name is required"
	case utf8.RuneCountInString(in.Name) > MaxNameLen:
		errs["name"] = fmt.Sprintf("name must be at most %d characters", MaxNameLen)
	}
	if !emailRe.MatchString(in.Email) {
		errs["email"] = "a valid email address is required"
	}
	switch {
	case len(in.Password) < MinPasswordLen:
		errs["password"] = fmt.Sprintf("password must be at least %d characters", MinPasswordLen)
	case len(in.Password) > MaxPasswordLen:
		errs["password"] = fmt.Sprintf("password must be at most %d bytes", MaxPasswordLen)
	}
	return errs
}

type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
