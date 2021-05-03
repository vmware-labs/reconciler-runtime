/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package validation

import (
	"k8s.io/apimachinery/pkg/api/resource"
)

func Quantity(str, field string) FieldErrors {
	errs := FieldErrors{}

	if _, err := resource.ParseQuantity(str); err != nil {
		// TODO capture info about why the quantity is invalid
		errs = errs.Also(ErrInvalidValue(str, field))
	}

	return errs
}
