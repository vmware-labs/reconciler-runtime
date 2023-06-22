/*
Copyright 2020 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package reconcilers

import (
	"encoding/json"
	"errors"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewPatch(base, update client.Object) (*Patch, error) {
	baseBytes, err := json.Marshal(base)
	if err != nil {
		return nil, err
	}
	updateBytes, err := json.Marshal(update)
	if err != nil {
		return nil, err
	}
	patch, err := jsonpatch.CreateMergePatch(baseBytes, updateBytes)
	if err != nil {
		return nil, err
	}

	return &Patch{
		generation: base.GetGeneration(),
		bytes:      patch,
	}, nil
}

type Patch struct {
	generation int64
	bytes      []byte
}

var PatchGenerationMismatch = errors.New("patch generation did not match target")

func (p *Patch) Apply(rebase client.Object) error {
	if rebase.GetGeneration() != p.generation {
		return PatchGenerationMismatch
	}

	rebaseBytes, err := json.Marshal(rebase)
	if err != nil {
		return err
	}
	patchedBytes, err := jsonpatch.MergePatch(rebaseBytes, p.bytes)
	if err != nil {
		return err
	}
	// reset rebase to its empty value before unmarshaling into it
	replaceWithEmpty(rebase)
	return json.Unmarshal(patchedBytes, rebase)
}
