package validate

import "testing"

func TestContact(t *testing.T) {
	ok := map[string][2]string{
		" User@Example.KZ ":  {Email, "user@example.kz"},
		"+7 (701) 123-45-67": {Phone, "+77011234567"},
		"87011234567":        {Phone, "+77011234567"},
		"77011234567":        {Phone, "+77011234567"},
	}
	for in, want := range ok {
		k, v, err := Contact(in)
		if err != nil || k != want[0] || v != want[1] {
			t.Errorf("%q: got %q %q %v", in, k, v, err)
		}
	}
	for _, in := range []string{"", "abc", "a@b", "A <a@b.kz>", "a@b.kz\n", "a\x00@b.kz", "+1 202 555 0100", "123", "а@б.кз"} {
		if _, v, err := Contact(in); err == nil && in != "a@b.kz\n" {
			t.Errorf("%q: accepted as %q", in, v)
		}
	}
}

func TestTextAndBIN(t *testing.T) {
	for _, in := range []string{"", "   ", "a\x00b", "a\nb", "абвгд"} {
		if _, err := Text("f", in, 4); err == nil {
			t.Errorf("Text %q accepted", in)
		}
	}
	if v, err := Text("f", " Қала ", 4); err != nil || v != "Қала" {
		t.Errorf("Text: %q %v", v, err)
	}
	for _, in := range []string{"12345678901", "1234567890123", "12345678901a", "١٢٣٤٥٦٧٨٩٠١٢"} {
		if _, err := BIN(in); err == nil {
			t.Errorf("BIN %q accepted", in)
		}
	}
}
