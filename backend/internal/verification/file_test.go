package verification

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/jpeg"
	"image/png"
	"testing"
)

func TestDetectType(t *testing.T) {
	var p, j bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	_ = png.Encode(&p, img)
	_ = jpeg.Encode(&j, img, nil)
	pdf := []byte("%PDF-1.7\n1 0 obj\nendobj\n%%EOF\n")
	for want, data := range map[string][]byte{"image/png": p.Bytes(), "image/jpeg": j.Bytes(), "application/pdf": pdf} {
		if got, err := detectType(data); err != nil || got != want {
			t.Errorf("%s: %q %v", want, got, err)
		}
	}
	bomb := bytes.Clone(p.Bytes())
	binary.BigEndian.PutUint32(bomb[16:], 100_000) // declared width and height
	binary.BigEndian.PutUint32(bomb[20:], 100_000)
	for name, data := range map[string][]byte{
		"html": []byte("<html><script>alert(1)</script>"), "svg": []byte("<svg xmlns='http://www.w3.org/2000/svg'/>"),
		"exe": []byte("MZ\x90\x00"), "zip": []byte("PK\x03\x04"), "empty": {}, "gif": []byte("GIF89a\x01\x00\x01\x00"),
		"truncated pdf": []byte("%PDF-1.7\n"), "pdf not at start": append([]byte("<script>"), pdf...),
		"png with html": append(bytes.Clone(p.Bytes()), "<script>alert(1)</script>"...), "png header only": p.Bytes()[:20], "png bomb": bomb,
	} {
		if got, err := detectType(data); err == nil {
			t.Errorf("%s accepted as %s", name, got)
		}
	}
}

func TestCleanName(t *testing.T) {
	for in, want := range map[string]string{
		"../../etc/passwd": "passwd.pdf", `C:\x\Устав №1.exe`: "Устав №1.pdf", "a\r\nX: y\".pdf": "aX y.pdf",
		"": "document.pdf", "....": "document.pdf", "Жарғы.PDF": "Жарғы.pdf",
	} {
		if got := cleanName(in, "application/pdf"); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}
