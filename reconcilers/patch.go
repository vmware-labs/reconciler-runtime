/*
Copyright 2020 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package reconcilers

import (
	"encoding/json"
	"errors"

	jsonmergepatch "github.com/evanphx/json-patch/v5"
	jsonpatch "gomodules.xyz/jsonpatch/v2"
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
	patch, err := jsonpatch.CreatePatch(baseBytes, updateBytes)
	if err != nil {
		return nil, err
	}
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return nil, err
	}

	return &Patch{
		generation: base.GetGeneration(),
		bytes:      patchBytes,
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
	merge, err := jsonmergepatch.DecodePatch(p.bytes)
	if err != nil {
		return err
	}
	patchedBytes, err := merge.Apply(rebaseBytes)
	if err != nil {
		return err
	}
	return json.Unmarshal(patchedBytes, rebase)
}
