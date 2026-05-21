package proxy

import (
	"bytes"
	"fmt"
	pdflib "github.com/ledongthuc/pdf"
	"io"
	"strings"
	"unicode/utf8"
)

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

func parsePlainText(data []byte) (string, error) {
	if !utf8.Valid(data) {
		return "", fmt.Errorf("文件不是有效的 UTF-8 文本")
	}
	return string(data), nil
}
