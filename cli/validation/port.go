/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package validation

import (
	"fmt"
	"strconv"
)

func Port(port string, field string) FieldErrors {
	portNumber, err := strconv.ParseInt(port, 10, 32)
	if err != nil {
		return ErrInvalidValue(port, field)
	}
	return PortNumber(int32(portNumber), field)
}

func PortNumber(port int32, field string) FieldErrors {
	errs := FieldErrors{}

	if port < 0 || port > 65535 {
		errs = errs.Also(ErrInvalidValue(fmt.Sprint(port), field))
	}

	return errs
}
