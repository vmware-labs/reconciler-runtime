/*
Copyright 2020 the original author or authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Object = client.Object
type ObjectList = client.ObjectList
type Client = client.Client
type Reader = client.Reader
type Writer = client.Writer
type StatusClient = client.StatusClient
type StatusWriter = client.StatusWriter
type ObjectKey = client.ObjectKey
type InNamespace = client.InNamespace
type ListOption = client.ListOption
type ListOptions = client.ListOptions
type CreateOption = client.CreateOption
type UpdateOption = client.UpdateOption
type Patch = client.Patch
type PatchOption = client.PatchOption
type DeleteOption = client.DeleteOption
type DeleteAllOfOption = client.DeleteAllOfOption
type MatchingFields = client.MatchingFields
