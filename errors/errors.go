package errors

import (
	"errors"
	"fmt"
)

var (
	ErrConfigNotFound = errors.New("config not found")
)

func New(msg string, args ...any) error {
	return fmt.Errorf("errors: %s", fmt.Sprintf(msg, args...))
}