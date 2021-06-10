/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package validation

import (
	"strings"
)

func KeyValue(kv, field string) FieldErrors {
	errs := FieldErrors{}

	if strings.HasPrefix(kv, "=") || !strings.Contains(kv, "=") {
		errs = errs.Also(ErrInvalidValue(kv, field))
	}

	return errs
}

func KeyValues(kvs []string, field string) FieldErrors {
	errs := FieldErrors{}

	for i, kv := range kvs {
		errs = errs.Also(KeyValue(kv, CurrentField).ViaFieldIndex(field, i))
	}

	return errs
}
