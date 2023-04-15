package matcher

import (
	"Project2/rparse"
	"errors"
	"fmt"
)

const TrackParenthesis = "paren"

type TrackParenthesisState struct {
	done  bool
	Stack []byte
}

func (s TrackParenthesisState) Clone() TrackParenthesisState {
	return TrackParenthesisState{
		Stack: append([]byte(nil), s.Stack...),
		done:  s.done,
	}
}

func (s TrackParenthesisState) Depth(paren byte) int {
	if paren != '(' && paren != '{' {
		panic(fmt.Sprintf("invalid parenthesis: %c", paren))
	}
	depth := 0
	for _, c := range s.Stack {
		if c == paren {
			depth++
		}
	}
	return depth
}

func TrackParenthesisUpdate(state TrackParenthesisState, i int, tokens rparse.RTokenList) (next TrackParenthesisState, delta int, err error) {
	state = state.Clone()
	pushStack := func(c byte) {
		state.Stack = append(state.Stack, c)
	}
	popStack := func(expect byte) error {
		if len(state.Stack) == 0 {
			return errors.New("mismatched parenthesis")
		}
		if state.Stack[len(state.Stack)-1] != expect {
			return fmt.Errorf("mismatched parenthesis, exptected %c got %c", expect, state.Stack[len(state.Stack)-1])
		}
		state.Stack = state.Stack[:len(state.Stack)-1]
		return nil
	}
	token := tokens[i]
	switch token.Token {
	case "'('":
		if !state.done {
			pushStack('(')
			state.done = true
			return state, 0, nil
		} else {
			state.done = false
		}
	case "'{'":
		if !state.done {
			pushStack('{')
			state.done = true
			return state, 0, nil
		} else {
			state.done = false
		}
	case "')'":
		if err := popStack('('); err != nil {
			return state, 0, err
		}
	case "'}'":
		if err := popStack('{'); err != nil {
			return state, 0, err
		}
	}
	return state, 1, nil
}
