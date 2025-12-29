package db

import (
	"database/sql"
	"time"
)

func FormatTimestamp(ts int64) string {
	return time.Unix(ts, 0).Format(time.RFC3339)
}

func FormatNullTimestamp(ts sql.NullInt64, fallback string) string {
	if ts.Valid {
		return FormatTimestamp(ts.Int64)
	}
	return fallback
}
