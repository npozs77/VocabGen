// Package language provides prompt templates, JSON validation, and a language
// registry for the vocabulary generator. All templates are language-agnostic,
// parameterized by {source_language}.
package language

// SupportedLanguages maps language codes to full names.
// Used for both source and target language resolution.
var SupportedLanguages = map[string]string{
	"nl": "Dutch",
	"hu": "Hungarian",
	"it": "Italian",
	"ru": "Russian",
	"en": "English",
	"de": "German",
	"fr": "French",
	"es": "Spanish",
	"pt": "Portuguese",
	"pl": "Polish",
	"tr": "Turkish",
}

// ResolveLanguageName maps a language code to its full name.
// Returns the input as-is for unknown codes or names.
func ResolveLanguageName(code string) string {
	if name, ok := SupportedLanguages[code]; ok {
		return name
	}
	return code
}
