package proxy

import (
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

func extractOpenAIUserContent(content interface{}) (string, []KiroImage) {
	if s, ok := content.(string); ok {
		return s, nil
	}

	var text string
	var images []KiroImage
	docCount := 0

	tryDoc := func(part map[string]interface{}) bool {
		doc := extractDocFromOpenAIBlock(part)
		if doc == nil {
			return false
		}
		if docCount >= docMaxPerRequest {
			text += `<document error="超过单消息文档数上限 ` + strconv.Itoa(docMaxPerRequest) + ` 个，已忽略后续附件"/>` + "\n"
		} else {
			text += formatDocBlock(doc) + "\n"
		}
		docCount++
		return true
	}

	if part, ok := content.(map[string]interface{}); ok {
		if t, ok := extractOpenAITextPart(part); ok {
			text += t
		}
		if !tryDoc(part) {
			if img := extractImageFromOpenAIPart(part); img != nil {
				images = append(images, *img)
			}
		}
	}

	if parts, ok := content.([]interface{}); ok {
		for _, p := range parts {
			part, ok := p.(map[string]interface{})
			if !ok {
				continue
			}

			if t, ok := extractOpenAITextPart(part); ok {
				text += t
			}
			if tryDoc(part) {
				continue
			}
			if img := extractImageFromOpenAIPart(part); img != nil {
				images = append(images, *img)
			}
		}
	}

	if len(images) > 0 {
		text = sanitizeImagePlaceholders(text)
	}

	return text, images
}

func extractOpenAIMessageText(content interface{}) string {
	if content == nil {
		return ""
	}

	if s, ok := content.(string); ok {
		return s
	}

	if text, _ := extractOpenAIUserContent(content); strings.TrimSpace(text) != "" {
		return text
	}

	switch v := content.(type) {
	case map[string]interface{}:
		if nested, ok := v["content"]; ok {
			if nestedText := extractOpenAIMessageText(nested); strings.TrimSpace(nestedText) != "" {
				return nestedText
			}
		}
		if raw, err := json.Marshal(v); err == nil {
			return string(raw)
		}
	case []interface{}:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			partText := extractOpenAIMessageText(item)
			if strings.TrimSpace(partText) != "" {
				parts = append(parts, partText)
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "")
		}
		if raw, err := json.Marshal(v); err == nil {
			return string(raw)
		}
	default:
		if raw, err := json.Marshal(v); err == nil {
			return string(raw)
		}
	}

	return ""
}

func extractOpenAITextPart(part map[string]interface{}) (string, bool) {
	partType, _ := part["type"].(string)
	switch partType {
	case "text", "input_text":
		if t, ok := part["text"].(string); ok {
			return t, true
		}
	}

	if t, ok := part["text"].(string); ok {
		return t, true
	}

	return "", false
}

func extractImageFromOpenAIPart(part map[string]interface{}) *KiroImage {
	partType, _ := part["type"].(string)
	if partType != "" {
		switch partType {
		case "image", "image_url", "input_image", "file", "input_file":
		default:
			return nil
		}
	}

	if fileObj, ok := part["file"].(map[string]interface{}); ok {
		if img := extractImageFromOpenAIPart(fileObj); img != nil {
			return img
		}
	}

	if sourceObj, ok := part["source"].(map[string]interface{}); ok {
		if img := extractImageFromOpenAIPart(sourceObj); img != nil {
			return img
		}
	}

	if raw, ok := part["mime"].(string); ok && !strings.HasPrefix(strings.ToLower(raw), "image/") {
		return nil
	}
	if raw, ok := part["media_type"].(string); ok && !strings.HasPrefix(strings.ToLower(raw), "image/") {
		return nil
	}
	if raw, ok := part["mime_type"].(string); ok && !strings.HasPrefix(strings.ToLower(raw), "image/") {
		return nil
	}

	if raw, ok := part["url"].(string); ok {
		if img := parseDataURL(raw); img != nil {
			return img
		}
	}

	if raw, ok := part["b64_json"].(string); ok {
		if img := parseBase64Image(raw, "png"); img != nil {
			return img
		}
	}

	if raw, ok := part["image_url"]; ok {
		switch v := raw.(type) {
		case string:
			if img := parseDataURL(v); img != nil {
				return img
			}
		case map[string]interface{}:
			if u, ok := v["url"].(string); ok {
				if img := parseDataURL(u); img != nil {
					return img
				}
			}
		}
	}

	if raw, ok := part["image_base64"].(string); ok {
		if img := parseBase64Image(raw, "png"); img != nil {
			return img
		}
	}
	if raw, ok := part["data"].(string); ok {
		if img := parseDataURL(raw); img != nil {
			return img
		}
		if img := parseBase64Image(raw, "png"); img != nil {
			return img
		}
	}

	return nil
}

func sanitizeImagePlaceholders(text string) string {
	re := regexp.MustCompile(`\[Image\s+\d+\]`)
	cleaned := re.ReplaceAllString(text, "")
	cleaned = strings.Join(strings.Fields(cleaned), " ")
	return strings.TrimSpace(cleaned)
}

func normalizeUserContent(text string, hasImages bool) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" && hasImages {
		return "Please analyze the attached image."
	}
	return trimmed
}

func parseDataURL(url string) *KiroImage {
	cleaned := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(url, "\n", ""), "\r", ""))
	if strings.Contains(cleaned, "[Image") {
		return nil
	}
	re := regexp.MustCompile(`^data:image/([a-zA-Z0-9+.-]+)(;[a-zA-Z0-9=._:+-]+)*;base64,(.+)$`)
	matches := re.FindStringSubmatch(cleaned)
	if len(matches) == 4 {
		return parseBase64Image(matches[3], matches[1])
	}
	if len(matches) != 3 {
		return nil
	}

	return parseBase64Image(matches[2], matches[1])
}

func parseBase64Image(data, format string) *KiroImage {
	format = strings.ToLower(format)
	if format == "jpg" {
		format = "jpeg"
	}

	// 验证 base64
	if _, err := base64.StdEncoding.DecodeString(data); err != nil {
		if _, errRaw := base64.RawStdEncoding.DecodeString(data); errRaw != nil {
			if _, errURL := base64.URLEncoding.DecodeString(data); errURL != nil {
				if _, errRawURL := base64.RawURLEncoding.DecodeString(data); errRawURL != nil {
					return nil
				}
			}
		}
	}

	if format == "" {
		format = "png"
	}

	return &KiroImage{
		Format: format,
		Source: struct {
			Bytes string `json:"bytes"`
		}{Bytes: data},
	}
}
