package fs

import (
	"errors"
	"fmt"
	"os"
)

var ErrRefuseOverwrite = errors.New("refusing to overwrite existing file")

// ShouldOverwrite проверяет, можно ли писать в path.
// Возвращает nil, если файла не существует либо force=true.
// Возвращает ErrRefuseOverwrite, если файл существует и force=false.
// Возвращает wrapped error для прочих проблем (stat, директория).
func ShouldOverwrite(path string, force bool) error {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return fmt.Errorf("stat output file: %w", err)
	}

	if info.IsDir() {
		return fmt.Errorf("output path is a directory: %s", path)
	}

	if !force {
		return ErrRefuseOverwrite
	}

	return nil
}
