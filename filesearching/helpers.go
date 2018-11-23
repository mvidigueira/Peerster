package filesearching

import (
	"strings"
)

//ContainsKeyword - returns true if 'name' has a substring matching one of the keywords, false otherwise.
func ContainsKeyword(name string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(name, keyword) {
			return true
		}
	}
	return false
}
