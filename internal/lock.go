package internal

import (
	"encoding/json"
	"hash/crc64"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"bake/internal/lang"
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

type TaskHash struct {
	// Creates keep a ref to the old filename in case it is renamed
	Creates string
	// hash the env and the command in case they change
	// Env     string TODO
	Command string
}

func newLock() *Lock {
	return &Lock{
		Version:   info.Version,
		Timestamp: time.Now(),
		Tasks:     make(map[string]TaskHash),
	}
}

func readLock(cwd string) (*Lock, error) {
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

func (lock *Lock) update(actions []lang.Action) {
	lock.Version = info.Version
	lock.Timestamp = time.Now()
	for _, action := range actions {
		task, ok := action.(*lang.Task)
		if ok && task.ExitCode.Int64 == 0 && task.Creates != "" {
			checksum := crc64.Checksum([]byte(task.Command), crc64.MakeTable(crc64.ISO))
			lock.Tasks[lang.PathString(task.GetPath())] = TaskHash{
				Creates: task.Creates,
				Command: strconv.FormatUint(checksum, 16),
			}
		}
	}
}

func (lock *Lock) store(cwd string) error {
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
