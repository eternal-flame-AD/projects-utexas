package feature

import (
	"Project2/model"
	"Project2/rparse/matcher"
)

func init() {
	extractFunctions["r_file"] = func(p *model.P, f *model.F) error {
		countEqAssign := 0
		countLeftAssign := 0
		sumTokens := 0
		if p.RFiles != nil {
			for _, file := range p.RFiles {
				sumTokens += file.NTokens
				if stats := file.Stats[matcher.MatchAssignment]; stats != nil {
					var state matcher.MatchAssignmentState
					if err := remarshalAs(stats, &state); err != nil {
						return err
					}
					countEqAssign += state.StatsEqAssignCount
					countLeftAssign += state.StatsLeftAssignCount
				}
			}
			f.AvgRTokens = float64(sumTokens) / float64(len(p.RFiles))
		}
		f.PropEqAssign = float64(countEqAssign) / float64(countEqAssign+countLeftAssign)
		return nil
	}
}
