package guild

import (
	"errors"
)

var (
	ErrVersionConflict        = errors.New("guild was modified by another process; please retry")
	ErrMemberNotFound         = errors.New("member not found in database")
	ErrUnableToProcessCommand = errors.New("unable to process command; check permissions and try again")
	ErrRoleAlreadyAssigned    = errors.New("role is already assigned to the member")
	ErrRoleNotFound           = errors.New("role not found in database")
	ErrAlreadyAdminRole       = errors.New("role is already an admin role")
	ErrNotAdminRole           = errors.New("role is not an admin role")
)
