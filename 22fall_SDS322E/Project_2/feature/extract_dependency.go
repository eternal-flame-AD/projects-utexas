package feature

import "Project2/model"

func init() {
	extractFunctions["dependency"] = func(p *model.P, f *model.F) error {
		if p.Namespace.Exports != nil {
			f.RExportNum = len(p.Namespace.Exports)
		}
		f.RImportToDepend = float64(len(p.Description.Imports)) / float64(len(p.Description.Depends))

		return nil
	}
}
