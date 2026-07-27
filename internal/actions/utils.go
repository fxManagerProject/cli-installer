package actions

import (
	"strconv"
	"strings"
)

func parseSemver(v string) (major, minor, patch int) {
	v = strings.TrimPrefix(v, "v")
	// Trim pre-release suffixes like -b, -beta, etc. before parsing digits
	if idx := strings.IndexByte(v, '-'); idx != -1 {
		v = v[:idx]
	}

	parts := strings.Split(v, ".")
	if len(parts) > 0 {
		major, _ = strconv.Atoi(parts[0])
	}
	if len(parts) > 1 {
		minor, _ = strconv.Atoi(parts[1])
	}
	if len(parts) > 2 {
		patch, _ = strconv.Atoi(parts[2])
	}
	return major, minor, patch
}

func IsSameOrNewer(installed, latest string) bool {
	if strings.Contains(installed, "-b") {
		return true
	}

	iMaj, iMin, iPat := parseSemver(installed)
	lMaj, lMin, lPat := parseSemver(latest)

	if iMaj != lMaj {
		return iMaj > lMaj
	}
	if iMin != lMin {
		return iMin > lMin
	}
	return iPat >= lPat
}
