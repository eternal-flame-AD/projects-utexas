package feature

import (
	"Project2/model"
	"Project2/rparse/matcher"
	"path"
	"strings"
)

func init() {
	extractFunctions["naming_convention"] = func(p *model.P, f *model.F) error {
		f.NameVariable = model.NamingConvention{}
		f.NameExport = model.NamingConvention{}
		f.NameRFile = model.NamingConvention{}
		if p.Namespace.Exports != nil {
			for _, e := range p.Namespace.Exports {
				countNamingConvention(&f.NameExport, e)
			}
		}
		if p.RFiles != nil {
			for _, file := range p.RFiles {
				baseName := path.Base(file.Name)
				baseName = strings.TrimSuffix(baseName, path.Ext(baseName))
				countNamingConvention(&f.NameRFile, baseName)
				if state := file.Stats[matcher.MatchAssignment]; state != nil {
					var assignmentState matcher.MatchAssignmentState
					if err := remarshalAs(state, &assignmentState); err != nil {
						return err
					}
					for _, v := range assignmentState.StatsVariables {
						countNamingConvention(&f.NameVariable, v.Name)
					}
				}
			}
			f.NameRFileProp = f.NameRFile.Props()
			f.NameExportProp = f.NameExport.Props()
			f.NameVariableProp = f.NameVariable.Props()
		}
		return nil
	}
}
func countNamingConvention(counts *model.NamingConvention, ident string) {
	if ident == "" {
		return
	}
	counts.Total++

	countDot := 0
	countUnderscore := 0
	countUpper := 0
	countLower := 0
	for i := 0; i < len(ident); i++ {
		c := ident[i]
		if c == '_' {
			countUnderscore++
		} else if c == '.' {
			countDot++
		} else if c >= 'A' && c <= 'Z' {
			countUpper++
		} else if c >= 'a' && c <= 'z' {
			countLower++
		}
	}

	if countUpper == 0 && countUnderscore == 0 && countDot > 0 {
		counts.PeriodSeparated++
	} else if countLower == 0 && countUpper > 0 {
		counts.AllCaps++
	} else if countUnderscore > 0 && countUpper == 0 {
		counts.SnakeCase++
	} else if countUnderscore == 0 && countDot == 0 {
		if ident[0] >= 'A' && ident[0] <= 'Z' {
			counts.PascalCase++
		} else {
			counts.CamelCase++
		}
	}
}
