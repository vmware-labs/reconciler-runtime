/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package testing

func StringPtr(str string) *string {
	return &str
}

func BoolPtr(b bool) *bool {
	return &b
}

func Int32Ptr(i int32) *int32 {
	return &i
}

func Int64Ptr(i int64) *int64 {
	return &i
}
