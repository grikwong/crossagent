package state

import "time"

// currentTime returns the current time. Can be overridden in tests.
var currentTime = func() time.Time {
	return time.Now()
}
