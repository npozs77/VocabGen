package parsing

import "strings"

// nlPOSCanonical maps lowercased LLM-generated Dutch POS labels to their
// canonical abbreviation. Applied only when source language is Dutch.
var nlPOSCanonical = map[string]string{
	// wederkerend werkwoord → wed. ww.
	"wederkerend werkwoord":   "wed. ww.",
	"ww (wederkerend)":        "wed. ww.",
	"werkwoord (wederkerend)": "wed. ww.",
	"wed. ww":                 "wed. ww.",
	"wed.ww.":                 "wed. ww.",
	"wed.ww":                  "wed. ww.",

	// scheidbaar werkwoord → sch. ww.
	"scheidbaar werkwoord": "sch. ww.",
	"scheidbaar ww.":       "sch. ww.",
	"scheidbaar ww":        "sch. ww.",
	"ww. (scheidbaar)":     "sch. ww.",
	"ww (scheidbaar)":      "sch. ww.",
	"sch. ww":              "sch. ww.",
	"sch.ww.":              "sch. ww.",
	"sch.ww":               "sch. ww.",

	// sterk werkwoord
	"sterk werkwoord": "ww. (sterk)",
	"ww (sterk)":      "ww. (sterk)",

	// werkwoord → ww.
	"werkwoord": "ww.",
	"ww":        "ww.",
	"werkw.":    "ww.",
	"werkw":     "ww.",

	// werkwoordelijke uitdrukking
	"werkwoordelijke uitdrukking": "ww. uitdr.",
	"ww. uitdr":                   "ww. uitdr.",
	"ww uitdr.":                   "ww. uitdr.",

	// zelfstandig naamwoord → zn. (all gender/number variants collapse)
	"zelfst. nw.":           "zn.",
	"zelfst.nw.":            "zn.",
	"zelfst. nw":            "zn.",
	"zelfstandig naamwoord": "zn.",
	"naamwoord":             "zn.",
	"znw.":                  "zn.",
	"znw":                   "zn.",
	"zn":                    "zn.",
	"nw.":                   "zn.",
	"nw":                    "zn.",
	"zn (de)":               "zn.",
	"zn. (de)":              "zn.",
	"zelfst. nw. (de)":      "zn.",
	"zn (het)":              "zn.",
	"zn. (het)":             "zn.",
	"zelfst. nw. (het)":     "zn.",
	"zelfst. nw. (mv.)":     "zn.",
	"zn (mv.)":              "zn.",
	"zn. (mv.)":             "zn.",
	"znw. (mv.)":            "zn.",
	"znw. (o.)":             "zn.",
	"zn (o.)":               "zn.",
	"zn. (o.)":              "zn.",
	"zelfst. nw. (o.)":      "zn.",

	// bijvoeglijk naamwoord → bijv. nw.
	"bijvoeglijk naamwoord": "bijv. nw.",
	"bijv.nw.":              "bijv. nw.",
	"bijv.nw":               "bijv. nw.",
	"bijv. nw":              "bijv. nw.",
	"bn.":                   "bijv. nw.",
	"bn":                    "bijv. nw.",

	// bijwoord → bijw.
	"bijwoord": "bijw.",
	"bijw":     "bijw.",
	"bw.":      "bijw.",
	"bw":       "bijw.",

	// bijw./bijv. nw. (dual — kept as-is)
	"bijw./bn.":      "bijw./bijv. nw.",
	"bijw./bn":       "bijw./bijv. nw.",
	"bijw/bn.":       "bijw./bijv. nw.",
	"bijw./bijv.nw.": "bijw./bijv. nw.",
	"bijw./bijv. nw": "bijw./bijv. nw.",

	// uitdrukking → uitdr.
	"uitdrukking": "uitdr.",
	"uitdr":       "uitdr.",
}

// dutchLangCodes are the source language codes that trigger Dutch POS normalization.
var dutchLangCodes = map[string]bool{
	"nl":    true,
	"dutch": true,
	"nld":   true,
}

// NormalizePOS maps an LLM-generated part-of-speech label to its canonical
// abbreviation. Only applies Dutch normalization when sourceLang is Dutch.
// Returns the input unchanged for non-Dutch languages or unknown labels.
func NormalizePOS(pos, sourceLang string) string {
	trimmed := strings.TrimSpace(pos)
	if trimmed == "" {
		return trimmed
	}
	if !dutchLangCodes[strings.ToLower(strings.TrimSpace(sourceLang))] {
		return trimmed
	}
	key := strings.ToLower(trimmed)
	if canonical, ok := nlPOSCanonical[key]; ok {
		return canonical
	}
	return trimmed
}
