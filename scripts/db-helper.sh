#!/bin/bash
# db-helper.sh — Mass update/insert helper for vocabgen SQLite database
# Usage:
#   ./scripts/db-helper.sh update    — mass update a column with before/after preview
#   ./scripts/db-helper.sh add-tag   — add a tag to matching entries
#   ./scripts/db-helper.sh list-pos  — show distinct POS values (useful for normalization)

set -euo pipefail

DB="${VOCABGEN_DB:-$HOME/.vocabgen/vocabgen.db}"

if [ ! -f "$DB" ]; then
    echo "Error: Database not found at $DB"
    echo "Set VOCABGEN_DB env var or ensure ~/.vocabgen/vocabgen.db exists"
    exit 1
fi

usage() {
    echo "Usage: $0 <command>"
    echo ""
    echo "Commands:"
    echo "  update     Mass update a column value (with before/after preview)"
    echo "  add-tag    Add a tag to entries matching a filter"
    echo "  list-pos   Show distinct part_of_speech values per language"
    echo ""
    echo "Environment:"
    echo "  VOCABGEN_DB  Path to database (default: ~/.vocabgen/vocabgen.db)"
}

cmd_list_pos() {
    echo "=== Distinct POS values in words table ==="
    sqlite3 "$DB" "SELECT source_language, part_of_speech, COUNT(*) as count FROM words GROUP BY source_language, part_of_speech ORDER BY source_language, count DESC;"
    echo ""
    echo "=== Distinct POS values in expressions table ==="
    sqlite3 "$DB" "SELECT source_language, COUNT(*) as count FROM expressions GROUP BY source_language ORDER BY source_language, count DESC;"
}

cmd_update() {
    echo "=== Mass Update ==="
    echo ""

    # Table
    read -p "Table (words/expressions) [words]: " table
    table="${table:-words}"
    if [[ "$table" != "words" && "$table" != "expressions" ]]; then
        echo "Error: table must be 'words' or 'expressions'"
        exit 1
    fi

    # Column
    echo ""
    if [ "$table" = "words" ]; then
        echo "Available columns: word, part_of_speech, article, definition, english_definition,"
        echo "  example, english, target_translation, notes, connotation, register,"
        echo "  collocations, contrastive_notes, secondary_meanings, tags, source_language, target_language"
    else
        echo "Available columns: expression, definition, english_definition, example,"
        echo "  english, target_translation, notes, connotation, register,"
        echo "  contrastive_notes, tags, source_language, target_language"
    fi
    echo ""
    read -p "Column to update: " column

    # Filter
    echo ""
    read -p "WHERE clause (e.g., source_language = 'nl' AND part_of_speech = 'werkwoord'): " where_clause
    if [ -z "$where_clause" ]; then
        echo "Error: WHERE clause is required (safety measure)"
        exit 1
    fi

    # New value
    read -p "New value: " new_value

    # Preview — before
    echo ""
    echo "=== BEFORE (matching rows) ==="
    if [ "$table" = "words" ]; then
        sqlite3 -header -column "$DB" "SELECT id, word, $column FROM $table WHERE $where_clause;"
    else
        sqlite3 -header -column "$DB" "SELECT id, expression, $column FROM $table WHERE $where_clause;"
    fi

    count=$(sqlite3 "$DB" "SELECT COUNT(*) FROM $table WHERE $where_clause;")
    echo ""
    echo "Rows to update: $count"

    # Confirm
    read -p "Apply update? (y/N): " confirm
    if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
        echo "Aborted."
        exit 0
    fi

    # Execute
    sqlite3 "$DB" "UPDATE $table SET $column = '$new_value', updated_at = datetime('now') WHERE $where_clause;"

    # Preview — after
    echo ""
    echo "=== AFTER ==="
    if [ "$table" = "words" ]; then
        sqlite3 -header -column "$DB" "SELECT id, word, $column FROM $table WHERE $where_clause OR $column = '$new_value';" 2>/dev/null || \
        sqlite3 -header -column "$DB" "SELECT id, word, $column FROM $table WHERE $column = '$new_value';"
    else
        sqlite3 -header -column "$DB" "SELECT id, expression, $column FROM $table WHERE $where_clause OR $column = '$new_value';" 2>/dev/null || \
        sqlite3 -header -column "$DB" "SELECT id, expression, $column FROM $table WHERE $column = '$new_value';"
    fi

    echo ""
    echo "Done. Updated $count rows."
}

cmd_add_tag() {
    echo "=== Add Tag ==="
    echo ""

    read -p "Table (words/expressions) [words]: " table
    table="${table:-words}"
    if [[ "$table" != "words" && "$table" != "expressions" ]]; then
        echo "Error: table must be 'words' or 'expressions'"
        exit 1
    fi

    read -p "Tag to add: " new_tag
    if [ -z "$new_tag" ]; then
        echo "Error: tag cannot be empty"
        exit 1
    fi

    echo ""
    read -p "WHERE clause (e.g., source_language = 'nl') or 'all': " where_clause
    if [ "$where_clause" = "all" ]; then
        where_clause="1=1"
    fi
    if [ -z "$where_clause" ]; then
        echo "Error: WHERE clause is required"
        exit 1
    fi

    count=$(sqlite3 "$DB" "SELECT COUNT(*) FROM $table WHERE $where_clause;")
    echo "Rows matching: $count"

    # Show how many already have this tag
    already=$(sqlite3 "$DB" "SELECT COUNT(*) FROM $table WHERE ($where_clause) AND (',' || tags || ',') LIKE '%,$new_tag,%';")
    echo "Already tagged with '$new_tag': $already"
    echo "Will add tag to: $((count - already)) rows"

    read -p "Apply? (y/N): " confirm
    if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
        echo "Aborted."
        exit 0
    fi

    # Add tag: if tags is empty, set to new_tag; otherwise append with comma
    sqlite3 "$DB" "
        UPDATE $table
        SET tags = CASE
            WHEN tags IS NULL OR tags = '' THEN '$new_tag'
            WHEN (',' || tags || ',') LIKE '%,$new_tag,%' THEN tags
            ELSE tags || ',$new_tag'
        END,
        updated_at = datetime('now')
        WHERE $where_clause;
    "

    echo "Done. Tag '$new_tag' added."
}

# Main
case "${1:-}" in
    update)    cmd_update ;;
    add-tag)   cmd_add_tag ;;
    list-pos)  cmd_list_pos ;;
    *)         usage ;;
esac
