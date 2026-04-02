// Package output provides field mapping, translation flattening, and Excel export
// for vocabulary entries.
package output

import (
	"bytes"

	"github.com/xuri/excelize/v2"
)

// wordsColumns defines column headers for words mode export.
var wordsColumns = []string{
	"word", "type", "article", "definition", "english_definition",
	"example", "english", "target_translation", "notes", "connotation",
	"register", "collocations", "contrastive_notes", "secondary_meanings", "tags",
}

// expressionsColumns defines column headers for expressions mode export.
var expressionsColumns = []string{
	"expression", "definition", "english_definition", "example", "english",
	"target_translation", "notes", "connotation", "register",
	"contrastive_notes", "tags",
}

// ExportToExcel writes entries to an .xlsx file in memory.
// Mode determines which columns to include ("words" or "expressions").
func ExportToExcel(entries []Entry, mode string) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Sheet1"

	cols := wordsColumns
	if mode == "expressions" {
		cols = expressionsColumns
	}

	// Write headers.
	for i, h := range cols {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	// Write rows.
	for rowIdx, e := range entries {
		row := entryToRow(e, mode)
		for colIdx, val := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			f.SetCellValue(sheet, cell, val)
		}
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
// ExportBothToExcel writes words and expressions to separate sheets in one .xlsx file.
func ExportBothToExcel(words []Entry, expressions []Entry) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	// Words sheet (rename default Sheet1)
	f.SetSheetName("Sheet1", "Words")
	writeSheet(f, "Words", wordsColumns, words, "words")

	// Expressions sheet
	if _, err := f.NewSheet("Expressions"); err != nil {
		return nil, err
	}
	writeSheet(f, "Expressions", expressionsColumns, expressions, "expressions")

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeSheet(f *excelize.File, sheet string, cols []string, entries []Entry, mode string) {
	for i, h := range cols {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}
	for rowIdx, e := range entries {
		row := entryToRow(e, mode)
		for colIdx, val := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			f.SetCellValue(sheet, cell, val)
		}
	}
}

// entryToRow returns the cell values for an Entry in column order.
func entryToRow(e Entry, mode string) []string {
	if mode == "expressions" {
		return []string{
			e.Expression, e.Definition, e.EnglishDefinition, e.Example,
			e.English, e.TargetTranslation, e.Notes, e.Connotation,
			e.Register, e.ContrastiveNotes, e.Tags,
		}
	}
	return []string{
		e.Word, e.Type, e.Article, e.Definition, e.EnglishDefinition,
		e.Example, e.English, e.TargetTranslation, e.Notes, e.Connotation,
		e.Register, e.Collocations, e.ContrastiveNotes, e.SecondaryMeanings, e.Tags,
	}
}
