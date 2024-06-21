package main

import (
	"os"
	"path/filepath"
	"io"
)

func IsEmpty(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdir(1)
	if err == nil {
		return false, nil
	}
	if err == io.EOF {
		return true, nil
	}
	return false, err
}

func DeleteEmptyDirs(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			empty, _ := IsEmpty(path)
			if empty {
				os.Remove(path)
			}
		}
		return nil
	})
}