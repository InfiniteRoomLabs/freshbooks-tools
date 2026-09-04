#!/usr/bin/env bash
# Regenerates site/docs/ (Docusaurus content) from README.md and the
# published docs/*.md guides. site/docs/ is gitignored and never
# hand-edited -- this script is the only thing that writes to it.
#
# For each source file: prepend Docusaurus front matter (title,
# sidebar_position, slug, mdx.format: md so the guides -- which contain
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

# name:title:position:source-path:edit-path
pages=(
  "index:freshbooks-tools:0:README.md:README.md"
  "getting-started:Getting started:1:docs/getting-started.md:docs/getting-started.md"
  "authentication:Authentication:2:docs/authentication.md:docs/authentication.md"
  "library:Library:3:docs/library.md:docs/library.md"
  "mcp:MCP server:4:docs/mcp.md:docs/mcp.md"
  "cli:CLI reference:5:docs/cli.md:docs/cli.md"
  "building:Building:6:docs/building.md:docs/building.md"
  "agentic-transformation:Agentic transformation:7:docs/agentic-transformation.md:docs/agentic-transformation.md"
)

# Alternation of the published slugs, derived from the table above so the
# link rewrite can never name a page the site does not build. `index` is
# excluded: it is README.md, not a docs/<name>.md anyone links to.
slugs=""
for page in "${pages[@]}"; do
  IFS=':' read -r name _ <<<"$page"
  if [ "$name" != "index" ]; then
    slugs="${slugs:+$slugs|}$name"
  fi
done

for page in "${pages[@]}"; do
  IFS=':' read -r name title position src edit_path <<<"$page"
  slug="/$name"
  if [ "$name" = "index" ]; then
    slug="/"
  fi

  out="$site_docs/$name.md"
  {
    echo "---"
    echo "title: \"$title\""
    echo "sidebar_position: $position"
    echo "slug: \"$slug\""
    echo "mdx:"
    echo "  format: md"
    echo "custom_edit_url: \"$edit_base/$edit_path\""
    echo "---"
    echo
    # Rewrite (docs/<x>.md[#frag]) markdown links to (/<x>[#frag]) slugs.
    sed -E "s%\(docs/($slugs)\.md(#[^)]*)?\)%(/\1\2)%g" "$repo_root/$src"
  } >"$out"
done

echo "site-sync: regenerated site/docs/ (${#pages[@]} pages)"
