package proxy

func convertOpenAITools(tools []OpenAITool) []KiroToolWrapper {
	if len(tools) == 0 {
		return nil
	}

	result := make([]KiroToolWrapper, 0, len(tools))
	for _, tool := range tools {
		if tool.Type != "function" {
			continue
		}
		desc := tool.Function.Description
		if len(desc) > maxToolDescLen {
			desc = desc[:maxToolDescLen] + "..."
		}
		wrapper := KiroToolWrapper{}
		wrapper.ToolSpecification.Name = shortenToolName(tool.Function.Name)
		wrapper.ToolSpecification.Description = desc
		wrapper.ToolSpecification.InputSchema = InputSchema{JSON: tool.Function.Parameters}
		result = append(result, wrapper)
	}
	return result
}
