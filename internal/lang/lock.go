package lang

import (
	"encoding/json"
	"hash/crc64"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"bake/internal/lang/info"
)

const (
	BakeDirPath      = ".bake"
	BakeLockFilename = "lock.json"
)

type Lock struct {
	Version   string
	Timestamp time.Time
	Tasks     map[string]TaskHash
}

func newLock() *Lock {
	return &Lock{
		Version:   info.Version,
		Timestamp: time.Now(),
		Tasks:     make(map[string]TaskHash),
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

func (lock *Lock) Update(actions []Action) {
	lock.Version = info.Version
	lock.Timestamp = time.Now()
	for _, action := range actions {
		task, ok := action.(*Task)
		if !ok {
			continue
		}

		if ok && task.ExitCode.Valid && task.ExitCode.Int64 == 0 && task.Creates != "" {
			lock.Tasks[AddressToString(task)] = NewTaskHash(*task)
		}
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

type TaskHash struct {
	// Creates keep a ref to the old filename in case it is renamed
	Creates string
	// hash the env and the command in case they change
	// Env     string TODO
	Command string
}

func NewTaskHash(task Task) TaskHash {
	checksum := crc64.Checksum([]byte(task.Command), crc64.MakeTable(crc64.ISO))
	return TaskHash{
		Creates: task.Creates,
		Command: strconv.FormatUint(checksum, 16),
	}
}
