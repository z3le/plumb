// internal/profile/staleness.go
package profile

import (
	"os"
	"time"
)

// StaleReason is the reason a source file newer than its profile
// carries. plumb reports the number anyway and puts this caveat beside
// it, so a reader can tell a measurement that may describe older text
// (D-45).
const StaleReason = "newer than the profile"

// ProfileModTime returns the modification time of the coverage profile
// at path. It reports the zero time when the profile cannot be stat-ed,
// which StaleAgainst reads as "make no staleness claim at all". A stat
// failure must not fail a build: the run already succeeded once to
// produce the profile, and the reason would be unrelated to coverage
// (D-45).
func ProfileModTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// StaleAgainst reports whether the source file at diskPath changed
// after the profile was written. It answers false whenever it cannot
// know: a zero profileModTime (the profile could not be stat-ed) and a
// source file that cannot be stat-ed both produce false rather than an
// error. An unreadable source file is already reported through the
// absent-file reason, and Annotate names it if it truly cannot be read,
// so a second error here would say the same thing twice.
func StaleAgainst(profileModTime time.Time, diskPath string) bool {
	if profileModTime.IsZero() {
		return false
	}
	info, err := os.Stat(diskPath)
	if err != nil {
		return false
	}
	return info.ModTime().After(profileModTime)
}
