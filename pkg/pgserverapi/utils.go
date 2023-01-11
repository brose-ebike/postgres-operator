package pgserverapi

import "fmt"

func formatQueryObj(query string, args ...string) string {
	escaped := []any{}
	for _, a := range args {
		escaped = append(escaped, "\""+a+"\"")
	}
	return fmt.Sprintf(query, escaped...)
}

func formatQueryValue(query string, args ...string) string {
	escaped := []any{}
	for _, a := range args {
		escaped = append(escaped, "'"+a+"'")
	}
	return fmt.Sprintf(query, escaped...)
}

// hasElementString checks if a given element e is contained in the slice s
func hasElementString(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
