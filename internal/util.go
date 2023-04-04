/*
Copyright 2023 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package internal

import "reflect"

// IsNil returns true if the value is nilable and nil
func IsNil(val interface{}) bool {
	// return val == nil
	v := reflect.ValueOf(val)
	switch v.Kind() {
	case reflect.Chan:
		return v.IsNil()
	case reflect.Func:
		return v.IsNil()
	case reflect.Interface:
		return v.IsNil()
	case reflect.Map:
		return v.IsNil()
	case reflect.Ptr:
		return v.IsNil()
	case reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}
