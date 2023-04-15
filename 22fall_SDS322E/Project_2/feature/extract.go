package feature

import (
	"Project2/model"
	"log"
)

var extractFunctions = make(map[string]func(*model.P, *model.F) error)

func Extract(p *model.P) (f *model.F, err error) {
	f = &model.F{
		Package: p.Description.Package,
	}
	for name, fn := range extractFunctions {
		if err := fn(p, f); err != nil {
			log.Printf("Error extracting feature: [%s::%s] %s", p.Description.Package, name, err)
			return nil, err
		}
	}
	f.FloatCheck()
	return f, nil
}
