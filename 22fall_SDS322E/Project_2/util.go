package main

import (
	"fmt"
	"time"
)

func formatDuration(dur time.Duration) string {
	secs := int(dur.Seconds())
	mins := secs / 60
	secs = secs % 60
	hours := mins / 60
	mins = mins % 60
	return fmt.Sprintf("%02dh%02dm%02ds", hours, mins, secs)
}
