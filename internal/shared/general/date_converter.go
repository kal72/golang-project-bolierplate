package shared

import (
	"golang-project-boilerplate/internal/shared/constant"
	"time"
)

func ParseDateString(date string) (time.Time, error) {
	// Parse the date string
	parsedTime, err := time.Parse(constant.DateFormat, date)
	if err != nil {
		return time.Time{}, err
	}

	return parsedTime, nil
}
