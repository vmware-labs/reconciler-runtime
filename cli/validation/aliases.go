/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package validation

import (
	"github.com/vmware-labs/reconciler-runtime/validation"
)

const CurrentField = validation.CurrentField

type FieldValidator = validation.FieldValidator
type FieldErrors = validation.FieldErrors
type Validatable = validation.Validatable

var ErrDisallowedFields = validation.ErrDisallowedFields
var ErrInvalidArrayValue = validation.ErrInvalidArrayValue
var ErrInvalidValue = validation.ErrInvalidValue
var ErrDuplicateValue = validation.ErrDuplicateValue
var ErrMissingField = validation.ErrMissingField
var ErrMissingOneOf = validation.ErrMissingOneOf
var ErrMultipleOneOf = validation.ErrMultipleOneOf
