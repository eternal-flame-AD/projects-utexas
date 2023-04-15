package feature

import (
	"Project2/model"
	"strings"
)

func init() {
	extractFunctions["extract_words"] = func(p *model.P, f *model.F) error {
		f.TitleWords = len(strings.Fields(p.Description.Title))
		f.DescriptionWords = len(strings.Fields(p.Description.Description))
		return nil
	}
}
