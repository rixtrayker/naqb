# Book Templates

`nqb` ships with three built-in templates selected during `nqb init`.

---

## Choosing a Template

When you run `nqb init`, the first question asks:

```
Templates:
  1 — Arabic Research (كتاب بحثي) — RTL, Amiri font, scholarly
  2 — CS / Technical Book        — Code blocks, English, precise
  3 — General Book               — Flexible, fill in yourself
```

The template pre-configures:
- `config/rules.yaml` — fonts, RTL, word count targets, callout colors
- `config/prompts/write.md` — writer system prompt
- `config/prompts/qa.md` — QA reviewer system prompt

---

## Template 1 — Arabic Research (`arabic-research`)

**Best for:** Scholarly Arabic books, cultural research, history, philosophy.

| Setting | Value |
|---------|-------|
| Language | `ar` (RTL) |
| Font (body) | Amiri |
| Font (code) | JetBrains Mono |
| Target words/chapter | 3000 |
| PDF engine | XeLaTeX + polyglossia |
| RTL | Yes |

The writer prompt instructs the model to write in Modern Standard Arabic (MSA)
with English technical terms in brackets, use frequent subheadings, and apply
ADHD-friendly callout blocks.

---

## Template 2 — CS / Technical Book (`cs-book`)

**Best for:** Programming books, computer science, software engineering.

| Setting | Value |
|---------|-------|
| Language | `en` |
| Font (body) | IBM Plex Sans |
| Font (code) | JetBrains Mono |
| Code theme | Dracula |
| Target words/chapter | 3000 |
| RTL | No |

The writer prompt instructs the model to include generous code examples,
precise technical language, and ADHD-friendly structure.

---

## Template 3 — General (`general`)

**Best for:** Any book type. No strong defaults — you configure it yourself.

| Setting | Value |
|---------|-------|
| Language | `ar` or `en` |
| Font | Configurable |
| RTL | Based on language |

---

## ADHD-Friendly Callouts

All templates use these callout conventions in Markdown:

| Syntax | Meaning | Color |
|--------|---------|-------|
| `[!] text` | Important note | Yellow `#FFF9C4` |
| `[?] text` | Deep dive / further reading | Blue `#BBDEFB` |
| `[X] text` | Warning / common mistake | Red-pink `#FFCDD2` |

The QA stage flags non-standard callouts like `[Note]`, `[Warning]`, `[TODO]`.

---

## Customizing a Template

After `nqb init`, edit these files directly in your book directory:

```
config/
  rules.yaml          ← fonts, word count, RTL, callout colors
  prompts/
    write.md          ← system prompt for chapter writing
    qa.md             ← system prompt for QA review
    init.md           ← system prompt for planning (rarely changed)
```

Changes take effect on the next `nqb write` or `nqb qa` run.

---

## Adding a Custom Template (for contributors)

See `pkg/config/templates.go`. Add a `Template` struct to the `templates` slice:

```go
{
    ID:          "my-template",
    Name:        "My Custom Template",
    Language:    "en",
    Domain:      "My domain",
    RulesYAML:   `...`,
    WritePrompt: `...`,
    QAPrompt:    `...`,
}
```
