//go:build !darwin

package main

// normalizeLaunchArgs returns args unchanged: LaunchServices' "-psn_<pid>" token is macOS-only.
func normalizeLaunchArgs(args []string) []string {
	return args
}
