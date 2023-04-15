package feature

import (
	"Project2/model"
	"Project2/rparse/matcher"
)

func init() {
	extractFunctions["function_calls"] = func(p *model.P, f *model.F) error {
		f.CallRandomForest = 0
		f.CallRpart = 0
		if p.RFiles != nil {
			for _, file := range p.RFiles {
				if state := file.Stats[matcher.MatchFunctionCall]; state != nil {
					var functionCallState matcher.MatchFunctionCallState
					if err := remarshalAs(state, &functionCallState); err != nil {
						return err
					}
					for _, v := range functionCallState.StatsFunctionCalls {
						switch v.Name {
						case "randomForest":
							f.CallRandomForest++
						case "rpart":
							f.CallRpart++
						}
					}
				}
			}
		}

		return nil
	}
}
