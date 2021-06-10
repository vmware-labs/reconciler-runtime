/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package printer

import (
	"reflect"
	"sort"
)

func SortByNamespaceAndName(s interface{}) {
	v := reflect.ValueOf(s)
	sort.SliceStable(s, func(i, j int) bool {
		vi, vj := v.Index(i), v.Index(j)

		viName, viNamespace := vi.FieldByName("Name").String(), vi.FieldByName("Namespace").String()
		vjName, vjNamespace := vj.FieldByName("Name").String(), vj.FieldByName("Namespace").String()

		switch {
		case viNamespace != vjNamespace:
			return viNamespace < vjNamespace
		default:
			return viName < vjName
		}
	})
}
