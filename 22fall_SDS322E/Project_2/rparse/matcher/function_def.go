package matcher

import (
	"Project2/rparse"
)

const MatchFunctionDef = "function_def"

type FunctionArg struct {
	Name    string
	Default string
}
type Function struct {
	AssignedName string
	Args         []FunctionArg
}

type MatchFunctionDefState struct {
	initialized         bool
	functionDef         Function
	curArg              FunctionArg
	nextIsFormalDefault bool

	thisFuntion         Function
	funcKeywordTokenIdx int
	beginParenStack     []byte
	StatFunctionDefs    []Function
}

func MatchFunctionDefUpdate(state MatchFunctionDefState, i int, tokens rparse.RTokenList) (next MatchFunctionDefState, delta int, err error) {
	if !state.initialized {
		state.initialized = true
		state.funcKeywordTokenIdx = -1
		return state, 0, nil
	}

	if tokens[i].Token == "FUNCTION" {
		state.funcKeywordTokenIdx = i
		assignMatcherState := tokens[i-1].MatcherState[MatchAssignment].(MatchAssignmentState)
		if assignMatcherState.assignName != "" {
			state.functionDef.AssignedName = assignMatcherState.assignName
		}
		state.beginParenStack = tokens[i].MatcherState[TrackParenthesis].(TrackParenthesisState).Stack
		return state, 1, nil
	} else if state.funcKeywordTokenIdx != -1 {
		thisParenStack := tokens[i].MatcherState[TrackParenthesis].(TrackParenthesisState).Stack
		diffDelta := len(thisParenStack) - len(state.beginParenStack)
		if diffDelta == 0 {
			if state.curArg.Name != "" {
				state.functionDef.Args = append(state.functionDef.Args, state.curArg)
			}
			// finish parsing all arguments, go back to function keyword
			stateOnFunctionKeyword := tokens[state.funcKeywordTokenIdx].MatcherState[MatchFunctionDef].(MatchFunctionDefState)
			stateOnFunctionKeyword.thisFuntion = state.functionDef
			tokens[state.funcKeywordTokenIdx].MatcherState[MatchFunctionDef] = stateOnFunctionKeyword
			state.StatFunctionDefs = append(state.StatFunctionDefs, state.functionDef)
			state.functionDef = Function{}
			state.funcKeywordTokenIdx = -1
			state.curArg.Name = ""
			return state, 1, nil
		} else if diffDelta == 1 {
			switch tokens[i].Token {
			case "SYMBOL_FORMALS":
				state.curArg.Name = tokens[i].Text
			case "EQ_FORMALS":
				state.nextIsFormalDefault = true
			case "','":
				state.functionDef.Args = append(state.functionDef.Args, state.curArg)
				state.curArg.Name = ""
				state.curArg.Default = ""
			default:
				if state.nextIsFormalDefault {
					state.curArg.Default = tokens[i].Text
					state.nextIsFormalDefault = false
				}
			}
		}
	}
	return state, 1, nil
}
