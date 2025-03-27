package docs

import (
    "fmt"
    "iter"
    "slices"
    "strings"
)

func NewSection(title string) *Section {
    return &Section{
        title: title,
    }
}

type Section struct {
    title       string
    body        string
    subSections []*Section
}

func (s *Section) Title() string {
    return strings.TrimSpace(s.title)
}

func (s *Section) Body() string {
    return s.body
}

func (s *Section) SubSections() iter.Seq[*Section] {
    return slices.Values(s.subSections)
}

func (s *Section) WithBody(body string) *Section {
    s.body = strings.TrimSpace(body)
    return s
}

func (s *Section) WithSubSections(subs ...*Section) *Section {
    s.subSections = append(s.subSections, subs...)
    return s
}

func (s *Section) Markdown(level int) string {
    var result string

    if s.title != "" {
        result += fmt.Sprintf("%s %s\n", strings.Repeat("#", level), s.title)
    }

    if s.body != "" {
        result += fmt.Sprintf("%s\n", s.body)
    }

    for _, subSection := range s.subSections {
        result += subSection.Markdown(level + 1)
    }

    return result
}
