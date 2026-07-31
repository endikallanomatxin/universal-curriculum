# Unit content

## Storage and publication

The current unit-content representation is one non-empty Markdown document per
unit. Content changes are explicit `update_content` proposal changes and the
published `units.content` value is rebuilt from accepted proposal history with
the rest of the curriculum projection.

Units intentionally have no separate short-description field. Their concise
name identifies them in the graph, and their document provides the complete
learning material.

## Rendering

The server renders GitHub-Flavoured Markdown with Goldmark. The client enhances
the resulting document with vendored dependencies:

- KaTeX renders inline and display LaTeX;
- Highlight.js highlights fenced code blocks and their declared language is
  shown inside the block.

Enhancement runs after the initial page load and after HTMX swaps.

## Embedded documents

Ordinary raw HTML is not rendered. Interactive HTML, CSS and JavaScript must be
placed in a standard `<iframe srcdoc="…"></iframe>` element. The renderer wraps
that document in a restrictive content-security policy and forces
`sandbox="allow-scripts"` without platform-origin, cookie, parent-DOM or network
access.

External iframe sources are rejected except for HTTPS `/embed/` URLs on the
official YouTube and YouTube No-Cookie hosts. Those frames receive only the
additional sandbox and feature permissions required for video playback.

## Editing

Curriculum Modification shows the published rendered document and can replace
it in place with a source editor. Renames and content edits are idempotent
within a draft: submitting the published value removes the pending change, and
submitting another value replaces the existing change of the same kind.

Opening a unit with a pending `update_content` change renders its content panel
directly as a server-rendered word-and-punctuation diff between the stored
previous and resulting Markdown. This source diff is the initial view and can
be switched locally to a rendered diff. Unchanged Markdown blocks appear once;
changed regions render their previous and proposed versions independently so
structured content such as formulas and code remains valid in both. The source
comparison falls back to whole-document replacement for unusually large inputs.
Units without a pending content change continue to render their Markdown normally.
