#!/usr/bin/env bash
# update-seo.sh — Update SEO metadata for VocabGen repository
# Usage: ./scripts/update-seo.sh
#
# Updates: GitHub repo description, topics, homepage URL
# Requires: gh CLI authenticated with repo permissions

set -euo pipefail

REPO="npozs77/VocabGen"
PAGES_URL="https://npozs77.github.io/VocabGen/"

echo "=== Updating repository description ==="
gh repo edit "$REPO" --description \
  "Open-source vocabulary web app and flashcard tool for language learners. Look up words, batch-process CSV lists, and study with flashcards in your browser. Powered by LLM (OpenAI, Anthropic, Bedrock, Ollama). Single binary, zero setup."

echo "=== Setting homepage URL ==="
gh repo edit "$REPO" --homepage "$PAGES_URL"

echo "=== Adding topics ==="
TOPICS=(
  vocabulary-web-app
  vocabulary-generator
  flashcard-app
  language-learning
  study-tool
  llm
  golang
  openai
  anthropic
  aws-bedrock
  ollama
  htmx
  sqlite
  csv-tool
  vocabulary
  flashcards
  education
)

for topic in "${TOPICS[@]}"; do
  echo "  Adding topic: $topic"
  gh repo edit "$REPO" --add-topic "$topic" 2>/dev/null || true
done

echo "=== Updating sitemap lastmod date ==="
TODAY=$(date -u +%Y-%m-%d)
if [[ "$OSTYPE" == "darwin"* ]]; then
  sed -i '' "s|<lastmod>[0-9-]*</lastmod>|<lastmod>${TODAY}</lastmod>|" docs/sitemap.xml
else
  sed -i "s|<lastmod>[0-9-]*</lastmod>|<lastmod>${TODAY}</lastmod>|" docs/sitemap.xml
fi

echo ""
echo "Done. Next steps:"
echo "  1. Commit and push docs/index.html, docs/sitemap.xml, docs/robots.txt, docs/llms.txt"
echo "  2. Enable GitHub Pages: Settings → Pages → Source: Deploy from branch → main → /docs"
echo "     Or via API: gh api repos/$REPO/pages --method POST --field source='{\"branch\":\"main\",\"path\":\"/docs\"}'"
echo "  3. Submit sitemap to Google Search Console: https://search.google.com/search-console"
