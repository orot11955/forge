package util

import "time"

func NowISO() string {
	return time.Now().Format(time.RFC3339)
}
