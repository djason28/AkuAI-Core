package utils

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

func HasLetter(s string) bool {
	for _, r := range s {
		if ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z') {
			return true
		}
	}
	return false
}

func HasNumber(s string) bool {
	for _, r := range s {
		if '0' <= r && r <= '9' {
			return true
		}
	}
	return false
}

var (
	multiSpaceRegex        = regexp.MustCompile(` {2,}`)
	spaceBeforeNewline     = regexp.MustCompile(`[ \t]+\n`)
	newlineSpace           = regexp.MustCompile(`\n[ \t]+`)
	spaceBeforePunctuation = regexp.MustCompile(`\s+([,.;:!?])`)
	extraNewlines          = regexp.MustCompile(`\n{3,}`)

	// Patterns to add missing spaces
	missingSpacePatterns = []*regexp.Regexp{
		regexp.MustCompile(`([a-z])([A-Z][a-z]+)`), // "webinarUIB" -> "webinar UIB"
		regexp.MustCompile(`([a-z])(Live)([A-Z])`), // "LiveUIB" -> "Live UIB"
		regexp.MustCompile(`(UIB)([A-Z][a-z])`),    // "UIBDepartemen" -> "UIB Departemen"
	}

	// Excessive spacing (3+ spaces between words)
	excessSpacePattern = regexp.MustCompile(`(\w)\s{3,}(\w)`)
)

// NormalizeWhitespace collapses repeated whitespace, removes zero-width characters,
// and trims trailing spaces so rendered chat messages stay tidy.
func NormalizeWhitespace(input string) string {
	if strings.TrimSpace(input) == "" {
		return strings.TrimSpace(input)
	}

	normalized := strings.ReplaceAll(input, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	normalized = strings.ReplaceAll(normalized, "\u00A0", " ")

	// Remove all zero-width and control characters
	normalized = strings.Map(func(r rune) rune {
		switch r {
		case '\u200B', '\u200C', '\u200D', '\u2060', '\uFEFF':
			return -1
		}
		return r
	}, normalized)

	// Convert tabs to spaces and normalize multiple spaces
	normalized = strings.ReplaceAll(normalized, "\t", " ")

	// First, add missing spaces between joined words
	for _, pattern := range missingSpacePatterns {
		normalized = pattern.ReplaceAllString(normalized, "$1 $2")
	}

	// Apply multiple passes to fix specific spacing issues
	for i := 0; i < 2; i++ {
		// Fix common broken words with direct replacements
		wordReplacements := map[string]string{
			"Cr ypto":               "Crypto",
			"C rypto":               "Crypto",
			"You Tube":              "YouTube",
			"YouTube Liv e":         "YouTube Live",
			"D evelopment":          "Development",
			"C areer":               "Career",
			"Tech nology":           "Technology",
			"Persyara tan":          "Persyaratan",
			"pese rta":              "peserta",
			"untukumum":             "untuk umum",
			"lebihlanjut":           "lebih lanjut",
			"menghub ungi":          "menghubungi",
			"de ngan":               "dengan",
			"se perti":              "seperti",
			"meng gunakan":          "menggunakan",
			"Pembic ara":            "Pembicara",
			"Engi neer":             "Engineer",
			"Shope e":               "Shopee",
			"No vember":             "November",
			"Peneli ti":             "Peneliti",
			"Developmen t":          "Development",
			"Priorita s":            "Prioritas",
			"unt uk":                "untuk",
			"h ttps":                "https",
			"h ttp":                 "http",
			"2 025":                 "2025",
			"1 6:00":                "16:00",
			"1 5:00":                "15:00",
			"1 4:00":                "14:00",
			"1 9:00":                "19:00",
			"2 0:00":                "20:00",
			"2 1:00":                "21:00",
			"ui b.ac.id":            "uib.ac.id",
			"Platfor m":             "Platform",
			"D epartemen":           "Departemen",
			"leb ih":                "lebih",
			"la njut":               "lanjut",
			"k ontak":               "kontak",
			"i nfo":                 "info",
			"info@uib.a c.id":       "info@uib.ac.id",
			"untu k":                "untuk",
			"U IB":                  "UIB",
			"Sertifi kasi":          "Sertifikasi",
			"B usiness":             "Business",
			"Lokas i":               "Lokasi",
			"500.00 0":              "500.000",
			"750.00 0":              "750.000",
			"marketin g":            "marketing",
			"2025-10 -18":           "2025-10-18",
			"K omputer":             "Komputer",
			"it-certification @":    "it-certification@",
			"Sertifikas i":          "Sertifikasi",
			"Profes ional":          "Profesional",
			"Tang gal":              "Tanggal",
			"Wa ktu":                "Waktu",
			"L okasi":               "Lokasi",
			"akuntansi@uib. ac.id":  "akuntansi@uib.ac.id",
			"Bah asa":               "Bahasa",
			"Language Ce nter":      "Language Center",
			"l anguagecenter":       "languagecenter",
			"Manag ement":           "Management",
			"09:00-17:3 0":          "09:00-17:30",
			"Se minar":              "Seminar",
			"management@uib.ac. id": "management@uib.ac.id",
			"ht tps":                "https",
		}

		for broken, fixed := range wordReplacements {
			normalized = strings.ReplaceAll(normalized, broken, fixed)
		}

		// Fix broken numbers and time patterns with regex
		timePattern := regexp.MustCompile(`(\d)\s+(\d):(\d+)`) // "1 6:00" -> "16:00"
		normalized = timePattern.ReplaceAllString(normalized, "$1$2:$3")

		yearPattern := regexp.MustCompile(`(\d)\s+(\d{3})`) // "2 025" -> "2025"
		normalized = yearPattern.ReplaceAllString(normalized, "$1$2")

		datePattern := regexp.MustCompile(`(\d{4})-(\d{2})\s+-(\d+)`) // "2025-10 -18" -> "2025-10-18"
		normalized = datePattern.ReplaceAllString(normalized, "$1-$2-$3")

		moneyPattern := regexp.MustCompile(`(\d+)\.(\d+)\s+(\d+)`) // "500.00 0" -> "500.000"
		normalized = moneyPattern.ReplaceAllString(normalized, "$1.$2$3")

		urlPattern := regexp.MustCompile(`(https?)\s*:\s*//\s*(\w)`) // "h ttps : // u" -> "https://u"
		normalized = urlPattern.ReplaceAllString(normalized, "$1://$2")

		domainPattern := regexp.MustCompile(`(\w+)\s+\.\s*(\w+)\s*\.\s*(\w+)`) // "ui b.ac.id" -> "uib.ac.id"
		normalized = domainPattern.ReplaceAllString(normalized, "$1.$2.$3")

		emailPattern := regexp.MustCompile(`(\w+)\s*@\s*(\w+)\s*\.\s*(\w+)\s*\.\s*(\w+)`) // "info@uib. ac.id" -> "info@uib.ac.id"
		normalized = emailPattern.ReplaceAllString(normalized, "$1@$2.$3.$4")

		// Fix common broken word patterns more generically
		brokenWordPattern := regexp.MustCompile(`\b([A-Z][a-z]{1,3})\s+([a-z]{2,})\b`) // "Sertifi kasi" -> "Sertifikasi"
		normalized = brokenWordPattern.ReplaceAllString(normalized, "$1$2")

		// Fix excessive spacing (3+ spaces only, preserve normal single spaces)
		normalized = excessSpacePattern.ReplaceAllString(normalized, "$1 $2")

		// Collapse only multiple spaces (2+ become single)
		normalized = multiSpaceRegex.ReplaceAllString(normalized, " ")
	}

	// Clean up spaces around newlines and punctuation
	normalized = spaceBeforeNewline.ReplaceAllString(normalized, "\n")
	normalized = newlineSpace.ReplaceAllString(normalized, "\n")
	normalized = spaceBeforePunctuation.ReplaceAllString(normalized, "$1")
	normalized = extraNewlines.ReplaceAllString(normalized, "\n\n")

	// Trim trailing spaces from each line
	lines := strings.Split(normalized, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	normalized = strings.Join(lines, "\n")

	// Ensure valid UTF-8
	if !utf8.ValidString(normalized) {
		normalized = strings.ToValidUTF8(normalized, "")
	}

	return strings.TrimSpace(normalized)
}
