#!/usr/bin/env bash
# Regenerates site/docs/ (Docusaurus content) from README.md and the
# published docs/*.md guides. site/docs/ is gitignored and never
# hand-edited -- this script is the only thing that writes to it.
#
# For each source file: prepend Docusaurus front matter (title, slug,
# mdx.format: md so the guides -- which contain
# `{`/`<` sequences, and docs/cli.md is ~8k generated lines -- parse as
# CommonMark instead of MDX (Docusaurus 3.10 nests this under `mdx:`, not
# a bare top-level `format:` key -- @docusaurus/mdx-loader's
# compileToJSX reads `frontMatter.mdx.format`), and custom_edit_url
# pointing at the real source file so the site's "Edit this page" link
# doesn't point at a gitignored generated copy), then rewrite
# `(docs/<name>.md)` markdown links to the site's own slugs (`(/<name>)`),
# anchors included (`(docs/cli.md#frag)` -> `(/cli#frag)`). Only the pages
# published below are rewritten: a link to an unpublished `docs/*.md` is
# left alone so onBrokenMarkdownLinks: 'throw' names the real source path
# rather than an invented slug.
#
# docs/phases/, docs/progress.md, docs/superpowers/ are process/internal
# and are never synced.

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
site_docs="$repo_root/site/docs"
edit_base="https://github.com/InfiniteRoomLabs/freshbooks-tools/edit/main"

rm -rf "$site_docs"
mkdir -p "$site_docs"

# name|title|source-path. The source path is also what the page's "Edit
# this page" link points at. `|` rather than `:` so a future title may
# contain a colon. Sidebar order lives in site/sidebars.js and nowhere
# else -- a `sidebar_position` front matter key is inert next to an
# explicit sidebar, so this table deliberately does not carry one.
pages=(
  "index|freshbooks-tools|README.md"
  "getting-started|Getting started|docs/getting-started.md"
  "authentication|Authentication|docs/authentication.md"
  "library|Library|docs/library.md"
  "mcp|MCP server|docs/mcp.md"
  "cli|CLI reference|docs/cli.md"
  "building|Building|docs/building.md"
  "agentic-transformation|Agentic transformation|docs/agentic-transformation.md"
)

# Alternation of the published slugs, derived from the table above so the
# link rewrite can never name a page the site does not build. `index` is
# excluded: it is README.md, not a docs/<name>.md anyone links to.
slugs=""
for page in "${pages[@]}"; do
  IFS='|' read -r name _ <<<"$page"
  if [ "$name" != "index" ]; then
    slugs="${slugs:+$slugs|}$name"
  fi
done

for page in "${pages[@]}"; do
  IFS='|' read -r name title src <<<"$page"
  slug="/$name"
  if [ "$name" = "index" ]; then
    slug="/"
  fi

  out="$site_docs/$name.md"
  {
    echo "---"
    echo "title: \"$title\""
    echo "slug: \"$slug\""
    echo "mdx:"
    echo "  format: md"
    echo "custom_edit_url: \"$edit_base/$src\""
    echo "---"
    echo
    # Rewrite (docs/<x>.md[#frag]) markdown links to (/<x>[#frag]) slugs.
    sed -E "s%\(docs/($slugs)\.md(#[^)]*)?\)%(/\1\2)%g" "$repo_root/$src"
  } >"$out"
done

echo "site-sync: regenerated site/docs/ (${#pages[@]} pages)"
