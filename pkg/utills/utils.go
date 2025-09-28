package utils

// HasLetter returns true if s contains at least one ASCII letter (a-zA-Z)
func HasLetter(s string) bool {
	for _, r := range s {
		if ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z') {
			return true
		}
	}
	return false
}

// HasNumber returns true if s contains at least one ASCII digit (0-9)
func HasNumber(s string) bool {
	for _, r := range s {
		if '0' <= r && r <= '9' {
			return true
		}
	}
	return false
}
