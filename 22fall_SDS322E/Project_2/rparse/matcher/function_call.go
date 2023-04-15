package matcher

import (
	"Project2/rparse"
	"strings"
)

const MatchFunctionCall = "function_call"

type FunctionCallArg struct {
	Name  string
	Value string
}

type FunctionCall struct {
	Name string
	Args []FunctionCallArg
}

type MatchFunctionCallState struct {
	curCallIdx int

	inSub              bool
	thisCallParenDepth int
	thisCall           FunctionCall

	Errors             []string
	StatsFunctionCalls []FunctionCall
}

func MatchFunctionCallUpdate(state MatchFunctionCallState, i int, tokens rparse.RTokenList) (next MatchFunctionCallState, delta int, err error) {
	parenStack := tokens[i].MatcherState[TrackParenthesis].(TrackParenthesisState).Stack
	if state.thisCall.Name != "" {
		if len(parenStack) == state.thisCallParenDepth {
			// finish parsing this function call
			state.StatsFunctionCalls = append(state.StatsFunctionCalls, state.thisCall)
			state.thisCall = FunctionCall{}
			state.thisCallParenDepth = 0
			state.inSub = false
			return state, 1, nil
		}
		token := tokens[i].Token
		if token == "SYMBOL" || strings.HasSuffix(token, "_CONST") {
			if state.inSub {
				state.thisCall.Args[len(state.thisCall.Args)-1].Value = tokens[i].Text
				state.inSub = false
			} else {
				state.thisCall.Args = append(state.thisCall.Args, FunctionCallArg{
					Name:  "",
					Value: tokens[i].Text,
				})
			}
		} else if token == "SYMBOL_SUB" {
			state.thisCall.Args = append(state.thisCall.Args, FunctionCallArg{
				Name:  tokens[i].Text,
				Value: "",
			})
			state.inSub = true
		}
		return state, 1, nil
	}

	if tokens[i].Token == "SYMBOL_FUNCTION_CALL" {
		if tokens[i+1].Token != "'('" {
			state.Errors = append(state.Errors, "function call missing '('")
			return state, 1, nil
		}
		state.thisCall.Name = tokens[i].Text
		state.thisCall.Args = []FunctionCallArg{}
		state.curCallIdx = i
		state.thisCallParenDepth = len(parenStack)
		return state, 2, nil
	}

	return state, 1, nil
}
