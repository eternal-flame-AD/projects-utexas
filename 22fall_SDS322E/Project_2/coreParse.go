package main

import (
	"Project2/model"
	"Project2/rparse"
	"Project2/rparse/matcher"
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"strings"
)

var rDescriptionHeader = regexp.MustCompile(`^([\w@\.]+):\s+(.*)$`)

func (p *Parser) GetParseResult() *model.P {
	return p.currentPackage
}

func (p *Parser) ParseRFile(filename string, rFile io.Reader) {
	if p.tmpRFile == nil {
		tmpRFile, err := os.CreateTemp("", "rparse_tmp_*")
		if err != nil {
			log.Fatalf("Failed to create temporary file: %v", err)
		}
		p.tmpRFile = tmpRFile
	}
	if err := p.tmpRFile.Truncate(0); err != nil {
		log.Fatalf("Failed to truncate temporary file: %v", err)
	}
	if _, err := p.tmpRFile.Seek(0, io.SeekStart); err != nil {
		log.Fatalf("Failed to seek to start of temporary file: %v", err)
	}
	if _, err := io.Copy(p.tmpRFile, rFile); err != nil {
		p.currentPackage.ParseError = append(p.currentPackage.ParseError, model.ParseError{Stage: "R", Message: fmt.Sprintf("Error reading file %s: %v", filename, err)})
	}
	tokenList, err := p.rParserAgent.CmdParseFile(filename, p.tmpRFile.Name())
	if err != nil {
		p.currentPackage.ParseError = append(p.currentPackage.ParseError, model.ParseError{Stage: "R", Message: fmt.Sprintf("Error parsing file %s: %v", filename, err)})
		return
	}

	p.currentPackage.RFiles = append(p.currentPackage.RFiles, model.RFile{
		Name:    filename,
		NTokens: len(tokenList),
	})
	if len(tokenList) == 0 {
		return
	}
	if err := rparse.RunTokenMatcher(matcher.TrackParenthesis, tokenList, matcher.TrackParenthesisUpdate); err != nil {
		p.currentPackage.ParseError = append(p.currentPackage.ParseError, model.ParseError{Stage: "R", Message: fmt.Sprintf("Error matching parenthesis in file %s: %v", filename, err)})
		return
	}
	if err := rparse.RunTokenMatcher(matcher.MatchAssignment, tokenList, matcher.MatchAssignmentUpdate); err != nil {
		p.currentPackage.ParseError = append(p.currentPackage.ParseError, model.ParseError{Stage: "R", Message: fmt.Sprintf("Error matching assignments in file %s: %v", filename, err)})
		return
	}
	if err := rparse.RunTokenMatcher(matcher.MatchFunctionDef, tokenList, matcher.MatchFunctionDefUpdate); err != nil {
		p.currentPackage.ParseError = append(p.currentPackage.ParseError, model.ParseError{Stage: "R", Message: fmt.Sprintf("Error matching function definitions in file %s: %v", filename, err)})
		return
	}
	if err := rparse.RunTokenMatcher(matcher.MatchLibraryCalls, tokenList, matcher.MatchLibraryCallsUpdate); err != nil {
		p.currentPackage.ParseError = append(p.currentPackage.ParseError, model.ParseError{Stage: "R", Message: fmt.Sprintf("Error matching library calls in file %s: %v", filename, err)})
		return
	}
	if err := rparse.RunTokenMatcher(matcher.MatchFunctionCall, tokenList, matcher.MatchFunctionCallUpdate); err != nil {
		p.currentPackage.ParseError = append(p.currentPackage.ParseError, model.ParseError{Stage: "R", Message: fmt.Sprintf("Error matching function calls in file %s: %v", filename, err)})
		return
	}
	p.currentPackage.RFiles[len(p.currentPackage.RFiles)-1].Stats = map[string]interface{}{
		matcher.MatchAssignment:   tokenList.FinalMatcherState(matcher.MatchAssignment),
		matcher.MatchFunctionDef:  tokenList.FinalMatcherState(matcher.MatchFunctionDef),
		matcher.TrackParenthesis:  tokenList.FinalMatcherState(matcher.TrackParenthesis),
		matcher.MatchLibraryCalls: tokenList.FinalMatcherState(matcher.MatchLibraryCalls),
		matcher.MatchFunctionCall: tokenList.FinalMatcherState(matcher.MatchFunctionCall),
	}
}
func (p *Parser) ParseDescriptionFile(descFile io.Reader) {
	scanner := bufio.NewScanner(descFile)
	var descFieldName string
	var descFieldValue string
	for scanner.Scan() {
		line := scanner.Text()
		if subMatch := rDescriptionHeader.FindStringSubmatch(line); subMatch != nil {
			if descFieldName != "" {
				descFieldValue = strings.TrimSpace(descFieldValue)
				targetField := reflect.ValueOf(&p.currentPackage.Description).Elem().
					FieldByNameFunc(func(name string) bool {
						return strings.EqualFold(name, descFieldName)
					})
				if targetField.IsValid() {
					switch targetField.Kind() {
					case reflect.String:
						targetField.SetString(descFieldValue)
					case reflect.Slice:
						switch targetField.Type().Elem().Kind() {
						case reflect.String:
							fields := strings.Split(descFieldValue, ",")
							for i, field := range fields {
								field = strings.TrimSpace(field)
								if match := regexp.MustCompile(`^([\w\.]+)\s*\((.+)\)$`).FindStringSubmatch(field); match != nil {
									field = match[1]
								}
								fields[i] = field
							}
							targetField.Set(reflect.ValueOf(fields))
						}
					}
				}
				descFieldName = ""
			}
			descFieldName = subMatch[1]
			descFieldValue = subMatch[2]
		} else {
			descFieldValue += " " + strings.TrimSpace(line)
		}
	}
}

func (p *Parser) ParseNamespaceFile(nameFile io.Reader) {
	rParseCmd := exec.Command(
		"Rscript", "--vanilla", "-e",
		strings.Join([]string{
			`write.csv(`,
			`getParseData(parse(file="stdin", keep.source=TRUE))[c("token", "text")]`,
			`,row.names=FALSE)`,
		}, ""))
	rParseCmd.Stdin = nameFile
	rParseCmd.Stderr = os.Stderr
	stdout, err := rParseCmd.StdoutPipe()
	if err != nil {
		log.Panicf("failed to get stdout pipe: %v", err)
	}
	defer stdout.Close()

	if err := rParseCmd.Start(); err != nil {
		log.Panicf("failed to start Rscript: %v", err)
	}

	csvReader := csv.NewReader(stdout)
	header, err := csvReader.Read()
	if err != nil {
		log.Panicf("failed to read header: %v", err)
	}
	if len(header) != 2 || header[0] != "token" || header[1] != "text" {
		log.Panicf("unexpected header: %v", header)
	}

	tokens := make([][2]string, 0)
	for {
		rec, err := csvReader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Panicf("failed to read csv record: %v", err)
		}
		token := rec[0]
		text := rec[1]
		if text == "" {
			// not termination token
			continue
		}

		tokens = append(tokens, [2]string{token, text})
	}
	ptr := 0
	args := make([]string, 0, len(tokens)-1)
	opts := make(map[string]string)
	optName := ""
	parenDepth := 0
	topLevelFunction := ""
	for ptr < len(tokens) {
		token := tokens[ptr][0]
		text := tokens[ptr][1]
		switch token {
		case "SYMBOL_FUNCTION_CALL":
			if parenDepth != 0 {
				p.currentPackage.ParseError = append(p.currentPackage.ParseError, model.ParseError{
					Stage:   "NAMESPACE",
					File:    "/NAMESPACE",
					Message: fmt.Sprintf("unexpected function call at depth %d: %v", parenDepth, text),
				})
				ptr++
				continue
			} else {
				topLevelFunction = text
				ptr++
			}
		case "','":
			ptr++
		case "'('":
			parenDepth++
			ptr++
		case "')'":
			parenDepth--
			ptr++
		case "STR_CONST":
			if text[0] == '"' || text[0] == '\'' {
				text = text[1 : len(text)-1]
			}
			fallthrough
		case "SYMBOL", "NUM_CONST":
			if optName == "" {
				args = append(args, text)
			} else {
				opts[optName] = text
				optName = ""
			}
			ptr++
		case "SYMBOL_SUB":
			if tokens[ptr+1][0] != "EQ_SUB" {
				p.currentPackage.ParseError = append(p.currentPackage.ParseError, model.ParseError{
					Stage:   "NAMESPACE",
					File:    "/NAMESPACE",
					Message: fmt.Sprintf("unexpected token after SYMBOL_SUB: EQ_SUB exptected, got %v", tokens[ptr+1]),
				})
				ptr++
			} else {
				optName = text
				ptr += 2
			}
		default:
			p.currentPackage.ParseError = append(p.currentPackage.ParseError, model.ParseError{
				Stage:   "NAMESPACE",
				File:    "/NAMESPACE",
				Message: fmt.Sprintf("don't know what to do with token: %s text=%v", token, text),
			})
			ptr++
		}
		if parenDepth == 0 && topLevelFunction != "" && (len(args)+len(opts) > 0) {
			p.currentPackage.Namespace.Calls = append(p.currentPackage.Namespace.Calls, model.NamespaceCall{
				Name: topLevelFunction,
				Args: args,
				Opts: opts,
			})
			switch topLevelFunction {
			case "export", "exportClasses", "exportMethods":
				p.currentPackage.Namespace.Exports = append(p.currentPackage.Namespace.Exports, args...)
			case "import":
				p.currentPackage.Namespace.Imports = append(p.currentPackage.Namespace.Imports, args...)
			case "importFrom", "importClassesFrom", "importMethodsFrom":
				pkg := args[0]
				for _, arg := range args[1:] {
					p.currentPackage.Namespace.Imports = append(p.currentPackage.Namespace.Imports, pkg+"::"+arg)
				}
			case "S3method":
				p.currentPackage.Namespace.Exports = append(p.currentPackage.Namespace.Exports, args[0]+"."+args[1])
			default:
				p.currentPackage.ParseError = append(p.currentPackage.ParseError, model.ParseError{
					Stage:   "NAMESPACE",
					File:    "/NAMESPACE",
					Message: fmt.Sprintf("dont know what to do with top-level function call: %v", topLevelFunction),
				})
			}
			args = args[:0]
			opts = make(map[string]string)
		}
	}
	rParseCmd.Wait()
}
