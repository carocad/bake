package module

import (
	"fmt"
	"os"
	"path/filepath"
)

type FileCache struct {
	// todo: do I need to cache this?
	// files concurrent.Map[string, os.FileInfo]
}

func (cache *FileCache) Refresh(sources []string, target string) (bool, error) {
	// phony task
	if len(sources) == 0 || target == "" {
		return true, nil
	}

	// if the target doesn't exist then it should be created
	targetInfo, err := os.Stat(target)
	if err != nil {
		return true, err
	}

	for _, pattern := range sources {
		// Check pattern is well-formed.
		_, err := filepath.Match(pattern, "")
		if err != nil {
			return false, fmt.Errorf(`pattern "%s" is malformed: %w`, pattern, err)
		}

		matches, err := filepath.Glob(pattern)
		if err != nil {
			return false, err
		}

		if len(matches) == 0 {
			return false, fmt.Errorf(`pattern "%s" doesn't match any local files`, pattern)
		}

		for _, filename := range matches {
			info, err := os.Stat(filename)
			if err != nil {
				return false, err
			}

			// sources are newer than target, create it
			if info.ModTime().After(targetInfo.ModTime()) {
				return true, nil
			}
		}
	}

	return false, nil
}
