package bank

import "errors"

var (
	ErrVersionConflict   = errors.New("document was modified by another process; please retry")
	ErrInsufficientFunds = errors.New("account has insufficient funds for this operation")
	ErrInvalidAmount     = errors.New("invalid balance amount")
)
