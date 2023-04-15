package feature

import (
	"Project2/model"
	"regexp"
	"strconv"
)

func init() {
	extractFunctions["version"] = func(p *model.P, f *model.F) error {
		if submatch := regexp.MustCompile("^(\\d+)").FindStringSubmatch(p.Description.Version); submatch != nil {
			f.MajorVersion, _ = strconv.Atoi(submatch[1])
		} else {
			f.MajorVersion = -1
		}
		return nil
	}
}
