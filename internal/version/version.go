package version

import "runtime/debug"

var Commit = "dev"

func Short() string {
	if Commit != "" && Commit != "dev" {
		return short(Commit)
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" && setting.Value != "" {
				return short(setting.Value)
			}
		}
	}
	return Commit
}

func short(value string) string {
	if len(value) <= 8 {
		return value
	}
	return value[:8]
}
