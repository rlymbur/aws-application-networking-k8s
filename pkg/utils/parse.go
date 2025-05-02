package utils

import (
	"strconv"
)

// ParseInt32 parses a string into an int32
func ParseInt32(s string) (int32, error) {
	i, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, err
	}
	return int32(i), nil
}
