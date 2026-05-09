package proxy

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

// ---------- helpers: build minimal valid Office files in-memory ----------

func buildDocx(t *testing.T, body string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	add := func(name, content string) {
		f, err := w.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		_, _ = f.Write([]byte(content))
	}

	add("[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="xml" ContentType="application/xml"/>
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`)

	add("_rels/.rels", `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`)

	add("word/document.xml", fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>%s</w:body>
</w:document>`, body))

	if err := w.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

func buildXlsx(t *testing.T, sharedStrings []string, rows [][]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	add := func(name, content string) {
		f, _ := w.Create(name)
		_, _ = f.Write([]byte(content))
	}

	add("[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="xml" ContentType="application/xml"/></Types>`)

	var ssBuf strings.Builder
	ssBuf.WriteString(`<?xml version="1.0" encoding="UTF-8"?><sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)
	for _, s := range sharedStrings {
		fmt.Fprintf(&ssBuf, "<si><t>%s</t></si>", s)
	}
	ssBuf.WriteString(`</sst>`)
	add("xl/sharedStrings.xml", ssBuf.String())

	var sheetBuf strings.Builder
	sheetBuf.WriteString(`<?xml version="1.0" encoding="UTF-8"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	for r, row := range rows {
		fmt.Fprintf(&sheetBuf, `<row r="%d">`, r+1)
		for _, cell := range row {
			fmt.Fprintf(&sheetBuf, `<c t="s"><v>%s</v></c>`, cell)
		}
		sheetBuf.WriteString(`</row>`)
	}
	sheetBuf.WriteString(`</sheetData></worksheet>`)
	add("xl/worksheets/sheet1.xml", sheetBuf.String())

	_ = w.Close()
	return buf.Bytes()
}

func buildPptx(t *testing.T, slidesText []string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	add := func(name, content string) {
		f, _ := w.Create(name)
		_, _ = f.Write([]byte(content))
	}

	add("[Content_Types].xml", `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>`)

	for i, text := range slidesText {
		add(fmt.Sprintf("ppt/slides/slide%d.xml", i+1),
			fmt.Sprintf(`<?xml version="1.0"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
       xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:cSld><p:spTree><p:sp><p:txBody>
    <a:p><a:r><a:t>%s</a:t></a:r></a:p>
  </p:txBody></p:sp></p:spTree></p:cSld>
</p:sld>`, text))
	}
	_ = w.Close()
	return buf.Bytes()
}

// ---------- tests ----------

func TestParseDocxText_HappyPath(t *testing.T) {
	body := `<w:p><w:r><w:t>Hello</w:t></w:r></w:p><w:p><w:r><w:t>World</w:t><w:tab/><w:t>!</w:t></w:r></w:p>`
	data := buildDocx(t, body)

	got, err := parseDocxText(data)
	if err != nil {
		t.Fatalf("parseDocxText error: %v", err)
	}
	if !strings.Contains(got, "Hello") || !strings.Contains(got, "World") || !strings.Contains(got, "!") {
		t.Errorf("docx text missing pieces, got: %q", got)
	}
	if !strings.Contains(got, "\t") {
		t.Errorf("expected tab character to be preserved, got: %q", got)
	}
}

func TestParseXlsxText_HappyPath(t *testing.T) {
	data := buildXlsx(t,
		[]string{"alpha", "beta", "gamma"},
		[][]string{{"0", "1"}, {"2"}})

	got, err := parseXlsxText(data)
	if err != nil {
		t.Fatalf("parseXlsxText error: %v", err)
	}
	if !strings.Contains(got, "alpha") || !strings.Contains(got, "beta") || !strings.Contains(got, "gamma") {
		t.Errorf("xlsx text missing pieces, got: %q", got)
	}
	if !strings.Contains(got, "[Sheet 1]") {
		t.Errorf("expected sheet header, got: %q", got)
	}
}

func TestParsePptxText_MultiSlide(t *testing.T) {
	data := buildPptx(t, []string{"first slide", "second slide"})

	got, err := parsePptxText(data)
	if err != nil {
		t.Fatalf("parsePptxText error: %v", err)
	}
	if !strings.Contains(got, "first slide") || !strings.Contains(got, "second slide") {
		t.Errorf("pptx text missing slides, got: %q", got)
	}
	if !strings.Contains(got, "[Slide 1]") || !strings.Contains(got, "[Slide 2]") {
		t.Errorf("expected slide headers, got: %q", got)
	}
}

func TestParsePlainText_HappyPath(t *testing.T) {
	got, err := parsePlainText([]byte("hello world"))
	if err != nil || got != "hello world" {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestParsePlainText_NonUTF8(t *testing.T) {
	if _, err := parsePlainText([]byte{0xff, 0xfe, 0xfd, 0x00}); err == nil {
		t.Errorf("expected error for non-UTF8 input")
	}
}

func TestExtractDocFromClaudeBlock_CorruptedPDF_NoPanic(t *testing.T) {
	block := map[string]interface{}{
		"type": "document",
		"source": map[string]interface{}{
			"type":       "base64",
			"media_type": "application/pdf",
			"data":       base64.StdEncoding.EncodeToString([]byte("not a real pdf")),
		},
	}
	doc := extractDocFromClaudeBlock(block)
	if doc == nil {
		t.Fatal("expected KiroDoc with ErrMsg, got nil")
	}
	if doc.ErrMsg == "" {
		t.Errorf("expected ErrMsg for corrupted PDF, got Text=%q", doc.Text)
	}
}

func TestExtractDocFromClaudeBlock_LegacyDoc(t *testing.T) {
	block := map[string]interface{}{
		"type": "document",
		"source": map[string]interface{}{
			"type":       "base64",
			"media_type": "application/msword",
			"data":       base64.StdEncoding.EncodeToString([]byte("anything")),
		},
	}
	doc := extractDocFromClaudeBlock(block)
	if doc == nil || doc.ErrMsg == "" {
		t.Fatalf("expected legacy-doc error, got %+v", doc)
	}
	if !strings.Contains(doc.ErrMsg, "旧版") {
		t.Errorf("expected legacy hint in ErrMsg, got: %s", doc.ErrMsg)
	}
}

func TestExtractDocFromClaudeBlock_TextSource(t *testing.T) {
	block := map[string]interface{}{
		"type": "document",
		"source": map[string]interface{}{
			"type": "text",
			"data": "plain text body",
		},
	}
	doc := extractDocFromClaudeBlock(block)
	if doc == nil || doc.ErrMsg != "" {
		t.Fatalf("text source unexpectedly failed: %+v", doc)
	}
	if doc.Text != "plain text body" {
		t.Errorf("got text=%q", doc.Text)
	}
}

func TestExtractDocFromOpenAIBlock_ImageDefersToImageBranch(t *testing.T) {
	pngB64 := base64.StdEncoding.EncodeToString([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})
	part := map[string]interface{}{
		"type": "file",
		"file": map[string]interface{}{
			"filename":  "logo.png",
			"file_data": "data:image/png;base64," + pngB64,
		},
	}
	doc := extractDocFromOpenAIBlock(part)
	if doc != nil {
		t.Errorf("expected nil for image, got %+v", doc)
	}
}

func TestExtractDocFromOpenAIBlock_DocxFromDataURL(t *testing.T) {
	docxBytes := buildDocx(t, `<w:p><w:r><w:t>OpenAI side</w:t></w:r></w:p>`)
	dataURL := "data:application/vnd.openxmlformats-officedocument.wordprocessingml.document;base64," +
		base64.StdEncoding.EncodeToString(docxBytes)
	part := map[string]interface{}{
		"type": "file",
		"file": map[string]interface{}{
			"filename":  "x.docx",
			"file_data": dataURL,
		},
	}
	doc := extractDocFromOpenAIBlock(part)
	if doc == nil || doc.ErrMsg != "" {
		t.Fatalf("expected docx parsed, got %+v", doc)
	}
	if !strings.Contains(doc.Text, "OpenAI side") {
		t.Errorf("text mismatch: %q", doc.Text)
	}
}

func TestSizeLimit_Truncate(t *testing.T) {
	long := strings.Repeat("a", docMaxChars+500)
	body := fmt.Sprintf(`<w:p><w:r><w:t>%s</w:t></w:r></w:p>`, long)
	data := buildDocx(t, body)

	doc := extractDocFromClaudeBlock(map[string]interface{}{
		"type": "document",
		"source": map[string]interface{}{
			"type":       "base64",
			"media_type": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			"data":       base64.StdEncoding.EncodeToString(data),
		},
	})
	if doc == nil || doc.ErrMsg != "" {
		t.Fatalf("unexpected: %+v", doc)
	}
	if !doc.Truncated {
		t.Errorf("expected Truncated=true")
	}
	if utf8.RuneCountInString(doc.Text) != docMaxChars {
		t.Errorf("expected exactly %d runes, got %d", docMaxChars, utf8.RuneCountInString(doc.Text))
	}
}

func TestPerRequestLimit_Anthropic(t *testing.T) {
	docxBytes := buildDocx(t, `<w:p><w:r><w:t>tiny</w:t></w:r></w:p>`)
	b64 := base64.StdEncoding.EncodeToString(docxBytes)
	mime := "application/vnd.openxmlformats-officedocument.wordprocessingml.document"

	blocks := make([]interface{}, 0, docMaxPerRequest+2)
	for i := 0; i < docMaxPerRequest+2; i++ {
		blocks = append(blocks, map[string]interface{}{
			"type": "document",
			"source": map[string]interface{}{
				"type":       "base64",
				"media_type": mime,
				"data":       b64,
			},
		})
	}

	text, _, _ := extractClaudeUserContent(blocks)
	limitMsg := "超过单消息文档数上限"
	if !strings.Contains(text, limitMsg) {
		t.Errorf("expected limit message in output, got %q", text)
	}
	got := strings.Count(text, limitMsg)
	want := 2
	if got != want {
		t.Errorf("limit message count = %d, want %d", got, want)
	}
}

func TestPerRequestLimit_OpenAI(t *testing.T) {
	docxBytes := buildDocx(t, `<w:p><w:r><w:t>tiny</w:t></w:r></w:p>`)
	dataURL := "data:application/vnd.openxmlformats-officedocument.wordprocessingml.document;base64," +
		base64.StdEncoding.EncodeToString(docxBytes)

	parts := make([]interface{}, 0, docMaxPerRequest+2)
	for i := 0; i < docMaxPerRequest+2; i++ {
		parts = append(parts, map[string]interface{}{
			"type": "file",
			"file": map[string]interface{}{
				"filename":  "tiny.docx",
				"file_data": dataURL,
			},
		})
	}

	text, _ := extractOpenAIUserContent(parts)
	if !strings.Contains(text, "超过单消息文档数上限") {
		t.Errorf("expected limit warning, got: %q", text)
	}
}

func TestFormatDocBlock_OutputShape(t *testing.T) {
	doc := &KiroDoc{
		Filename: "report.docx",
		MimeType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		Text:     "hello",
	}
	out := formatDocBlock(doc)
	if !strings.HasPrefix(out, "<document ") {
		t.Errorf("expected <document prefix, got: %q", out)
	}
	if !strings.Contains(out, `filename="report.docx"`) || !strings.Contains(out, `chars="5"`) {
		t.Errorf("missing attrs in: %q", out)
	}
	if !strings.HasSuffix(out, "</document>") {
		t.Errorf("expected </document> suffix, got: %q", out)
	}

	errDoc := &KiroDoc{Filename: "x.pdf", MimeType: "application/pdf", ErrMsg: "boom"}
	errOut := formatDocBlock(errDoc)
	if !strings.HasSuffix(errOut, "/>") {
		t.Errorf("error doc should self-close, got: %q", errOut)
	}
	if !strings.Contains(errOut, `error="boom"`) {
		t.Errorf("missing error attr: %q", errOut)
	}
}

func TestDocKindFor_FallbackToExt(t *testing.T) {
	cases := []struct {
		mime, name, want string
	}{
		{"", "x.pdf", "pdf"},
		{"", "x.docx", "docx"},
		{"", "x.xlsx", "xlsx"},
		{"", "x.pptx", "pptx"},
		{"", "x.txt", "text"},
		{"", "notes.md", "text"},
		{"", "config.yaml", "text"},
		{"application/pdf", "", "pdf"},
		{"application/msword", "x.doc", "doc-legacy"},
		{"", "binary.unknown", "unknown"},
	}
	for _, c := range cases {
		if got := docKindFor(c.mime, c.name); got != c.want {
			t.Errorf("docKindFor(%q,%q) = %q, want %q", c.mime, c.name, got, c.want)
		}
	}
}

func TestClaudeToKiro_EmbedsDocxIntoContent(t *testing.T) {
	docxBytes := buildDocx(t, `<w:p><w:r><w:t>kiro inline doc body</w:t></w:r></w:p>`)
	b64 := base64.StdEncoding.EncodeToString(docxBytes)

	req := &ClaudeRequest{
		Model:     "claude-sonnet-4-5",
		MaxTokens: 100,
		Messages: []ClaudeMessage{{
			Role: "user",
			Content: []interface{}{
				map[string]interface{}{
					"type": "document",
					"source": map[string]interface{}{
						"type":       "base64",
						"media_type": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
						"data":       b64,
					},
				},
				map[string]interface{}{
					"type": "text",
					"text": "summarize this doc",
				},
			},
		}},
	}

	payload := ClaudeToKiro(req, false)
	if payload == nil {
		t.Fatal("ClaudeToKiro returned nil")
	}
	content := payload.ConversationState.CurrentMessage.UserInputMessage.Content
	if !strings.Contains(content, "<document ") {
		t.Errorf("expected <document tag in payload Content, got: %q", content)
	}
	if !strings.Contains(content, "kiro inline doc body") {
		t.Errorf("expected docx body text, got: %q", content)
	}
	if !strings.Contains(content, "summarize this doc") {
		t.Errorf("expected user text preserved, got: %q", content)
	}
}

func TestOpenAIToKiro_EmbedsPlainTextFile(t *testing.T) {
	dataURL := "data:text/plain;base64," + base64.StdEncoding.EncodeToString([]byte("README contents"))
	req := &OpenAIRequest{
		Model:     "claude-sonnet-4-5",
		MaxTokens: 50,
		Messages: []OpenAIMessage{{
			Role: "user",
			Content: []interface{}{
				map[string]interface{}{
					"type": "file",
					"file": map[string]interface{}{
						"filename":  "README.txt",
						"file_data": dataURL,
					},
				},
				map[string]interface{}{
					"type": "text",
					"text": "what does this file say?",
				},
			},
		}},
	}

	payload := OpenAIToKiro(req, false)
	if payload == nil {
		t.Fatal("OpenAIToKiro returned nil")
	}
	content := payload.ConversationState.CurrentMessage.UserInputMessage.Content
	if !strings.Contains(content, "<document ") || !strings.Contains(content, "README contents") {
		t.Errorf("expected document with body in Content, got: %q", content)
	}
}

func TestZipSafety_RejectsTraversal(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, _ := w.Create("../../etc/passwd")
	_, _ = f.Write([]byte("evil"))
	_ = w.Close()

	_, err := parseDocxText(buf.Bytes())
	if err == nil {
		t.Fatal("expected error for path traversal entry")
	}
}
