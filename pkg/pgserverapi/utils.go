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
