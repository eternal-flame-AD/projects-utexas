package matcher

import (
	"Project2/rparse"
	"fmt"
	"strings"
)

const MatchLibraryCalls = "library"

type LibraryCall struct {
	Method    string
	Namespace string
}
type MatchLibraryCallsState struct {
	Errors        []string
	NamespaceUsed []string
	LibraryCalls  []LibraryCall
}

const packageLoadFunctions = "library;require;attachNamespace;loadNamespace;requireNamespace;"

func MatchLibraryCallsUpdate(state MatchLibraryCallsState, i int, tokens rparse.RTokenList) (next MatchLibraryCallsState, delta int, err error) {
	if tokens[i].Token == "SYMBOL_PACKAGE" {
		if !contains(state.NamespaceUsed, tokens[i].Text) {
			state.NamespaceUsed = append(state.NamespaceUsed, tokens[i].Text)
		}
		state.LibraryCalls = append(state.LibraryCalls, LibraryCall{
			Method:    tokens[i].Token,
			Namespace: tokens[i].Text,
		})
	} else if tokens[i].Token == "SYMBOL_FUNCTION_CALL" {
		if strings.Contains(packageLoadFunctions, tokens[i].Text+";") {
			if tokens[i+1].Token != "'('" {
				state.Errors = append(state.Errors, "library call missing '('")
				return state, 1, nil
			} else {
				ns := tokens[i+2].Text
				if tokens[i+2].Token == "STR_CONST" {
					if strings.HasPrefix(ns, "\"") {
						ns = strings.Trim(ns, "\"")
					} else if strings.HasPrefix(ns, "'") {
						ns = strings.Trim(ns, "'")
					}
				} else if tokens[i+2].Token != "SYMBOL" {
					state.Errors = append(state.Errors, fmt.Sprintf("unexpected token %s trying to resolve library call at %d", tokens[i+2].Token, i+2))
					return state, 1, nil
				}
				if !contains(state.NamespaceUsed, ns) {
					state.NamespaceUsed = append(state.NamespaceUsed, ns)
				}
				state.LibraryCalls = append(state.LibraryCalls, LibraryCall{
					Method:    tokens[i].Text,
					Namespace: ns,
				})
			}
		}
	}
	return state, 1, nil
}
