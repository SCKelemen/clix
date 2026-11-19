package format

import "time"

// SetNowForTest makes timeago read t as the current time until the
// returned function is called.
func SetNowForTest(t time.Time) (restore func()) {
	old := now
	now = func() time.Time { return t }
	return func() { now = old }
}
