package catalog

import (
	"bytes"
	"os"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
)

// DecodeText converts raw text bytes (such as idgames .txt and README files) into valid UTF-8,
// normalizing line endings and automatically detecting DOS Code Page 437 (ASCII and ANSI art),
// Windows-1252, or standard UTF-8.
func DecodeText(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}

	normalized := bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))
	normalized = bytes.ReplaceAll(normalized, []byte("\r"), []byte("\n"))

	if utf8.Valid(normalized) {
		text := string(normalized)
		hasAccidentalArabic := false
		for _, r := range text {
			if r >= 0x0600 && r <= 0x08FF {
				hasAccidentalArabic = true
				break
			}
		}
		if !hasAccidentalArabic {
			return text
		}
	}

	cpScore := 0
	winScore := 0
	for _, b := range normalized {
		switch {
		case b >= 0xB0 && b <= 0xDF:
			cpScore++
		case b == 0x81 || b == 0x8D || b == 0x8F || b == 0x90 || b == 0x9D:
			cpScore++
		case b >= 0x80 && b <= 0x9F:
			winScore++
		}
	}

	if cpScore > winScore {
		if decoded, err := charmap.CodePage437.NewDecoder().Bytes(normalized); err == nil {
			return string(decoded)
		}
	}

	if decoded, err := charmap.Windows1252.NewDecoder().Bytes(normalized); err == nil {
		return string(decoded)
	}

	return string(normalized)
}

// ReadReadme reads and decodes an accompanying documentation file, converting CP437 or Windows-1252 to UTF-8
// and normalizing carriage returns.
func ReadReadme(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return DecodeText(raw), nil
}
