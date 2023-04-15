package rparse

import (
	"errors"
)

type RTokenList []RToken

func (l RTokenList) FinalMatcherState(name string) any {
	return l[len(l)-1].MatcherState[name]
}

func RunTokenMatcher[S any](
	name string,
	tokenList RTokenList,
	matcher func(lastState S, i int, tokens RTokenList) (next S, delta int, err error)) error {
	var state S
	i := 0
	for {
		if i < 0 {
			return errors.New("negative index")
		} else if i >= len(tokenList) {
			return nil
		}
		if tokenList[i].MatcherState == nil {
			tokenList[i].MatcherState = make(map[string]any)
		}
		//log.Printf("token %d: %s, state=%v", i, tokenList[i].Token, state)
		tokenList[i].MatcherState[name] = state

		next, delta, err := matcher(state, i, tokenList)
		if err != nil {
			return err
		}
		i += delta
		state = next
	}
}
