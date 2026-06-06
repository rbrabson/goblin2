package stats

import "errors"

var (
	ErrVersionConflict = errors.New("stats document was modified by another process; please retry")
)
