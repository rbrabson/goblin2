package discordid

import (
	"fmt"
	"strconv"

	"github.com/disgoorg/snowflake/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// SnowflakeID is a Discord snowflake ID stored in BSON as a string.
type SnowflakeID snowflake.ID

// NewSnowflakeID converts a Disgo snowflake.ID into a SnowflakeID.
func NewSnowflakeID(id snowflake.ID) SnowflakeID {
	return SnowflakeID(id)
}

// SnowflakeIDFromString converts a string into a SnowflakeID.
func SnowflakeIDFromString(s string) (SnowflakeID, error) {
	parsed, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return SnowflakeID(0), fmt.Errorf("cannot parse SnowflakeID %q: %w", s, err)
	}
	return SnowflakeID(snowflake.ID(parsed)), nil
}

// ID converts the SnowflakeID back to a Disgo snowflake.ID.
func (s SnowflakeID) ID() snowflake.ID {
	return snowflake.ID(s)
}

// String returns the string representation of the SnowflakeID.
func (s SnowflakeID) String() string {
	return strconv.FormatUint(uint64(s), 10)
}

// MarshalBSONValue stores SnowflakeID as a BSON string.
func (s SnowflakeID) MarshalBSONValue() (byte, []byte, error) {
	t, data, err := bson.MarshalValue(s.String())
	return byte(t), data, err
}

// UnmarshalBSONValue reads SnowflakeID from a BSON string or integer.
func (s *SnowflakeID) UnmarshalBSONValue(t byte, data []byte) error {
	var value string

	switch bson.Type(t) {
	case bson.TypeString:
		if err := bson.UnmarshalValue(bson.Type(t), data, &value); err != nil {
			return err
		}

	case bson.TypeInt64:
		var id int64
		if err := bson.UnmarshalValue(bson.Type(t), data, &id); err != nil {
			return err
		}
		if id < 0 {
			return fmt.Errorf("cannot unmarshal negative int64 %d into SnowflakeID", id)
		}
		value = strconv.FormatUint(uint64(id), 10)

	case bson.TypeInt32:
		var id int32
		if err := bson.UnmarshalValue(bson.Type(t), data, &id); err != nil {
			return err
		}
		if id < 0 {
			return fmt.Errorf("cannot unmarshal negative int32 %d into SnowflakeID", id)
		}
		value = strconv.FormatUint(uint64(id), 10)

	default:
		return fmt.Errorf("cannot unmarshal BSON type %s into SnowflakeID", bson.Type(t))
	}

	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return fmt.Errorf("cannot parse SnowflakeID %q: %w", value, err)
	}

	*s = SnowflakeID(snowflake.ID(parsed))
	return nil
}
