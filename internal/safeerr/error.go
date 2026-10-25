// Package safeerr exposes bounded error codes and validated field identifiers.
package safeerr

import "errors"

type Code string

const (
	Invalid        Code = "invalid_config"
	Denied         Code = "denied"
	Conflict       Code = "conflict"
	Busy           Code = "busy"
	Unavailable    Code = "unavailable"
	NotFound       Code = "not_found"
	NotImplemented Code = "not_implemented"
	Internal       Code = "internal"
)

type Error struct {
	Code      Code
	Field     string
	Retryable bool
}

func (e *Error) Error() string {
	if e == nil || !valid(e.Code, e.Field) {
		return string(Internal)
	}
	if e.Field == "" {
		return string(e.Code)
	}
	return string(e.Code) + ": " + e.Field
}

// Public never reflects an unknown error's text.
func Public(err error) Error {
	var e *Error
	if errors.As(err, &e) && e != nil && valid(e.Code, e.Field) {
		return *e
	}
	return Error{Code: Internal}
}

func valid(code Code, field string) bool {
	switch code {
	case Invalid, Denied, Conflict, Busy, Unavailable, NotFound, NotImplemented, Internal:
	default:
		return false
	}
	if len(field) > 128 {
		return false
	}
	for i := 0; i < len(field); i++ {
		c := field[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '.' || c == '[' || c == ']' {
			continue
		}
		return false
	}
	return true
}
