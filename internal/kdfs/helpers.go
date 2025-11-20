package kdfs

import (
	"strings"
)

func ReverseMap(m map[string]string) map[string]string {
	r := make(map[string]string, len(m))
	for k, v := range m {
		r[v] = k
	}
	return r
}

var kpToFS = map[string]string{
	"UserName": "username",
	"Password": "password",
	"URL":      "url",
	"Notes":    "notes",
}

var fsToKP = ReverseMap(kpToFS)

func cleanEntryPath(entryPath string) string {
	splits := strings.Split(entryPath, ".entry")
	return splits[0]
}
