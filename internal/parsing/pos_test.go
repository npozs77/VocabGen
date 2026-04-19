package parsing

import "testing"

func TestNormalizePOS_Dutch(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// werkwoord variants
		{"werkwoord", "ww."},
		{"ww", "ww."},
		{"ww.", "ww."},
		{"werkw.", "ww."},
		{"Werkwoord", "ww."},

		// scheidbaar
		{"scheidbaar werkwoord", "sch. ww."},
		{"scheidbaar ww.", "sch. ww."},
		{"ww. (scheidbaar)", "sch. ww."},

		// wederkerend
		{"wederkerend werkwoord", "wed. ww."},
		{"ww (wederkerend)", "wed. ww."},

		// zelfstandig naamwoord — all collapse to zn.
		{"naamwoord", "zn."},
		{"zn", "zn."},
		{"zn.", "zn."},
		{"zelfst. nw.", "zn."},
		{"znw.", "zn."},
		{"nw.", "zn."},
		{"zn (de)", "zn."},
		{"zn. (de)", "zn."},
		{"zn. (het)", "zn."},
		{"zn. (mv.)", "zn."},
		{"znw. (o.)", "zn."},

		// bijvoeglijk naamwoord
		{"bijv.nw.", "bijv. nw."},
		{"bijv. nw.", "bijv. nw."},
		{"bn.", "bijv. nw."},

		// bijwoord
		{"bijwoord", "bijw."},
		{"bijw.", "bijw."},

		// dual — kept
		{"bijw./bn.", "bijw./bijv. nw."},

		// uitdrukking
		{"uitdrukking", "uitdr."},

		// passthrough unknown
		{"voorzetsel", "voorzetsel"},

		// whitespace
		{"  ww  ", "ww."},
		{"", ""},

		// already canonical
		{"ww.", "ww."},
		{"zn.", "zn."},
		{"bijv. nw.", "bijv. nw."},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizePOS(tt.input, "nl")
			if got != tt.want {
				t.Errorf("NormalizePOS(%q, \"nl\") = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizePOS_NonDutch_Passthrough(t *testing.T) {
	tests := []struct {
		pos        string
		sourceLang string
	}{
		{"werkwoord", "de"},
		{"naamwoord", "fr"},
		{"verb", "en"},
		{"ww.", "es"},
	}

	for _, tt := range tests {
		t.Run(tt.sourceLang+"/"+tt.pos, func(t *testing.T) {
			got := NormalizePOS(tt.pos, tt.sourceLang)
			if got != tt.pos {
				t.Errorf("NormalizePOS(%q, %q) = %q, want passthrough %q", tt.pos, tt.sourceLang, got, tt.pos)
			}
		})
	}
}

func TestNormalizePOS_DutchLangVariants(t *testing.T) {
	// All Dutch language code variants should trigger normalization.
	for _, lang := range []string{"nl", "NL", "dutch", "Dutch", "nld"} {
		t.Run(lang, func(t *testing.T) {
			got := NormalizePOS("werkwoord", lang)
			if got != "ww." {
				t.Errorf("NormalizePOS(\"werkwoord\", %q) = %q, want \"ww.\"", lang, got)
			}
		})
	}
}
