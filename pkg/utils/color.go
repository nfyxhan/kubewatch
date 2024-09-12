package utils

import "fmt"

var (
	reset = string([]byte{27, 91, 48, 109})
	Red   = "\x1b[38;5;9m"
	// orange      = "\x1b[38;5;214m"
	// coral       = "\x1b[38;5;204m"
	Magenta = "\x1b[38;5;13m"
	Green   = "\x1b[38;5;10m"
	// darkGreen   = "\x1b[38;5;28m"
	Yellow = "\x1b[38;5;11m"
	// lightYellow = "\x1b[38;5;228m"
	Cyan = "\x1b[38;5;14m"
	// gray        = "\x1b[38;5;243m"
	// lightGray   = "\x1b[38;5;246m"
	Blue = "\x1b[38;5;12m"
)

func ColorString(color, format string, ss ...interface{}) string {
	s := fmt.Sprintf(format, ss...)
	return fmt.Sprintf("%s%s%s", color, s, reset)
}
