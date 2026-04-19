package db

// WordRow represents a row in the words table.
type WordRow struct {
	ID                int64
	Word              string
	PartOfSpeech      string
	Article           string
	Definition        string
	EnglishDefinition string
	Example           string
	English           string
	TargetTranslation string
	Notes             string
	Connotation       string
	Register          string
	Collocations      string
	ContrastiveNotes  string
	SecondaryMeanings string
	Tags              string
	SourceLanguage    string
	TargetLanguage    string
	Difficulty        string
	CreatedAt         string
	UpdatedAt         string
}

// ExpressionRow represents a row in the expressions table.
type ExpressionRow struct {
	ID                int64
	Expression        string
	Definition        string
	EnglishDefinition string
	Example           string
	English           string
	TargetTranslation string
	Notes             string
	Connotation       string
	Register          string
	ContrastiveNotes  string
	Tags              string
	SourceLanguage    string
	TargetLanguage    string
	Difficulty        string
	CreatedAt         string
	UpdatedAt         string
}

// FlashcardItem is a unified view of a word or expression entry used for flashcard display.
type FlashcardItem struct {
	ID                int64  `json:"id"`
	Type              string `json:"type"`
	Text              string `json:"text"`
	Definition        string `json:"definition"`
	English           string `json:"english"`
	TargetTranslation string `json:"target_translation"`
	Difficulty        string `json:"difficulty"`
}
