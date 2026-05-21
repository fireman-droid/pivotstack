package proxy

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

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
