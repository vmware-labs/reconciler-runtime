/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package validation

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

// Deprecated CurrentField is an empty string representing an empty path to the current field.
const CurrentField = ""

type FieldValidator interface {
	Validate() FieldErrors
}

// Deprecated FieldErrors extends an ErrorList to compose helper methods to compose field paths.
type FieldErrors field.ErrorList

// Deprecated Also appends additional field errors to the current set of errors.
func (e FieldErrors) Also(errs ...FieldErrors) FieldErrors {
	aggregate := e
	for _, err := range errs {
		aggregate = append(aggregate, err...)
	}
	return aggregate
}

// Deprecated ViaField prepends the path of each error with the field's key using a dot notation separator (e.g. '.foo')
func (e FieldErrors) ViaField(key string) FieldErrors {
	errs := make(FieldErrors, len(e))
	for i, err := range e {
		newField := key
		if !strings.HasPrefix(err.Field, "[") {
			newField = newField + "."
		}
		if err.Field != "[]" {
			newField = newField + err.Field
		}
		errs[i] = &field.Error{
			Type:     err.Type,
			Field:    newField,
			BadValue: err.BadValue,
			Detail:   err.Detail,
		}
	}
	return errs
}

// Deprecated ViaIndex prepends the path of each error with the field's index using square bracket separators (e.g. '[0]').
func (e FieldErrors) ViaIndex(index int) FieldErrors {
	errs := make(FieldErrors, len(e))
	for i, err := range e {
		newField := fmt.Sprintf("[%d]", index)
		if !strings.HasPrefix(err.Field, "[") {
			newField = newField + "."
		}
		if err.Field != "[]" {
			newField = newField + err.Field
		}
		errs[i] = &field.Error{
			Type:     err.Type,
			Field:    newField,
			BadValue: err.BadValue,
			Detail:   err.Detail,
		}
	}
	return errs
}

// Deprecated ViaFieldIndex prepends the path of each error with the fields key and index (e.g. '.foo[0]').
func (e FieldErrors) ViaFieldIndex(key string, index int) FieldErrors {
	return e.ViaIndex(index).ViaField(key)
}

// Deprecated ErrorList converts a FieldErrors to an api machinery field ErrorList
func (e FieldErrors) ErrorList() field.ErrorList {
	list := make(field.ErrorList, len(e))
	for i := range e {
		list[i] = e[i]
	}
	return list
}

// Deprecated ToAggregate combines the field errors into a single go error, or nil if there are no errors.
func (e FieldErrors) ToAggregate() error {
	l := e.ErrorList()
	if len(l) == 0 {
		return nil
	}
	return l.ToAggregate()
}

// Deprecated
type Validatable = interface {
	Validate(context.Context) FieldErrors
}

// Deprecated ErrDisallowedFields wraps a forbidden error as field errors
func ErrDisallowedFields(name string, detail string) FieldErrors {
	return FieldErrors{
		field.Forbidden(field.NewPath(name), detail),
	}
}

// Deprecated ErrInvalidArrayValue wraps an invalid error for an array item as field errors
func ErrInvalidArrayValue(value interface{}, name string, index int) FieldErrors {
	return FieldErrors{
		field.Invalid(field.NewPath(name).Index(index), value, ""),
	}
}

// Deprecated ErrInvalidValue wraps an invalid error as field errors
func ErrInvalidValue(value interface{}, name string) FieldErrors {
	return FieldErrors{
		field.Invalid(field.NewPath(name), value, ""),
	}
}

// Deprecated ErrDuplicateValue wraps an duplicate error as field errors
func ErrDuplicateValue(value interface{}, names ...string) FieldErrors {
	errs := FieldErrors{}

	for _, name := range names {
		errs = append(errs, field.Duplicate(field.NewPath(name), value))
	}

	return errs
}

// Deprecated ErrMissingField wraps an required error as field errors
func ErrMissingField(name string) FieldErrors {
	return FieldErrors{
		field.Required(field.NewPath(name), ""),
	}
}

// Deprecated ErrMissingOneOf wraps an required error for the specified fields as field errors
func ErrMissingOneOf(names ...string) FieldErrors {
	return FieldErrors{
		field.Required(field.NewPath(fmt.Sprintf("[%s]", strings.Join(names, ", "))), "expected exactly one, got neither"),
	}
}

// Deprecated ErrMultipleOneOf wraps an required error for the specified fields as field errors
func ErrMultipleOneOf(names ...string) FieldErrors {
	return FieldErrors{
		field.Required(field.NewPath(fmt.Sprintf("[%s]", strings.Join(names, ", "))), "expected exactly one, got both"),
	}
}
