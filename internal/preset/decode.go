package preset

import (
	"bytes"
	"os"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
)

// DecodeText converts raw text bytes (e.g. from idgames .txt / README files) into valid UTF-8,
// normalizing line endings and automatically detecting DOS Code Page 437 (ASCII/ANSI art),
// Windows-1252, or standard UTF-8.
func DecodeText(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}

	// Normalize CRLF to LF and bare CR to LF
	normalized := bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))
	normalized = bytes.ReplaceAll(normalized, []byte("\r"), []byte("\n"))

	// Check if already valid UTF-8 without accidental Arabic code points from CP437 blocks
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

	// Count CP437 box-drawing, shading, and block characters vs Windows-1252 punctuation
	cpScore := 0
	winScore := 0
	for _, b := range normalized {
		switch {
		case b >= 0xB0 && b <= 0xDF:
			// Box-drawing, shading blocks (░▒▓█▄▀), and borders (║═│─) in CP437
			cpScore++
		case (b >= 0x80 && b <= 0x9F):
			// Smart quotes, dashes, ellipsis, bullet, trademark in Windows-1252
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

	if decoded, err := charmap.ISO8859_1.NewDecoder().Bytes(normalized); err == nil {
		return string(decoded)
	}

	// Fallback to CP437 as it maps all 256 byte values
	if decoded, err := charmap.CodePage437.NewDecoder().Bytes(normalized); err == nil {
		return string(decoded)
	}

	return string(normalized)
}

// ReadReadme reads and decodes an accompanying documentation file, converting CP437/Windows-1252 to UTF-8
// and normalizing carriage returns.
func ReadReadme(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return DecodeText(raw), nil
}
