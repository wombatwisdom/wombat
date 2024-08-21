package builder

import "strings"

func ParseTarget(target string) (string, string) {
	parts := strings.Split(target, "/")
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}
