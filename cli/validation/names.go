/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package validation

import (
	"k8s.io/apimachinery/pkg/api/validation"
)

func K8sName(name, field string) FieldErrors {
	errs := FieldErrors{}

	if out := validation.NameIsDNSLabel(name, false); len(out) != 0 {
		// TODO capture info about why the name is invalid
		errs = errs.Also(ErrInvalidValue(name, field))
	}

	return errs
}

func K8sNames(names []string, field string) FieldErrors {
	errs := FieldErrors{}

	for i, name := range names {
		if name == "" {
			errs = errs.Also(ErrInvalidValue(name, CurrentField).ViaFieldIndex(field, i))
		} else {
			errs = errs.Also(K8sName(name, CurrentField).ViaFieldIndex(field, i))
		}
	}

	return errs
}
