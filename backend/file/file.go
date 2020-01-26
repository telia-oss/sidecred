// Package file implements a sidecred.StateBackend that writes to a file.
package file

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/telia-oss/sidecred"
)

// New returns a file sidecred.StateBackend.
func New(f string) sidecred.StateBackend {
	return &fileStateBackend{file: f}
}

type fileStateBackend struct {
	file string
}

// Load implements sidecred.StateBackend.
func (b *fileStateBackend) Load() (*sidecred.State, error) {
	if err := b.createFileIfNotExists(); err != nil {
		return nil, err
	}
	var state sidecred.State
	data, err := ioutil.ReadFile(b.file)
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
func (b *fileStateBackend) Save(state *sidecred.State) error {
	if err := b.createFileIfNotExists(); err != nil {
		return err
	}
	o, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(b.file, o, 0644)
}

func (b *fileStateBackend) createFileIfNotExists() error {
	_, err := os.Stat(b.file)
	if os.IsNotExist(err) {
		_, err := os.Create(b.file)
		if err != nil {
			return fmt.Errorf("state file: %s", err)
		}
		return nil
	}
	return err
}
