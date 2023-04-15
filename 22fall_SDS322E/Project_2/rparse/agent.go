package rparse

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type agentCommand struct {
	OpCode string `json:"opCode"`
	Args   []string
}

type AgentStats struct {
	Start uint64
	Kill  uint64
	Err   uint64
	OK    uint64
}

func (a AgentStats) String() string {
	return fmt.Sprintf("Start: %d, Kill: %d, Err: %d, OK: %d", a.Start, a.Kill, a.Err, a.OK)
}

func (a *AgentStats) Collect(agents ...*Agent) {
	a.Err = 0
	a.Kill = 0
	a.OK = 0
	a.Start = 0
	for _, agent := range agents {
		a.Start += agent.Stats.Start
		a.Kill += agent.Stats.Kill
		a.Err += agent.Stats.Err
		a.OK += agent.Stats.OK
	}
}

func (a *AgentStats) Add(stats ...AgentStats) {
	for _, stat := range stats {
		a.Start += stat.Start
		a.Kill += stat.Kill
		a.Err += stat.Err
		a.OK += stat.OK
	}
}

type Agent struct {
	Stats     AgentStats
	rPath     string
	sep       string
	eol       string
	cmd       *exec.Cmd
	input     *bufio.Writer
	stdout    *bufio.Scanner
	busyMutex sync.Mutex
	Debug     bool
}

type RToken struct {
	Filename string
	Token    string
	Text     string

	MatcherState map[string]any
}

func (a *Agent) Start(Rpath string) (err error) {
	if Rpath == "" {
		Rpath = "Rscript"
	}

	a.rPath = Rpath
	a.cmd = exec.Command(
		Rpath, "--vanilla", "--slave", "-")
	//"cat")

	a.sep = fmt.Sprintf("RPARSE_sep_magic_%x", time.Now().UnixNano())
	a.eol = fmt.Sprintf("RPARSE_eol_magic_%x", time.Now().UnixNano())
	prelude := strings.ReplaceAll(strings.Join([]string{
		"suppressMessages(library(dplyr));",
		"stdin <- file('stdin', 'rb');",
		"stdout <- stdout();",
		"output <- function(type, data) {",
		"  write.table(cbind(type, data), file=stdout, sep='" + a.sep + "', eol='" + a.eol + "\\n', row.names=FALSE, col.names=FALSE, qmethod='double');",
		"};",
		"handle.input.real <- function(input) {",
		"	switch(input$opCode,",
		"		ping = {",
		"			output('data', 'pong');",
		"		},",
		"		quit = {",
		"			output('data', 'bye');",
		"			q('no');",
		"		},",
		"       parse_text = {",
		"           parse.data <- getParseData(",
		"               parse(text=input$args[[2]], keep.source=TRUE));",
		"           parse.data <- filter(parse.data, terminal == TRUE);",
		"           parse.data <- select(parse.data, token, text);",
		"           parse.data <- bind_cols(name = input$args[[1]], parse.data);",
		"           output('data', parse.data);",
		"       },",
		"       parse_file = {",
		"           parse.data <- getParseData(",
		"               parse(file=input$args[[2]], keep.source=TRUE));",
		"           parse.data <- filter(parse.data, terminal == TRUE);",
		"           parse.data <- select(parse.data, token, text);",
		"           parse.data <- bind_cols(name = input$args[[1]], parse.data);",
		"           output('data', parse.data);",
		"       }",
		"    );",
		"};",
		"handle.input <- function(input) {",
		"	if (!is.null(input)) {",
		"		tryCatch(handle.input.real(input), error = function(err) output('error', as.character(err)), finally = {",
		"			output('done', NULL);",
		"			flush(stdout);",
		"		});",
		"	};",
		"};\n",
	}, ""), "\t", " ")

	if stdin, err := a.cmd.StdinPipe(); err != nil {
		return err
	} else {
		a.input = bufio.NewWriter(stdin)
	}
	a.cmd.Stderr = os.Stderr
	if stdout, err := a.cmd.StdoutPipe(); err != nil {
		return err
	} else {
		a.stdout = bufio.NewScanner(stdout)
	}
	//a.cmd.Stdout = os.Stdout
	if err := a.cmd.Start(); err != nil {
		return err
	}
	if _, err := a.input.Write([]byte(prelude)); err != nil {
		return err
	}
	if err := a.input.Flush(); err != nil {
		return err
	}

	a.Stats.Start++
	return nil
}

func (a *Agent) CmdParseFile(filename string, path string) (tokens RTokenList, err error) {
	data, err := a.IssueCmd(agentCommand{
		OpCode: "parse_file",
		Args:   []string{filename, path},
	})
	if err != nil {
		return
	}
	for _, row := range data {
		tokens = append(tokens, RToken{
			Filename: row[0],
			Token:    row[1],
			Text:     row[2],
		})
	}
	return
}

func (a *Agent) CmdParseText(filename string, text string) (tokens RTokenList, err error) {
	data, err := a.IssueCmd(agentCommand{
		OpCode: "parse_text",
		Args:   []string{filename, text},
	})
	if err != nil {
		return
	}
	for _, row := range data {
		tokens = append(tokens, RToken{
			Filename: row[0],
			Token:    row[1],
			Text:     row[2],
		})
	}
	return
}

func writeHexString(out *bufio.Writer, s string) (err error) {
	if len(s) > 50 {
		if _, err := fmt.Fprintf(out, "paste0("); err != nil {
			return err
		}
		for len(s) > 50 {
			if err := writeHexString(out, s[:50]); err != nil {
				return err
			}
			if _, err := fmt.Fprint(out, ",\n"); err != nil {
				return err
			}
			if err := out.Flush(); err != nil {
				return err
			}
			s = s[50:]
		}
		if err := writeHexString(out, s); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, ")"); err != nil {
			return err
		}
		return nil
	}

	if _, err := out.Write([]byte("'")); err != nil {
		return err
	}
	for _, c := range s {
		if c == '\x00' {
			c = '\n'
		}
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == '_' || c == '-' || c == '.' || c == '{' || c == '}' || c == '[' || c == ']' || c == '(' || c == ')' || c == ',' || c == ':' || c == ';' || c == ' ' {
			if _, err := out.Write([]byte{byte(c)}); err != nil {
				return err
			}
		} else {
			if _, err := fmt.Fprintf(out, "\\u%04x", c); err != nil {
				return err
			}
		}
	}
	if _, err := out.Write([]byte("'\n\n")); err != nil {
		return err
	}
	if err := out.Flush(); err != nil {
		return err
	}
	return nil
}

func (a *Agent) IssueCmd(cmd agentCommand) (data [][]string, err error) {
	a.busyMutex.Lock()
	defer a.busyMutex.Unlock()

	kill := func(reason string) {
		a.Stats.Kill++
		if err := a.Stop(); err != nil {
			log.Printf("Error stopping agent: %s", err)
		} else {
			log.Printf("Killed R agent: %s", reason)
		}
	}
	watchdogDone := make(chan struct{})
	done := make(chan struct{})
	defer func() {
		<-watchdogDone
	}()
	defer close(done)
	go func() {
		timeout := time.After(40 * time.Second)
		select {
		case <-done:
		case <-timeout:
			kill("watchdog timeout")
		}
		close(watchdogDone)
	}()

	if a.cmd == nil || a.cmd.Process == nil || a.cmd.ProcessState != nil && a.cmd.ProcessState.Exited() {
		if err := a.Start(""); err != nil {
			return nil, err
		}
	}
	if _, err := fmt.Fprintf(a.input, "handle.input(list(opCode=\"%s\", args=list(", cmd.OpCode); err != nil {
		kill(err.Error())
		return nil, err
	}
	for i, arg := range cmd.Args {
		if i > 0 {
			if _, err := fmt.Fprintf(a.input, ",\n"); err != nil {
				kill(err.Error())
				return nil, err
			}
			if err := a.input.Flush(); err != nil {
				kill(err.Error())
				return nil, err
			}
		}
		if err := writeHexString(a.input, arg); err != nil {
			kill(err.Error())
			return nil, err
		}
	}
	if _, err := fmt.Fprintf(a.input, ")));\n\n"); err != nil {
		kill(err.Error())
		return nil, err
	}
	if err := a.input.Flush(); err != nil {
		kill(err.Error())
		return nil, err
	}

	for {
		if !a.stdout.Scan() {
			kill("unexpected EOF")
			return nil, errors.New("R agent died")
		}
		line := a.stdout.Text()
		if line == "" {
			continue
		}
		if line[0] != '"' {
			log.Printf("Spurious line from R agent: %s", line)
			continue
		}
		for !strings.HasSuffix(line, a.eol) {
			if !a.stdout.Scan() {
				kill("unexpected EOF")
				return nil, errors.New("R agent died")
			}
			line += a.stdout.Text()
		}
		line = line[:len(line)-len(a.eol)]
		record := strings.Split(line, a.sep)
		for i := range record {
			record[i] = strings.Trim(record[i], "\"")
			record[i] = strings.ReplaceAll(record[i], "\"\"", "\"")
		}
		if record[0] == "done" {
			if err == nil {
				a.Stats.OK++
			}
			return data, err
		} else if record[0] == "error" {
			a.Stats.Err++
			err = errors.New(record[1])
		} else if record[0] == "data" {
			data = append(data, record[1:])
		} else {
			log.Printf("Spurious output from R agent: %v", line)
		}
	}
}

func (a *Agent) Stop() error {
	if err := a.cmd.Process.Kill(); err != nil {
		return err
	}
	a.cmd.Wait()
	a.cmd = nil
	return nil
}
