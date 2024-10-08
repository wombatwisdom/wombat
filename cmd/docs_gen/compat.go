package main

import (
	"github.com/redpanda-data/benthos/v4/public/bloblang"
	"github.com/redpanda-data/benthos/v4/public/service"
	"regexp"
	"strings"
)

var (
	reHeader1 = regexp.MustCompile(`(?m)^== `)
	reHeader2 = regexp.MustCompile(`(?m)^=== `)
	reHeader3 = regexp.MustCompile(`(?m)^==== `)
	reHeader4 = regexp.MustCompile(`(?m)^===== `)

	typo_1 = regexp.MustCompile(`<(.*),(.*)>>`)
)

var replacements = map[*regexp.Regexp]string{
	reHeader1: "## ",
	reHeader2: "### ",
	reHeader3: "#### ",
	reHeader4: "##### ",
	typo_1:    "<<$1,$2>>",
}

func parsePluginTemplateData(data service.TemplateDataPlugin) service.TemplateDataPlugin {
	data.Description = correctString(data.Description)
	data.Summary = correctString(data.Summary)
	data.Footnotes = correctString(data.Footnotes)

	for i, field := range data.Fields {
		data.Fields[i] = parsePluginFieldTemplateData(field)
	}

	for i, example := range data.Examples {
		data.Examples[i].Summary = correctString(example.Summary)
	}

	return data
}

func parsePluginFieldTemplateData(data service.TemplateDataPluginField) service.TemplateDataPluginField {
	data.Description = correctString(data.Description)

	for i, str := range data.ExamplesMarshalled {
		data.ExamplesMarshalled[i] = correctString(str)
	}

	for i, str := range data.Options {
		data.Options[i] = correctString(str)
	}

	for i, atrArr := range data.AnnotatedOptions {
		atrArr[0] = correctString(atrArr[0])
		atrArr[1] = correctString(atrArr[1])
		data.AnnotatedOptions[i] = atrArr
	}

	return data
}

func parseTemplateFunctionData(data bloblang.TemplateFunctionData) bloblang.TemplateFunctionData {
	data.Description = correctString(data.Description)
	data.Params = parseTemplateParamsData(data.Params)

	for i, example := range data.Examples {
		data.Examples[i] = parseTemplateExampleData(example)
	}

	return data
}

func parseTemplateMethodData(data bloblang.TemplateMethodData) bloblang.TemplateMethodData {
	data.Description = correctString(data.Description)
	data.Params = parseTemplateParamsData(data.Params)

	for i, example := range data.Examples {
		data.Examples[i] = parseTemplateExampleData(example)
	}

	return data
}

func parseTemplateExampleData(data bloblang.TemplateExampleData) bloblang.TemplateExampleData {
	data.Summary = correctString(data.Summary)
	return data
}

func parseTemplateParamsData(data bloblang.TemplateParamsData) bloblang.TemplateParamsData {
	for i, param := range data.Definitions {
		data.Definitions[i] = parseTemplateParamData(param)
	}

	return data
}

func parseTemplateParamData(data bloblang.TemplateParamData) bloblang.TemplateParamData {
	data.Description = correctString(data.Description)
	return data
}

func correctString(inp string) string {
	res := correctHeaders(inp)
	res = replaceAsciidocLinksWithMarkdown(res)
	return res
}

func correctHeaders(inp string) string {
	for re, replacement := range replacements {
		inp = re.ReplaceAllString(inp, replacement)
	}
	return inp
}

var (
	asciidocLinkRegex          = regexp.MustCompile(`(https?://[^\[]+)\[([^\]]+)\]`)
	xrefLinkRegex              = regexp.MustCompile(`xref:([^\[]+)\[([^\]]+)\]`)
	angleBracketLinkRegex      = regexp.MustCompile(`<<([^,]+),([^>]+)>>`)
	singlAngleBracketLinkRegex = regexp.MustCompile(`<<([^>]+)>>`)
)

func replaceAsciidocLinksWithMarkdown(input string) string {
	input = asciidocLinkRegex.ReplaceAllStringFunc(input, func(match string) string {
		parts := asciidocLinkRegex.FindStringSubmatch(match)
		if len(parts) == 3 {
			url := parts[1]
			text := strings.TrimSuffix(parts[2], "^")
			return "[" + text + "](" + url + ")"
		}
		return match
	})

	input = xrefLinkRegex.ReplaceAllStringFunc(input, func(match string) string {
		parts := xrefLinkRegex.FindStringSubmatch(match)
		if len(parts) == 3 {
			//url := parts[1]
			text := strings.TrimSuffix(parts[2], "^")
			return text
		}
		return match
	})

	input = angleBracketLinkRegex.ReplaceAllStringFunc(input, func(match string) string {
		parts := angleBracketLinkRegex.FindStringSubmatch(match)
		if len(parts) == 3 {
			//url := parts[1]
			text := strings.TrimSuffix(parts[2], "^")
			return text
		}
		return match
	})

	input = singlAngleBracketLinkRegex.ReplaceAllStringFunc(input, func(match string) string {
		parts := singlAngleBracketLinkRegex.FindStringSubmatch(match)
		if len(parts) == 2 {
			//url := parts[1]
			text := strings.TrimSuffix(parts[1], "^")
			return text
		}
		return match
	})

	return input
}
