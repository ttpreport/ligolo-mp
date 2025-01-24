package utils

import (
	"fmt"
	"time"
)

func HumanBool(v bool) string {
	if v {
		return "Yes"
	} else {
		return "No"
	}
}

func HumanTime(v time.Time) string {
	return fmt.Sprintf("%d-%d-%d %d:%d:%d",
		v.Year(),
		v.Month(),
		v.Day(),
		v.Hour(),
		v.Minute(),
		v.Second())
}

func HumanTimeSince(v time.Time) string {
	duration := time.Since(v)

	if duration.Minutes() < 1 {
		return "now"
	}

	if duration.Hours() > 24 {
		return fmt.Sprintf("%d days ago", int(duration.Hours()/24))
	}

	if duration.Minutes() > 60 {
		return fmt.Sprintf("%d hours ago", int(duration.Hours()))
	}

	return fmt.Sprintf("%d minutes ago", int(duration.Minutes()))
}
