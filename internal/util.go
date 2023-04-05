/*
Copyright 2023 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package internal

import "reflect"

// IsNil returns true if the value is nil, false if the value is not nilable or not nil
func IsNil(val interface{}) bool {
	if !IsNilable(val) {
		return false
	}
	return reflect.ValueOf(val).IsNil()
}

// IsNilable returns true if the value can be nil
func IsNilable(val interface{}) bool {
	v := reflect.ValueOf(val)
	switch v.Kind() {
	case reflect.Chan:
		return true
	case reflect.Func:
		return true
	case reflect.Interface:
		return true
	case reflect.Map:
		return true
	case reflect.Ptr:
		return true
	case reflect.Slice:
		return true
	default:
		return false
	}
}
