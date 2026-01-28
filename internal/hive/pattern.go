package hive

import "regexp"

// matchRemotePattern checks if remote matches the regex pattern.
// Empty pattern matches all remotes.
func matchRemotePattern(pattern, remote string) (bool, error) {
	if pattern == "" {
		return true, nil
	}
	return regexp.MatchString(pattern, remote)
}
