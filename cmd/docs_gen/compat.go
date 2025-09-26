package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/redpanda-data/benthos/v4/public/bloblang"
	"github.com/redpanda-data/benthos/v4/public/service"
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
	// Convert AsciiDoc attributes before tables to avoid conflicts
	res = convertAsciidocAttributes(res)
	// Convert AsciiDoc tables AFTER processing links, but handle links specially in tables
	res = convertAsciidocTableToMarkdown(res)
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

func convertAsciidocAttributes(input string) string {
	// Look for AsciiDoc attributes like :driver-support: value
	attrRegex := regexp.MustCompile(`(?m)^:driver-support:\s*(.+)$`)
	
	return attrRegex.ReplaceAllStringFunc(input, func(match string) string {
		// Extract the attribute value
		parts := attrRegex.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		
		value := strings.TrimSpace(parts[1])
		
		// Parse the driver support levels
		drivers := strings.Split(value, ", ")
		
		// Build a support level table
		var result strings.Builder
		result.WriteString("\n")
		result.WriteString("| Driver | Support Level |\n")
		result.WriteString("|--------|---------------|\n")
		
		for _, driver := range drivers {
			parts := strings.Split(driver, "=")
			if len(parts) == 2 {
				driverName := strings.TrimSpace(parts[0])
				supportLevel := strings.TrimSpace(parts[1])
				
				// Capitalize support level
				supportLevel = strings.Title(supportLevel)
				
				result.WriteString(fmt.Sprintf("| `%s` | %s |\n", driverName, supportLevel))
			}
		}
		
		return result.String()
	})
}

func convertAsciidocTableToMarkdown(input string) string {
	// Look for AsciiDoc table blocks delimited by |===
	tableRegex := regexp.MustCompile(`(?s)\|===\s*\n(.*?)\n\|===`)
	
	return tableRegex.ReplaceAllStringFunc(input, func(match string) string {
		// Extract table content between |=== markers
		content := tableRegex.FindStringSubmatch(match)[1]
		lines := strings.Split(strings.TrimSpace(content), "\n")
		
		if len(lines) == 0 {
			return match
		}
		
		// Check if this is the SQL DSN table specifically
		if !strings.Contains(lines[0], "Driver | Data Source Name Format") {
			return match // Not the table we're looking for
		}
		
		// Build Markdown table
		var result strings.Builder
		result.WriteString("\n")
		
		// Header row
		result.WriteString("| Driver | Data Source Name Format |\n")
		result.WriteString("|---|---|\n")
		
		// Known drivers and their DSN formats from the source
		drivers := []struct {
			name string
			dsn  string
			link string
		}{
			{"clickhouse", `clickhouse://[username[:password]@][netloc][:port]/dbname[?param1=value1&...&paramN=valueN]`, "https://github.com/ClickHouse/clickhouse-go#dsn"},
			{"mysql", `[username[:password]@][protocol[(address)]]/dbname[?param1=value1&...&paramN=valueN]`, ""},
			{"postgres", `postgres://[user[:password]@][netloc][:port][/dbname][?param1=value1&...]`, ""},
			{"mssql", `sqlserver://[user[:password]@][netloc][:port][?database=dbname&param1=value1&...]`, ""},
			{"sqlite", `file:/path/to/filename.db[?param&=value1&...]`, ""},
			{"oracle", `oracle://[username[:password]@][netloc][:port]/service_name?server=server2&server=server3`, ""},
			{"snowflake", `username[:password]@account_identifier/dbname/schemaname[?param1=value&...&paramN=valueN]`, ""},
			{"trino", `http[s]://user[:pass]@host[:port][?parameters]`, "https://github.com/trinodb/trino-go-client#dsn-data-source-name"},
			{"gocosmos", `AccountEndpoint=<cosmosdb-endpoint>;AccountKey=<cosmosdb-account-key>[;TimeoutMs=<timeout-in-ms>][;Version=<cosmosdb-api-version>][;DefaultDb/Db=<db-name>][;AutoId=<true/false>][;InsecureSkipVerify=<true/false>]`, "https://pkg.go.dev/github.com/microsoft/gocosmos#readme-example-usage"},
			{"spanner", `projects/[PROJECT]/instances/[INSTANCE]/databases/[DATABASE]`, ""},
		}
		
		// Write each driver row
		for _, driver := range drivers {
			result.WriteString("| `")
			result.WriteString(driver.name)
			result.WriteString("` | ")
			
			if driver.link != "" {
				result.WriteString("[`")
				result.WriteString(driver.dsn)
				result.WriteString("`](")
				result.WriteString(driver.link)
				result.WriteString(")")
			} else {
				result.WriteString("`")
				result.WriteString(driver.dsn)
				result.WriteString("`")
			}
			
			result.WriteString(" |\n")
		}
		
		return result.String()
	})
}
