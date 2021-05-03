/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package validation

import (
	"strings"
)

func ObjectReference(ref, field string) FieldErrors {
	errs := FieldErrors{}

	parts := strings.Split(ref, ":")
	if len(parts) != 3 {
		errs = errs.Also(ErrInvalidValue(ref, field))
		return errs
	}

	if parts[0] != "v1" && !strings.Contains(parts[0], "/") {
		errs = errs.Also(ErrInvalidValue(ref, field))
	}
	if parts[1] == "" {
		errs = errs.Also(ErrInvalidValue(ref, field))
	}
	errs = errs.Also(K8sName(parts[2], field))

	return errs
}

func ObjectReferences(refs []string, field string) FieldErrors {
	errs := FieldErrors{}

	for i, ref := range refs {
		errs = errs.Also(ObjectReference(ref, CurrentField).ViaFieldIndex(field, i))
	}

	return errs
}
