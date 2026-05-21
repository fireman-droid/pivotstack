package proxy

import (
	"archive/zip"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

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
