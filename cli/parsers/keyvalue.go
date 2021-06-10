/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package parsers

import (
	"strings"
)

func KeyValue(kv string) []string {
	return strings.SplitN(kv, "=", 2)
}
