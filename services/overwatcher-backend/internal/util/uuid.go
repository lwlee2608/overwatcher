package util

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

// UUIDToString converts a pgtype.UUID to its string representation.
func UUIDToString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	b := u.Bytes
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
