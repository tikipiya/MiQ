package miq

import (
	"errors"
	"fmt"
)

var (
	ErrValidation = errors.New("validation error")
	ErrAsset      = errors.New("asset error")
	ErrFont       = errors.New("font error")
	ErrRender     = errors.New("render error")
	ErrAPI        = errors.New("API error")
)

// FieldError reports which public input failed validation.
type FieldError struct {
	Field string
	Err   error
}

func (e *FieldError) Error() string {
	if e.Field == "" {
		return e.Err.Error()
	}
	return fmt.Sprintf("%s: %v", e.Field, e.Err)
}

func (e *FieldError) Unwrap() error { return e.Err }

// AssetError adds source information to an image or other asset failure.
type AssetError struct {
	Source string
	Err    error
}

func (e *AssetError) Error() string {
	if e.Source == "" {
		return fmt.Sprintf("asset: %v", e.Err)
	}
	return fmt.Sprintf("asset %q: %v", e.Source, e.Err)
}

func (e *AssetError) Unwrap() error { return e.Err }

func validationError(field, message string) error {
	return &FieldError{Field: field, Err: fmt.Errorf("%s: %w", message, ErrValidation)}
}
