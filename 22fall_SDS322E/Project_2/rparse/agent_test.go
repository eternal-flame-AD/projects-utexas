package rparse

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAgentCommandPing(t *testing.T) {
	agent := new(Agent)
	err := agent.Start("")
	assert.NoError(t, err)
	defer agent.Stop()

	pingCmd := agentCommand{
		OpCode: "ping",
		Args:   []string{},
	}
	data, err := agent.IssueCmd(pingCmd)
	assert.NoError(t, err)
	assert.Equal(t, [][]string{{"pong"}}, data)
}

func TestAgentCommandParse(t *testing.T) {
	exampleCode := `
	# this is a hello world function
	hello <- function(foo, bar) 
		sprintf("%s, %s and %s!", "world", foo, bar);
	`
	agent := new(Agent)
	err := agent.Start("")
	assert.NoError(t, err)
	defer agent.Stop()

	for retries := 0; retries < 5; retries++ {

		fileName := fmt.Sprintf("hello_%d.R", retries)
		tokens, err := agent.CmdParseText(fileName, exampleCode)
		assert.NoError(t, err)

		containTokens := make(map[string]bool)
		for _, token := range tokens {
			assert.Equal(t, fileName, token.Filename)
			containTokens[token.Token] = true
			assert.NotEmpty(t, token.Text)
		}
		assert.True(t, containTokens["SYMBOL_FORMALS"])
		assert.True(t, containTokens["SYMBOL_FUNCTION_CALL"])
		assert.True(t, containTokens["FUNCTION"])
		assert.True(t, containTokens["LEFT_ASSIGN"])
	}
}
