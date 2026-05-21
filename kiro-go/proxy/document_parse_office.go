package proxy

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

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
