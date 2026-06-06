package guild

import (
	"errors"
	"fmt"
	"goblin2/discordid"
)

var (
	ErrVersionConflict   = errors.New("guild was modified by another process; please retry")
	ErrRoleAlreadyExists = errors.New("role is already an admin role")
	ErrMemberNotFound    = errors.New("member not found in database")
)

type ErrRoleNotFound struct {
	guildID  discordid.SnowflakeID
	roleName string
}

func (e ErrRoleNotFound) Error() string {
	return fmt.Sprintf("role not found: guild=%v, role=%s", e.guildID, e.roleName)
}
