package config

import "fmt"

func ErrFileNotFound(err error) error {
	return fmt.Errorf("file not found: %w", err)
}

func ErrInvalidYaml(err error) error {
	return fmt.Errorf("failed to decode yaml: %w", err)
}
