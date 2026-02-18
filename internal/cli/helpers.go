package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func looksLikeRepoID(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return false
	}
	return len(parts[0]) > 0 && len(parts[1]) > 0 && !strings.ContainsAny(s, " \t\n")
}

func confirmFetch(query string) bool {
	fmt.Printf("%s not in list. Fetch from HuggingFace? [y/N] ", query)
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return false
	}
	line := strings.TrimSpace(strings.ToLower(scanner.Text()))
	return line == "y" || line == "yes"
}
