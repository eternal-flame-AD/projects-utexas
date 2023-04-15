package feature

import "Project2/model"

func init() {
	extractFunctions["file_extension"] = func(p *model.P, f *model.F) error {
		f.ExtR = float64(p.FileExtensions[".r"])
		f.ExtRd = float64(p.FileExtensions[".rd"])
		f.ExtRds = float64(p.FileExtensions[".rds"])
		f.ExtRda = float64(p.FileExtensions[".rda"]) + float64(p.FileExtensions[".rdata"])
		f.COverR = float64(
			p.FileExtensions[".c"]+
				p.FileExtensions[".cpp"]) / float64(p.FileExtensions[".r"])
		f.FOverR = float64(p.FileExtensions[".f"]+
			p.FileExtensions[".f90"]+
			p.FileExtensions[".for"]) / float64(p.FileExtensions[".r"])
		f.JOverR = float64(p.FileExtensions[".java"]+
			p.FileExtensions[".jar"]+
			p.FileExtensions[".class"]) / float64(p.FileExtensions[".r"])
		return nil
	}
}
