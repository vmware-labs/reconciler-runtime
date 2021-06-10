/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package validation

import (
	"strings"
)

func EnvVar(env, field string) FieldErrors {
	return KeyValue(env, field)
}

func EnvVars(envs []string, field string) FieldErrors {
	return KeyValues(envs, field)
}

func EnvVarFrom(env, field string) FieldErrors {
	errs := FieldErrors{}

	parts := strings.SplitN(env, "=", 2)
	if len(parts) != 2 || parts[0] == "" {
		errs = errs.Also(ErrInvalidValue(env, field))
	} else {
		value := strings.SplitN(parts[1], ":", 3)
		if len(value) != 3 {
			errs = errs.Also(ErrInvalidValue(env, field))
		} else if value[0] != "configMapKeyRef" && value[0] != "secretKeyRef" {
			errs = errs.Also(ErrInvalidValue(env, field))
		} else if value[1] == "" {
			errs = errs.Also(ErrInvalidValue(env, field))
		}
	}

	return errs
}

func EnvVarFroms(envs []string, field string) FieldErrors {
	errs := FieldErrors{}

	for i, env := range envs {
		errs = errs.Also(EnvVarFrom(env, CurrentField).ViaFieldIndex(field, i))
	}

	return errs
}
