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
	CreatedAt         string
	UpdatedAt         string
}
