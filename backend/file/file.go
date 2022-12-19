// Package file implements a sidecred.StateBackend that writes to a file.
package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/telia-oss/sidecred"
)

// New returns a file sidecred.StateBackend.
func New() sidecred.StateBackend {
	return &fileStateBackend{}
}

type fileStateBackend struct{}

// Load implements sidecred.StateBackend.
func (b *fileStateBackend) Load(ctx context.Context, file string) (*sidecred.State, error) {
	if err := b.createFileIfNotExists(file); err != nil {
		return nil, err
	}
	var state sidecred.State
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return &state, nil
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

// Save implements sidecred.StateBackend.
func (b *fileStateBackend) Save(ctx context.Context, file string, state *sidecred.State) error {
	if err := b.createFileIfNotExists(file); err != nil {
		return err
	}
	o, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(file, o, 0o644)
}

func (b *fileStateBackend) createFileIfNotExists(file string) error {
	_, err := os.Stat(file)
	if os.IsNotExist(err) {
		_, err := os.Create(file)
		if err != nil {
			return fmt.Errorf("state file: %s", err)
		}
		return nil
	}
	return err
}
