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
	Tasks     []Hash
}

func newLock() *Lock {
	return &Lock{
		Version:   info.Version,
		Timestamp: time.Now(),
		Tasks:     make([]Hash, 0),
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

func (lock *Lock) Update(hasher Hasher) {
	lock.Version = info.Version
	lock.Timestamp = time.Now()
	hashes := hasher.Hash()
	for _, hash := range hashes {
		if hash.Dirty || hash.Creates == "" {
			continue
		}

		found := false
		for index, oldHash := range lock.Tasks {
			// update the hash if it already exist
			if oldHash.Path == hash.Path {
				lock.Tasks[index] = hash
				found = true
				break
			}
		}

		if !found {
			// create it otherwise
			lock.Tasks = append(lock.Tasks, hash)
		}
	}
}

func (lock *Lock) Get(path cty.Path) (*Hash, bool) {
	for _, hash := range lock.Tasks {
		if hash.Path == paths.String(path) {
			return &hash, true
		}
	}

	return nil, false
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

type Hasher interface {
	Hash() []Hash
}

type Hash struct {
	// Path is the absolute path used to refer to the parent task (ex: hello["world"])
	Path string
	// Dirty flags a Hash as comming from a Task that might have not exit correctly
	Dirty bool `json:"-"`
	// Creates keep a ref to the old filename in case it is renamed
	Creates string
	// Env hash just to check if it changes between executions
	Env string
	// Command hash just to check if it changes between executions
	Command string
}
