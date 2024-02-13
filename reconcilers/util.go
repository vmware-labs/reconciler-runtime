/*
Copyright 2024 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package reconcilers

import (
	"fmt"
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// extractItems returns a typed slice of objects from an object list
func extractItems[T client.Object](list client.ObjectList) []T {
	items := []T{}
	listValue := reflect.ValueOf(list).Elem()
	itemsValue := listValue.FieldByName("Items")
	for i := 0; i < itemsValue.Len(); i++ {
		itemValue := itemsValue.Index(i)
		var item T
		switch itemValue.Kind() {
		case reflect.Pointer:
			item = itemValue.Interface().(T)
		case reflect.Interface:
			item = itemValue.Interface().(T)
		case reflect.Struct:
			item = itemValue.Addr().Interface().(T)
		default:
			panic(fmt.Errorf("unknown type %s for Items slice, expected Pointer or Struct", itemValue.Kind().String()))
		}
		items = append(items, item)
	}
	return items
}
