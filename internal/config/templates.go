package config

// Template defines a pre-built book configuration template.
type Template struct {
	ID          string
	Name        string
	Description string
	Language    string
	Domain      string
	RulesYAML   string
	WritePrompt string
	QAPrompt    string
}

// BuiltinTemplates returns the three built-in templates.
func BuiltinTemplates() []Template {
	return []Template{
		arabicResearchTemplate(),
		csbookTemplate(),
		generalTemplate(),
	}
}

// TemplateByID finds a template by its ID.
func TemplateByID(id string) *Template {
	for _, t := range BuiltinTemplates() {
		if t.ID == id {
			return &t
		}
	}
	return nil
}

func arabicResearchTemplate() Template {
	return Template{
		ID:          "arabic-research",
		Name:        "Arabic Research Book (كتاب بحثي)",
		Description: "RTL Arabic book with Amiri font, scholarly tone, MSA language",
		Language:    "ar",
		Domain:      "Arabic research and scholarship",
		RulesYAML: `language: ar

word_count:
  min: 2000
  max: 6000
  target: 3500

formatting:
  line_height: 1.8
  font_arabic: "Amiri"
  font_latin: "IBM Plex Sans"
  font_code: "JetBrains Mono"
  code_theme: "dracula"
  callouts:
    note:    { prefix: "[!]", bg: "#FFF9C4" }
    deep:    { prefix: "[?]", bg: "#BBDEFB" }
    warning: { prefix: "[X]", bg: "#FFCDD2" }
  margins: "normal"

export:
  pdf: true
  epub: true
  docx: false
  web: false
  pdf_engine: xelatex
  rtl: true

tone: scholarly
audience: educated Arabic readers
glossary: {}
`,
		WritePrompt: `You are an expert Arabic author and researcher writing in Modern Standard Arabic (MSA).
Write rich, detailed, ADHD-friendly content following these rules:
- Use frequent subheadings (H2, H3) to break up text
- Bold keywords and important concepts on first use (باستخدام **تغميق** للمصطلحات)
- Use callout blocks: [!] for notes, [?] for deep dives, [X] for warnings
- Write in scholarly MSA with English technical terms in parentheses where needed
- Include concrete examples and historical context
- Follow the chapter outline provided
- Start directly with the first heading — no preamble`,
		QAPrompt: `You are a professional Arabic book editor and QA reviewer.
Review the provided Arabic chapter against the book's rules and previous chapters.
Check for:
- Correct Modern Standard Arabic (MSA) grammar and style
- Consistency with previous chapter summaries and the book's scholarly tone
- ADHD-friendly formatting (subheadings, bold keywords, callouts)
- Coverage of all outline points
- Proper use of Arabic typography conventions
- Arabic/English term consistency
Provide specific, actionable feedback in Arabic or English. Rate each dimension 1-5.`,
	}
}

func csbookTemplate() Template {
	return Template{
		ID:          "cs-book",
		Name:        "Computer Science / Technical Book",
		Description: "English technical book with code blocks, diagrams, and precise explanations",
		Language:    "en",
		Domain:      "Computer Science and Software Engineering",
		RulesYAML: `language: en

word_count:
  min: 1500
  max: 5000
  target: 3000

formatting:
  line_height: 1.6
  font_arabic: "IBM Plex Sans"
  font_latin: "IBM Plex Sans"
  font_code: "JetBrains Mono"
  code_theme: "dracula"
  callouts:
    note:    { prefix: "[!]", bg: "#FFF9C4" }
    deep:    { prefix: "[?]", bg: "#BBDEFB" }
    warning: { prefix: "[X]", bg: "#FFCDD2" }
  margins: "normal"

export:
  pdf: true
  epub: true
  docx: true
  web: false
  pdf_engine: xelatex
  rtl: false

tone: technical, precise, educational
audience: software engineers and CS students
glossary: {}
`,
		WritePrompt: `You are an expert technical author specializing in Computer Science and Software Engineering.
Write clear, precise, ADHD-friendly technical content following these rules:
- Use frequent subheadings (H2, H3) to break up text
- Bold key terms, algorithms, and concepts on first use
- Use callout blocks: [!] for important notes, [?] for deeper exploration, [X] for common mistakes
- Include working code examples with language tags in ALL code blocks
- Use diagrams described in ASCII or Mermaid where helpful
- Explain concepts with analogies before diving into technical details
- Follow the chapter outline provided
- Start directly with the first heading — no preamble`,
		QAPrompt: `You are a professional technical book editor. Review the chapter for:
- Technical accuracy of all code examples and algorithms
- Clarity of explanations for the target audience (CS students/engineers)
- Consistent use of terminology
- All code blocks have language tags
- ADHD-friendly formatting (frequent headers, bold terms, callouts)
- Coverage of outline points
Rate each dimension 1-5. Suggest specific improvements.`,
	}
}

func generalTemplate() Template {
	return Template{
		ID:          "general",
		Name:        "General Book",
		Description: "Flexible template — you fill in the domain and language",
		Language:    "en",
		Domain:      "",
		RulesYAML: `language: en

word_count:
  min: 1000
  max: 5000
  target: 2500

formatting:
  line_height: 1.6
  font_arabic: "Amiri"
  font_latin: "IBM Plex Sans"
  font_code: "JetBrains Mono"
  code_theme: "dracula"
  callouts:
    note:    { prefix: "[!]", bg: "#FFF9C4" }
    deep:    { prefix: "[?]", bg: "#BBDEFB" }
    warning: { prefix: "[X]", bg: "#FFCDD2" }
  margins: "normal"

export:
  pdf: true
  epub: true
  docx: false
  web: false
  pdf_engine: xelatex
  rtl: false

tone: engaging, clear
audience: general readers
glossary: {}
`,
		WritePrompt: `You are an expert author writing for a general audience.
Write engaging, well-structured, ADHD-friendly content:
- Use frequent subheadings to break up text
- Bold key concepts on first use
- Use callout blocks: [!] for notes, [?] for deeper dives, [X] for warnings
- Include examples and analogies to illustrate abstract ideas
- Follow the chapter outline provided
- Start directly with the first heading — no preamble`,
		QAPrompt: `You are a professional book editor. Review the chapter for:
- Engaging, clear writing appropriate for the target audience
- Consistent tone and style
- ADHD-friendly formatting
- Coverage of outline points
Rate each dimension 1-5 and suggest improvements.`,
	}
}
