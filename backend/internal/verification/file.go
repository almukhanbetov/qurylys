package verification

import (
	"bytes"
	"image"
	_ "image/jpeg" // registers the decoder
	_ "image/png"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxPixels = 60_000_000

// detectType decides by content only; the name and the declared type are ignored.
func detectType(data []byte) (string, error) {
	if bytes.HasPrefix(data, []byte("%PDF-")) {
		tail := data[max(0, len(data)-2048):]
		if bytes.Contains(tail, []byte("%%EOF")) {
			return "application/pdf", nil
		}
		return "", errFileType
	}
	// Only the header is decoded, so a small file cannot expand in memory.
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > maxPixels/cfg.Height {
		return "", errFileType
	}
	switch {
	case format == "jpeg" && bytes.Contains(data[max(0, len(data)-64):], []byte{0xFF, 0xD9}):
		return "image/jpeg", nil
	case format == "png" && bytes.HasSuffix(data, []byte("IEND\xaeB`\x82")):
		return "image/png", nil
	}
	return "", errFileType
}

var extensions = map[string]string{"application/pdf": ".pdf", "image/jpeg": ".jpg", "image/png": ".png"}

// cleanName keeps a readable base name and forces the extension of the real type.
func cleanName(name, contentType string) string {
	name = filepath.Base(strings.ReplaceAll(strings.ToValidUTF8(name, ""), `\`, "/"))
	name = strings.TrimSuffix(name, filepath.Ext(name))
	name = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune(" .-_()№", r) {
			return r
		}
		return -1
	}, name)
	name = strings.Trim(name, " .")
	for utf8.RuneCountInString(name) > 80 {
		_, n := utf8.DecodeLastRuneInString(name)
		name = name[:len(name)-n]
	}
	if name == "" {
		name = "document"
	}
	return name + extensions[contentType]
}
