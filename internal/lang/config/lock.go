package config

import (
	"bake/internal/info"
	"bake/internal/paths"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/zclconf/go-cty/cty"
)

const (
	BakeDirPath      = ".bake"
	BakeLockFilename = "lock.json"
)

type Lock struct {
	Version   string
	Timestamp time.Time
	Tasks     map[string]Hash
}

func newLock() *Lock {
	return &Lock{
		Version:   info.Version,
		Timestamp: time.Now(),
		Tasks:     make(map[string]Hash),
	}
}

func lockFromFilesystem(cwd string) (*Lock, error) {
	lockPath := filepath.Join(cwd, BakeDirPath, BakeLockFilename)
	_, err := os.Stat(lockPath)
	if err != nil {
		return newLock(), nil
	}

	file, err := os.Open(lockPath)
	if err != nil {
		return nil, err
	}

	var lock Lock
	err = json.NewDecoder(file).Decode(&lock)
	if err != nil {
		return nil, err
	}

	return &lock, nil
}

type Hasher interface {
	Hash() (cty.Path, interface{})
}

func (lock *Lock) Update(hashes []Hasher) {
	lock.Version = info.Version
	lock.Timestamp = time.Now()
	for _, hasher := range hashes {
		path, hash := hasher.Hash()
		if hash == nil {
			continue
		}

		if hash.Dirty || hasher.Creates == "" {
			continue
		}

		lock.Tasks[paths.String(hasher.Path)] = *hasher
	}
}

func (lock *Lock) Store(cwd string) error {
	statePath := filepath.Join(cwd, BakeDirPath, BakeLockFilename)
	err := os.MkdirAll(filepath.Dir(statePath), 0770)
	if err != nil {
		return err
	}

	file, err := os.Create(statePath)
	if err != nil {
		return err
	}

	// pretty print it for easier debugging
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(lock)
}

type Hash struct {
	// Dirty flags a Hash as comming from a Task that might have not exit correctly
	Dirty bool `json:"-"`
	// Creates keep a ref to the old filename in case it is renamed
	Creates string
	// Env hash just to check if it changes between executions
	Env string
	// Command hash just to check if it changes between executions
	Command string
}
