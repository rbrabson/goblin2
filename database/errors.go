package database

import "errors"

var (
	ErrInvalidDocument         = errors.New("unable to decode document")
	ErrCollectionNotAccessible = errors.New("unable to create or access the collection")
)
