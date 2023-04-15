package feature

import (
	"Project2/model"
	"log"
	"net/url"
	"strings"
)

func init() {
	extractFunctions["package_source"] = func(p *model.P, f *model.F) error {
		url, err := url.Parse(p.URL)
		if err != nil {
			return err
		}
		if strings.HasSuffix(url.Host, "r-project.org") {
			f.Repo = "CRAN"
		} else if strings.HasSuffix(url.Host, "bioconductor.org") {
			f.Repo = "Bioconductor"
		} else {
			log.Printf("Unknown repo: %s", url.Host)
			f.Repo = "Other"
		}
		return nil
	}
}
