package matcher

import (
	"Project2/rparse"
	"fmt"
)

const MatchAssignment = "assign"

type Variable struct {
	Name       string
	AssignType string
	RHSType    string
}

type MatchAssignmentState struct {
	initialized    bool
	assignTokenIdx int
	assignName     string

	partAssignMode          string
	partAssignParenStackLen int

	Errors               []string
	StatsVariables       []Variable
	StatsEqAssignCount   int
	StatsLeftAssignCount int
}

func MatchAssignmentUpdate(state MatchAssignmentState, i int, tokens rparse.RTokenList) (next MatchAssignmentState, delta int, err error) {
	if !state.initialized {
		state.initialized = true
		state.assignTokenIdx = -1
		return state, 0, nil
	}
	if state.assignName != "" {
		if i != state.assignTokenIdx {
			return state, state.assignTokenIdx - i, nil
		}
		rhsType := tokens[i+1].Token
		state.StatsVariables = append(state.StatsVariables, Variable{
			Name:       state.assignName,
			AssignType: tokens[i].Token,
			RHSType:    rhsType,
		})
		state.assignName = ""
		state.assignTokenIdx = -1
		return state, 1, nil
	}
	thisParenStackLen := len(tokens[i].MatcherState[TrackParenthesis].(TrackParenthesisState).Stack)
	if state.partAssignParenStackLen > 0 && state.partAssignParenStackLen == thisParenStackLen {
		if tokens[i].Token == "SYMBOL" {
			state.assignName = tokens[i].Text
		} else {
			state.Errors = append(state.Errors, fmt.Sprintf("unexpected token %s trying to resolve ']' assign at %d", tokens[i].Token, i))
		}
		return state, state.assignTokenIdx - i, nil
	}

	if state.assignTokenIdx > i {
		if i == 0 || tokens[i-1].Token != "'$'" && tokens[i-1].Token != "'@'" {
			if tokens[i].Token == "SYMBOL" {
				state.assignName = tokens[i].Text
			} else if tokens[i].Token == "']'" {
				state.partAssignMode = tokens[i].Token
			} else if tokens[i].Token == "')'" {
				// attributes(x)...
			} else {
				state.Errors = append(state.Errors, fmt.Sprintf("unexptected token before assignment %s", tokens[i].Token))
			}
		}
		return state, state.assignTokenIdx - i + 1, nil
	}
	isAssign := false
	if tokens[i].Token == "EQ_ASSIGN" {
		isAssign = true
		state.StatsEqAssignCount++
	} else if tokens[i].Token == "LEFT_ASSIGN" {
		isAssign = true
		state.StatsLeftAssignCount++
	}
	if isAssign {
		state.assignName = ""
		state.assignTokenIdx = i
		state.partAssignParenStackLen = 0
		state.partAssignMode = ""
		return state, -1, nil
	}

	return state, 1, nil
}
