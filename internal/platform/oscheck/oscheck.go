package oscheck

import (
	"os"
	"runtime"
	"strconv"
	"strings"
)

const minUbuntuMajor = 22

type Result struct {
	GOOS      string `json:"goos"`
	ID        string `json:"id"`
	VersionID string `json:"version_id"`
	Supported bool   `json:"supported"`
}

func Detect() Result {
	res := Result{
		GOOS: runtime.GOOS,
	}

	if runtime.GOOS != "linux" {
		return res
	}

	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return res
	}

	values := parseOSRelease(string(data))
	res.ID = values["id"]
	res.VersionID = values["version_id"]
	res.Supported = isSupportedUbuntu(res.ID, res.VersionID)
	return res
}

func parseOSRelease(raw string) map[string]string {
	out := make(map[string]string)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.Trim(strings.TrimSpace(parts[1]), "\"")
		out[key] = strings.ToLower(value)
	}
	return out
}

func isSupportedUbuntu(id, versionID string) bool {
	if id != "ubuntu" {
		return false
	}
	parts := strings.SplitN(versionID, ".", 2)
	if len(parts) == 0 || parts[0] == "" {
		return false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return false
	}
	return major >= minUbuntuMajor
}

