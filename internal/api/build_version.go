package api

import "runtime/debug"

func currentBuildVersion(repository, service string) map[string]any {
	commit, builtAt := "unknown", ""
	dirty := false
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				commit = setting.Value
			case "vcs.time":
				builtAt = setting.Value
			case "vcs.modified":
				dirty = setting.Value == "true"
			}
		}
	}
	version := "dev"
	if commit != "unknown" {
		short := commit
		if len(short) > 12 {
			short = short[:12]
		}
		version = "git-" + short
		if dirty {
			version += "-dirty"
		}
	}
	return map[string]any{"schema": "integ.life/build-version/v1", "repository": repository, "service": service, "version": version, "commit": commit, "builtAt": builtAt, "dirty": dirty}
}
