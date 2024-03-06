/*
Copyright 2024 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package reconcilers

import (
	"unicode"
	"unicode/utf8"

	"github.com/google/go-cmp/cmp"
)

// IgnoreAllUnexported is a cmp.Option that ignores unexported fields in all structs
var IgnoreAllUnexported = cmp.FilterPath(func(p cmp.Path) bool {
	// from cmp.IgnoreUnexported with type info removed
	sf, ok := p.Index(-1).(cmp.StructField)
	if !ok {
		return false
	}
	r, _ := utf8.DecodeRuneInString(sf.Name())
	return !unicode.IsUpper(r)
}, cmp.Ignore())
