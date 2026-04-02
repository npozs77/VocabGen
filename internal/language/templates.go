package language

import (
	"fmt"
	"strings"
)

// WordsTemplate is the unified prompt template for word lookups.
// Placeholders: {source_language}, {word}, {context}, {target_language_name}.
const WordsTemplate = `You are helping to build a vocabulary list for {source_language} learners at B2–C1 level.
Return your answer ONLY as a single JSON object, no extra text.

Given:
- {source_language} word or phrase: "{word}"
- Context sentence: "{context}"
- Context: This word appears in a {source_language} textbook for advanced learners (B2 → C1).

If a context sentence is provided, use it to determine the correct sense, connotation, and register of the word.
If no context sentence is provided, infer the most typical B2–C1 textbook sense. If the word is highly polysemous, note this in the connotation field.

CORE RULES:
- Preserve the connotation, register, and tone of the source word in ALL translation fields.
- Do NOT default to the most common or simplest dictionary translation when a more connotation-accurate alternative exists.

DECISION RUBRIC (follow this priority order):
1. Prefer the translation that best preserves the connotation of the source word, even if less common.
2. When two translations preserve connotation equally, prefer the one that matches the register (formality level).
3. When connotation and register are equal, prefer the more widely understood translation.

Requirements for the JSON fields:
- "word": repeat the canonical form (infinitive for verbs, singular for nouns)
- "type": {source_language} part-of-speech label using {source_language}'s own grammatical terminology, abbreviated where conventional
- "article": article/gender marker if applicable, otherwise "—"
- "definition": definition in {source_language}
- "english_definition": concise English-language explanation of the word's meaning
- "example": example sentence in {source_language}
- "english": object with "primary" (best connotation-preserving English translation) and "alternatives" (semicolon-separated alternatives)
- "target_translation": object with "primary" (best connotation-preserving {target_language_name} translation) and "alternatives" (semicolon-separated alternatives)
- "notes": optional notes — include connotation notes, register label (in {source_language}'s terminology), and tone indicators when relevant
- "connotation": short description of the emotional or evaluative association of the source word
- "register": register label in {source_language}'s own terminology (e.g., formal/informal/literary/colloquial/neutral in the source language)
- "collocations": list two to four common collocations separated by semicolons
- "contrastive_notes": name one or two near-synonyms and briefly explain how they differ
- "secondary_meanings": list additional distinct meanings separated by semicolons, or leave empty

Now process this word:
"{word}"`

// ExpressionsTemplate is the unified prompt template for expression lookups.
// Placeholders: {source_language}, {expression}, {context}, {target_language_name}.
const ExpressionsTemplate = `You are helping to build a vocabulary list for {source_language} learners at B2–C1 level.
Return your answer ONLY as a single JSON object, no extra text.

Given:
- {source_language} expression: "{expression}"
- Context sentence: "{context}"
- Context: This expression appears in a {source_language} textbook for advanced learners (B2 → C1).

If a context sentence is provided, use it to determine the correct sense, connotation, and register of the expression.
If no context sentence is provided, infer the most typical B2–C1 textbook sense. If the expression is highly context-dependent, note this in the connotation field.

CORE RULES:
- Preserve the connotation, register, and tone of the source word in ALL translation fields.
- Do NOT default to the most common or simplest dictionary translation when a more connotation-accurate alternative exists.

DECISION RUBRIC (follow this priority order):
1. Prefer the translation that best preserves the connotation of the source word, even if less common.
2. When two translations preserve connotation equally, prefer the one that matches the register (formality level).
3. When connotation and register are equal, prefer the more widely understood translation.

Requirements for the JSON fields:
- "expression": repeat the expression
- "definition": definition in {source_language}
- "english_definition": concise English-language explanation of the expression's meaning
- "example": example sentence in {source_language}
- "english": object with "primary" (best connotation-preserving English equivalent) and "alternatives" (semicolon-separated alternatives)
- "target_translation": object with "primary" (best connotation-preserving {target_language_name} equivalent) and "alternatives" (semicolon-separated alternatives)
- "notes": optional notes — include connotation notes, register label (in {source_language}'s terminology), and tone indicators when relevant
- "connotation": short description of the emotional or evaluative association of the source expression
- "register": register label in {source_language}'s own terminology
- "contrastive_notes": name one or two near-synonyms and briefly explain how they differ

Now process this expression:
"{expression}"`

// BuildPrompt constructs a complete prompt from template + parameters.
// sourceLang is a language code or name; mode is "words" or "expressions";
// token is the word/expression; context is an optional context sentence;
// targetLang is the target language code or name.
func BuildPrompt(sourceLang, mode, token, context, targetLang string) (string, error) {
	langName := ResolveLanguageName(sourceLang)
	targetName := ResolveLanguageName(targetLang)

	var tmpl string
	switch mode {
	case "words":
		tmpl = WordsTemplate
	case "expressions":
		tmpl = ExpressionsTemplate
	default:
		return "", fmt.Errorf("invalid mode: %q (must be \"words\" or \"expressions\")", mode)
	}

	r := strings.NewReplacer(
		"{source_language}", langName,
		"{word}", token,
		"{expression}", token,
		"{context}", context,
		"{target_language_name}", targetName,
	)
	return r.Replace(tmpl), nil
}
