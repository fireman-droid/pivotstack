package proxy

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	pdflib "github.com/ledongthuc/pdf"
)

const (
	docMaxChars       = 100_000
	docMaxPerRequest  = 8
	docMaxBytesDecode = 50 << 20
	docZipMaxFileSize = 50 << 20
	docZipMaxEntries  = 1000
)

type KiroDoc struct {
	Filename  string
	MimeType  string
	Text      string
	Truncated bool
	Pages     int
	ErrMsg    string
}

func extractDocFromClaudeBlock(block map[string]interface{}) *KiroDoc {
	blockType, _ := block["type"].(string)
	if blockType != "document" {
		return nil
	}

	source, _ := block["source"].(map[string]interface{})
	if source == nil {
		return &KiroDoc{ErrMsg: "document 块缺少 source 字段"}
	}

	filename, _ := block["filename"].(string)
	if filename == "" {
		filename, _ = source["filename"].(string)
	}

	mediaType, _ := source["media_type"].(string)
	if mediaType == "" {
		mediaType, _ = source["mediaType"].(string)
	}
	if mediaType == "" {
		mediaType, _ = source["mime_type"].(string)
	}

	sourceType, _ := source["type"].(string)
	var data []byte

	switch sourceType {
	case "base64":
		raw, _ := source["data"].(string)
		if raw == "" {
			return &KiroDoc{Filename: filename, MimeType: mediaType, ErrMsg: "document.source.data 为空"}
		}
		decoded, err := decodeBase64Flexible(raw)
		if err != nil {
			return &KiroDoc{Filename: filename, MimeType: mediaType, ErrMsg: "base64 解码失败: " + err.Error()}
		}
		data = decoded
	case "text":
		raw, _ := source["data"].(string)
		if raw == "" {
			return &KiroDoc{Filename: filename, MimeType: mediaType, ErrMsg: "document.source.data 为空"}
		}
		data = []byte(raw)
		if mediaType == "" {
			mediaType = "text/plain"
		}
	case "url":
		return &KiroDoc{Filename: filename, MimeType: mediaType, ErrMsg: "暂不支持 URL 类型 document，请改用 base64"}
	case "":
		raw, _ := source["data"].(string)
		if raw == "" {
			return &KiroDoc{Filename: filename, MimeType: mediaType, ErrMsg: "document.source 类型缺失且无 data"}
		}
		if strings.HasPrefix(raw, "data:") {
			if mt, body, ok := splitDataURL(raw); ok {
				if mediaType == "" {
					mediaType = mt
				}
				if decoded, err := decodeBase64Flexible(body); err == nil {
					data = decoded
				}
			}
		}
		if data == nil {
			if decoded, err := decodeBase64Flexible(raw); err == nil {
				data = decoded
			}
		}
		if data == nil {
			return &KiroDoc{Filename: filename, MimeType: mediaType, ErrMsg: "document.source 数据无法解析"}
		}
	default:
		return &KiroDoc{Filename: filename, MimeType: mediaType, ErrMsg: "不支持的 document.source.type: " + sourceType}
	}

	return processDocBytes(data, mediaType, filename)
}

func extractDocFromOpenAIBlock(part map[string]interface{}) *KiroDoc {
	partType, _ := part["type"].(string)
	if partType != "file" && partType != "input_file" && partType != "document" {
		return nil
	}

	fileObj := part
	if inner, ok := part["file"].(map[string]interface{}); ok {
		fileObj = inner
	}

	filename, _ := fileObj["filename"].(string)
	if filename == "" {
		filename, _ = part["filename"].(string)
	}

	var rawData string
	for _, key := range []string{"file_data", "data", "b64_json"} {
		if v, ok := fileObj[key].(string); ok && v != "" {
			rawData = v
			break
		}
	}
	if rawData == "" {
		if v, ok := part["file_data"].(string); ok {
			rawData = v
		}
	}
	if rawData == "" {
		return nil
	}

	var mediaType string
	var data []byte

	if strings.HasPrefix(rawData, "data:") {
		if mt, body, ok := splitDataURL(rawData); ok {
			mediaType = mt
			decoded, err := decodeBase64Flexible(body)
			if err != nil {
				return &KiroDoc{Filename: filename, MimeType: mediaType, ErrMsg: "base64 解码失败: " + err.Error()}
			}
			data = decoded
		}
	} else {
		decoded, err := decodeBase64Flexible(rawData)
		if err != nil {
			return nil
		}
		data = decoded
	}

	if data == nil {
		return nil
	}

	if strings.HasPrefix(strings.ToLower(mediaType), "image/") {
		return nil
	}
	if mediaType == "" && filename != "" {
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), "."))
		if isImageExt(ext) {
			return nil
		}
	}

	return processDocBytes(data, mediaType, filename)
}

func processDocBytes(data []byte, mime, filename string) *KiroDoc {
	if len(data) == 0 {
		return &KiroDoc{Filename: filename, MimeType: mime, ErrMsg: "文档数据为空"}
	}
	if len(data) > docMaxBytesDecode {
		return &KiroDoc{Filename: filename, MimeType: mime,
			ErrMsg: fmt.Sprintf("文件过大（%d MB），单文档上限 %d MB", len(data)>>20, docMaxBytesDecode>>20)}
	}

	kind := docKindFor(mime, filename)
	text, pages, truncated, err := dispatchParser(kind, data)
	if err != nil {
		return &KiroDoc{Filename: filename, MimeType: mime, Pages: pages, ErrMsg: err.Error()}
	}
	return &KiroDoc{
		Filename:  filename,
		MimeType:  mime,
		Text:      text,
		Pages:     pages,
		Truncated: truncated,
	}
}

func docKindFor(mime, filename string) string {
	m := strings.ToLower(strings.TrimSpace(mime))
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), "."))

	switch {
	case strings.Contains(m, "application/pdf"):
		return "pdf"
	case strings.Contains(m, "wordprocessingml.document"):
		return "docx"
	case strings.Contains(m, "spreadsheetml.sheet"):
		return "xlsx"
	case strings.Contains(m, "presentationml.presentation"):
		return "pptx"
	case m == "application/msword":
		return "doc-legacy"
	case m == "application/vnd.ms-excel":
		return "xls-legacy"
	case m == "application/vnd.ms-powerpoint":
		return "ppt-legacy"
	case strings.HasPrefix(m, "text/"),
		m == "application/json",
		m == "application/xml",
		m == "application/x-yaml",
		m == "application/yaml",
		m == "application/toml":
		return "text"
	}

	switch ext {
	case "pdf":
		return "pdf"
	case "docx":
		return "docx"
	case "xlsx":
		return "xlsx"
	case "pptx":
		return "pptx"
	case "doc":
		return "doc-legacy"
	case "xls":
		return "xls-legacy"
	case "ppt":
		return "ppt-legacy"
	}
	if isPlainTextExt(ext) {
		return "text"
	}
	return "unknown"
}

func dispatchParser(kind string, data []byte) (text string, pages int, truncated bool, err error) {
	switch kind {
	case "pdf":
		text, pages, err = parsePDFText(data)
	case "docx":
		text, err = parseDocxText(data)
	case "xlsx":
		text, err = parseXlsxText(data)
	case "pptx":
		text, err = parsePptxText(data)
	case "text":
		text, err = parsePlainText(data)
	case "doc-legacy", "xls-legacy", "ppt-legacy":
		return "", 0, false, fmt.Errorf("旧版 Office 二进制文档不支持，请另存为 .docx/.xlsx/.pptx 后上传")
	default:
		return "", 0, false, fmt.Errorf("不支持的文档类型，已识别的扩展名/MIME 才会被解析")
	}
	if err != nil {
		return "", pages, false, err
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return "", pages, false, fmt.Errorf("未抽出任何文本，可能是扫描件、加密文件或空文档")
	}

	if utf8.RuneCountInString(text) > docMaxChars {
		runes := []rune(text)
		text = string(runes[:docMaxChars])
		truncated = true
	}
	return text, pages, truncated, nil
}

func parsePDFText(data []byte) (string, int, error) {
	defer func() {
		_ = recover() // ledongthuc/pdf 在某些非法输入会 panic，统一吞为 error
	}()
	r, err := pdflib.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", 0, fmt.Errorf("PDF 解析失败: %v", err)
	}
	pages := r.NumPage()
	rd, err := r.GetPlainText()
	if err != nil {
		return "", pages, fmt.Errorf("PDF 文本抽取失败: %v", err)
	}
	var sb strings.Builder
	if _, err := io.Copy(&sb, rd); err != nil {
		return sb.String(), pages, nil
	}
	return sb.String(), pages, nil
}

func parseDocxText(data []byte) (string, error) {
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("docx zip 打开失败: %v", err)
	}
	if err := checkZipSafety(z); err != nil {
		return "", err
	}
	for _, f := range z.File {
		if f.Name == "word/document.xml" {
			return readDocxBodyXML(f)
		}
	}
	return "", fmt.Errorf("docx 缺少 word/document.xml")
}

func readDocxBodyXML(f *zip.File) (string, error) {
	rc, err := f.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()

	var sb strings.Builder
	dec := xml.NewDecoder(rc)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return sb.String(), nil
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "t":
				var inner string
				if err := dec.DecodeElement(&inner, &t); err == nil {
					sb.WriteString(inner)
				}
			case "tab":
				sb.WriteByte('\t')
			case "br", "cr":
				sb.WriteByte('\n')
			}
		case xml.EndElement:
			if t.Name.Local == "p" {
				sb.WriteByte('\n')
			}
		}
	}
	return sb.String(), nil
}

func parseXlsxText(data []byte) (string, error) {
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("xlsx zip 打开失败: %v", err)
	}
	if err := checkZipSafety(z); err != nil {
		return "", err
	}

	var shared []string
	for _, f := range z.File {
		if f.Name == "xl/sharedStrings.xml" {
			if ss, err := readXlsxSharedStrings(f); err == nil {
				shared = ss
			}
			break
		}
	}

	type sheetEntry struct {
		order int
		f     *zip.File
	}
	var sheets []sheetEntry
	re := regexp.MustCompile(`^xl/worksheets/sheet(\d+)\.xml$`)
	for _, f := range z.File {
		if m := re.FindStringSubmatch(f.Name); m != nil {
			n, _ := strconv.Atoi(m[1])
			sheets = append(sheets, sheetEntry{order: n, f: f})
		}
	}
	sort.Slice(sheets, func(i, j int) bool { return sheets[i].order < sheets[j].order })

	var sb strings.Builder
	for _, s := range sheets {
		fmt.Fprintf(&sb, "[Sheet %d]\n", s.order)
		rows, _ := readXlsxSheetRows(s.f, shared)
		for _, row := range rows {
			sb.WriteString(strings.Join(row, "\t"))
			sb.WriteByte('\n')
		}
		sb.WriteByte('\n')
	}
	return sb.String(), nil
}

func readXlsxSharedStrings(f *zip.File) ([]string, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	var result []string
	dec := xml.NewDecoder(rc)
	var inSi, inT bool
	var current strings.Builder

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "si":
				inSi = true
				current.Reset()
			case "t":
				if inSi {
					inT = true
				}
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "si":
				if inSi {
					result = append(result, current.String())
				}
				inSi = false
			case "t":
				inT = false
			}
		case xml.CharData:
			if inSi && inT {
				current.Write(t)
			}
		}
	}
	return result, nil
}

func readXlsxSheetRows(f *zip.File, shared []string) ([][]string, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	var rows [][]string
	var currentRow []string
	dec := xml.NewDecoder(rc)

	var cellType string
	var inV, inIs, inT bool
	var vBuf, tBuf strings.Builder

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "row":
				currentRow = nil
			case "c":
				cellType = ""
				for _, a := range t.Attr {
					if a.Name.Local == "t" {
						cellType = a.Value
					}
				}
			case "v":
				inV = true
				vBuf.Reset()
			case "is":
				inIs = true
			case "t":
				if inIs {
					inT = true
					tBuf.Reset()
				}
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "v":
				inV = false
				val := vBuf.String()
				if cellType == "s" {
					if idx, err := strconv.Atoi(val); err == nil && idx >= 0 && idx < len(shared) {
						currentRow = append(currentRow, shared[idx])
					} else {
						currentRow = append(currentRow, val)
					}
				} else {
					currentRow = append(currentRow, val)
				}
			case "t":
				if inT {
					currentRow = append(currentRow, tBuf.String())
				}
				inT = false
			case "is":
				inIs = false
			case "row":
				rows = append(rows, currentRow)
			}
		case xml.CharData:
			if inV {
				vBuf.Write(t)
			}
			if inT {
				tBuf.Write(t)
			}
		}
	}
	return rows, nil
}

func parsePptxText(data []byte) (string, error) {
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("pptx zip 打开失败: %v", err)
	}
	if err := checkZipSafety(z); err != nil {
		return "", err
	}

	type slideEntry struct {
		order int
		f     *zip.File
	}
	var slides []slideEntry
	re := regexp.MustCompile(`^ppt/slides/slide(\d+)\.xml$`)
	for _, f := range z.File {
		if m := re.FindStringSubmatch(f.Name); m != nil {
			n, _ := strconv.Atoi(m[1])
			slides = append(slides, slideEntry{order: n, f: f})
		}
	}
	sort.Slice(slides, func(i, j int) bool { return slides[i].order < slides[j].order })

	var sb strings.Builder
	for _, s := range slides {
		fmt.Fprintf(&sb, "[Slide %d]\n", s.order)
		text, _ := readPptxSlideText(s.f)
		sb.WriteString(text)
		sb.WriteByte('\n')
	}
	return sb.String(), nil
}

func readPptxSlideText(f *zip.File) (string, error) {
	rc, err := f.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()

	var sb strings.Builder
	dec := xml.NewDecoder(rc)
	var inT bool

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "t":
				inT = true
			case "br":
				sb.WriteByte('\n')
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t":
				inT = false
			case "p":
				sb.WriteByte('\n')
			}
		case xml.CharData:
			if inT {
				sb.Write(t)
			}
		}
	}
	return sb.String(), nil
}

func parsePlainText(data []byte) (string, error) {
	if !utf8.Valid(data) {
		return "", fmt.Errorf("文件不是有效的 UTF-8 文本")
	}
	return string(data), nil
}

func formatDocBlock(doc *KiroDoc) string {
	if doc == nil {
		return ""
	}
	if doc.ErrMsg != "" {
		attrs := []string{
			fmt.Sprintf(`filename=%q`, doc.Filename),
			fmt.Sprintf(`type=%q`, doc.MimeType),
			fmt.Sprintf(`error=%q`, doc.ErrMsg),
		}
		return fmt.Sprintf("<document %s/>", strings.Join(attrs, " "))
	}
	chars := utf8.RuneCountInString(doc.Text)
	attrs := []string{
		fmt.Sprintf(`filename=%q`, doc.Filename),
		fmt.Sprintf(`type=%q`, doc.MimeType),
	}
	if doc.Pages > 0 {
		attrs = append(attrs, fmt.Sprintf(`pages="%d"`, doc.Pages))
	}
	attrs = append(attrs, fmt.Sprintf(`chars="%d"`, chars))
	if doc.Truncated {
		attrs = append(attrs, `truncated="true"`)
	}
	return fmt.Sprintf("<document %s>\n%s\n</document>", strings.Join(attrs, " "), doc.Text)
}

func decodeBase64Flexible(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("空数据")
	}
	if data, err := base64.StdEncoding.DecodeString(s); err == nil {
		return data, nil
	}
	if data, err := base64.RawStdEncoding.DecodeString(s); err == nil {
		return data, nil
	}
	if data, err := base64.URLEncoding.DecodeString(s); err == nil {
		return data, nil
	}
	if data, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return data, nil
	}
	return nil, fmt.Errorf("base64 解码失败")
}

func splitDataURL(s string) (mime, data string, ok bool) {
	if !strings.HasPrefix(s, "data:") {
		return "", "", false
	}
	rest := s[5:]
	semi := strings.Index(rest, ";")
	comma := strings.Index(rest, ",")
	if comma < 0 {
		return "", "", false
	}
	if semi < 0 || semi > comma {
		mime = rest[:comma]
	} else {
		mime = rest[:semi]
	}
	data = rest[comma+1:]
	return mime, data, true
}

func isImageExt(ext string) bool {
	switch strings.TrimPrefix(ext, ".") {
	case "png", "jpg", "jpeg", "gif", "webp", "bmp", "ico", "svg", "tiff", "tif":
		return true
	}
	return false
}

var plainTextExts = map[string]bool{
	"txt": true, "md": true, "markdown": true, "rst": true,
	"csv": true, "tsv": true,
	"json": true, "xml": true, "yaml": true, "yml": true,
	"toml": true, "ini": true, "conf": true, "cfg": true, "env": true,
	"log": true,
	"go": true, "py": true, "js": true, "ts": true,
	"tsx": true, "jsx": true, "mjs": true, "cjs": true,
	"java": true, "c": true, "cc": true, "cpp": true, "cxx": true,
	"h": true, "hpp": true,
	"rs": true, "rb": true, "php": true,
	"sh": true, "bash": true, "zsh": true, "fish": true, "ps1": true, "bat": true,
	"swift": true, "kt": true, "kts": true, "scala": true, "groovy": true,
	"lua": true, "pl": true, "r": true, "dart": true, "ex": true, "exs": true,
	"html": true, "htm": true, "css": true, "scss": true, "sass": true, "less": true,
	"sql": true, "graphql": true, "gql": true, "proto": true,
	"vue": true, "svelte": true,
}

func isPlainTextExt(ext string) bool {
	return plainTextExts[strings.ToLower(strings.TrimPrefix(ext, "."))]
}

func checkZipSafety(z *zip.Reader) error {
	if len(z.File) > docZipMaxEntries {
		return fmt.Errorf("zip 条目数 %d 超过上限 %d", len(z.File), docZipMaxEntries)
	}
	for _, f := range z.File {
		if f.UncompressedSize64 > uint64(docZipMaxFileSize) {
			return fmt.Errorf("zip 内文件 %s 解压后超过 %d MB", f.Name, docZipMaxFileSize>>20)
		}
		cleaned := filepath.Clean(f.Name)
		if strings.Contains(cleaned, "..") || strings.HasPrefix(cleaned, "/") || strings.HasPrefix(cleaned, `\`) {
			return fmt.Errorf("zip 内含可疑路径: %s", f.Name)
		}
	}
	return nil
}
