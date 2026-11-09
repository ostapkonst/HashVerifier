//go:build darwin

package main

import "strings"

// normalizeLaunchArgs drops the "-psn_<pid>" token LaunchServices adds when the app is
// opened from Finder; left in place it would be treated as a GUI path by the token dispatcher.
func normalizeLaunchArgs(args []string) []string {
	out := args[:1]
	for _, arg := range args[1:] {
		if !strings.HasPrefix(arg, "-psn_") {
			out = append(out, arg)
		}
	}

	return out
}
