package docs

import "iter"

func NewDescription(title string) *Description {
    return &Description{
        section: NewSection(title),
    }
}

type Description struct {
    section *Section
}

func (d *Description) Title() string {
    return d.section.Title()
}

func (d *Description) Body() string {
    return d.section.Body()
}

func (d *Description) SubSections() iter.Seq[*Section] {
    return d.section.SubSections()
}

func (d *Description) WithBody(body string) *Description {
    d.section = d.section.WithBody(body)
    return d
}

func (d *Description) WithSubSections(sub ...*Section) *Description {
    d.section = d.section.WithSubSections(sub...)
    return d
}

func (d *Description) Markdown() string {
    return d.section.Markdown(1)
}
