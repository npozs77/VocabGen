#!/usr/bin/env bash
# normalize-pos.sh — Consolidate LLM-generated POS labels to standard abbreviations.
# Usage: ./scripts/normalize-pos.sh [path/to/vocabgen.db]
# Default DB: ~/.vocabgen/vocabgen.db

set -euo pipefail

DB="${1:-$HOME/.vocabgen/vocabgen.db}"

if [ ! -f "$DB" ]; then
  echo "ERROR: database not found at $DB" >&2
  exit 1
fi

echo "=== POS values BEFORE normalization ==="
sqlite3 "$DB" "SELECT part_of_speech, COUNT(*) FROM words GROUP BY part_of_speech ORDER BY COUNT(*) DESC;"
echo ""

# Each UPDATE maps verbose/inconsistent LLM labels to the canonical abbreviation.
# Ordering matters: specific patterns first, then general catch-alls.

sqlite3 "$DB" <<'SQL'
BEGIN;

-- wederkerend werkwoord → wed. ww.
UPDATE words SET part_of_speech = 'wed. ww.'
  WHERE LOWER(part_of_speech) IN (
    'wederkerend werkwoord',
    'ww (wederkerend)',
    'werkwoord (wederkerend)',
    'wed. ww',
    'wed.ww.',
    'wed.ww'
  );

-- scheidbaar werkwoord → sch. ww.
UPDATE words SET part_of_speech = 'sch. ww.'
  WHERE LOWER(part_of_speech) IN (
    'scheidbaar werkwoord',
    'scheidbaar ww.',
    'scheidbaar ww',
    'ww. (scheidbaar)',
    'ww (scheidbaar)',
    'sch. ww',
    'sch.ww.',
    'sch.ww'
  );

-- sterk werkwoord → ww. (sterk)
UPDATE words SET part_of_speech = 'ww. (sterk)'
  WHERE LOWER(part_of_speech) IN (
    'sterk werkwoord',
    'ww (sterk)'
  );

-- werkwoord (all remaining variants) → ww.
UPDATE words SET part_of_speech = 'ww.'
  WHERE LOWER(part_of_speech) IN (
    'werkwoord',
    'ww',
    'werkw.',
    'werkw'
  );

-- werkwoordelijke uitdrukking → ww. uitdr.
UPDATE words SET part_of_speech = 'ww. uitdr.'
  WHERE LOWER(part_of_speech) IN (
    'werkwoordelijke uitdrukking',
    'ww. uitdr',
    'ww uitdr.'
  );

-- zelfstandig naamwoord → zn.
UPDATE words SET part_of_speech = 'zn.'
  WHERE LOWER(part_of_speech) IN (
    'zelfst. nw.',
    'zelfst.nw.',
    'zelfst. nw',
    'zelfstandig naamwoord',
    'naamwoord',
    'znw.',
    'znw',
    'zn',
    'nw.',
    'nw'
  );

-- zn. with gender preserved
UPDATE words SET part_of_speech = 'zn. (de)'
  WHERE LOWER(part_of_speech) IN (
    'zn (de)',
    'zn. (de)',
    'zelfst. nw. (de)'
  );

UPDATE words SET part_of_speech = 'zn. (het)'
  WHERE LOWER(part_of_speech) IN (
    'zn (het)',
    'zn. (het)',
    'zelfst. nw. (het)'
  );

UPDATE words SET part_of_speech = 'zn. (mv.)'
  WHERE LOWER(part_of_speech) IN (
    'zelfst. nw. (mv.)',
    'zn (mv.)',
    'zn. (mv.)',
    'znw. (mv.)'
  );

UPDATE words SET part_of_speech = 'zn. (o.)'
  WHERE LOWER(part_of_speech) IN (
    'znw. (o.)',
    'zn (o.)',
    'zelfst. nw. (o.)'
  );

-- bijvoeglijk naamwoord → bijv. nw.
UPDATE words SET part_of_speech = 'bijv. nw.'
  WHERE LOWER(part_of_speech) IN (
    'bijvoeglijk naamwoord',
    'bijv.nw.',
    'bijv.nw',
    'bijv. nw',
    'bn.',
    'bn'
  );

-- bijwoord → bijw.
UPDATE words SET part_of_speech = 'bijw.'
  WHERE LOWER(part_of_speech) IN (
    'bijwoord',
    'bijw',
    'bw.',
    'bw'
  );

-- bijw./bn. (dual) — normalize spacing
UPDATE words SET part_of_speech = 'bijw./bijv. nw.'
  WHERE LOWER(part_of_speech) IN (
    'bijw./bn.',
    'bijw./bn',
    'bijw/bn.',
    'bijw./bijv.nw.',
    'bijw./bijv. nw'
  );

-- uitdrukking → uitdr.
UPDATE words SET part_of_speech = 'uitdr.'
  WHERE LOWER(part_of_speech) IN (
    'uitdrukking',
    'uitdr'
  );

COMMIT;
SQL

echo "=== POS values AFTER normalization ==="
sqlite3 "$DB" "SELECT part_of_speech, COUNT(*) FROM words GROUP BY part_of_speech ORDER BY COUNT(*) DESC;"
