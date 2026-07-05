package store

import "errors"

var ErrNotFound = errors.New("store record not found")

var ErrConflict = errors.New("store record conflict")

func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

func IsConflict(err error) bool {
	return errors.Is(err, ErrConflict)
}
