package main

import (
	"Project2/model"
	"Project2/rparse"
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

type Parser struct {
	tmpRFile     *os.File
	rParserAgent rparse.Agent

	currentPackage *model.P
}

func extractPackages(urls []string, outputType string, nProcs int) []string {
	ret := make([]string, len(urls))
	skipURLs := make(map[string]bool)
	var output func(i int, res model.P) error
	switch outputType {
	case "":
		output = func(i int, pkg model.P) error {
			pkgJSON, err := json.MarshalIndent(pkg, "", "  ")
			if err != nil {
				return err
			}
			ret[i] = string(pkgJSON)
			return nil
		}
	default:
		var of *os.File
		if _, err := os.Stat(outputType); err == nil {
			of, err = os.OpenFile(outputType, os.O_APPEND|os.O_RDWR, 0640)
			if err != nil {
				return []string{fmt.Sprintf("Aborted: could not open output file: %v", err)}
			}
			if _, err := of.Seek(0, io.SeekStart); err != nil {
				return []string{fmt.Sprintf("Aborted: could not seek to start of output file: %v", err)}
			}
			dec := json.NewDecoder(of)
			for {
				var res model.P
				if err := dec.Decode(&res); err != nil {
					if err == io.EOF {
						break
					}
					return []string{fmt.Sprintf("Aborted: could not decode output file: %v", err)}
				}
				skipURLs[res.URL] = true
			}
		} else if !os.IsNotExist(err) {
			return []string{fmt.Sprintf("Aborted: could not stat() output file: %v", err)}
		} else {
			of, err = os.Create(outputType)
			if err != nil {
				return []string{fmt.Sprintf("Aborted: error opening output file: %s", err)}
			}
		}

		defer of.Close()
		outMutex := new(sync.Mutex)
		enc := json.NewEncoder(of)
		enc.SetIndent("", "  ")
		output = func(i int, pkg model.P) error {
			outMutex.Lock()
			defer outMutex.Unlock()
			return enc.Encode(pkg)
		}
	}

	parsers := make([]Parser, nProcs)
	wg := new(sync.WaitGroup)
	workerChan := make(chan int)
	for i := 0; i < int(nProcs); i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			parser := &parsers[i]
			if err := parser.rParserAgent.Start(""); err != nil {
				log.Fatalf("could not start R parser agent: %v", err)
			}
			for idx := range workerChan {
				url := urls[idx]
				if skipURLs[url] {
					continue
				}
				var res *model.P
				if err := parser.ParseProjectURL(url); err != nil {
					res = &model.P{FetchError: err.Error()}
				} else {
					res = parser.GetParseResult()
				}
				if err := output(idx, *res); err != nil {
					ret[idx] = fmt.Sprintf("error writing output: %s", err)
				}
			}
		}(i)
	}

	fmt.Printf("Starting to fetch %d packages with %d threads...\n", len(urls), nProcs)
	startTime := time.Now()
	for i := range urls {
		workerChan <- i
		fmt.Printf("\rFetched %d/%d (%0.2f%%) ETA: %s", i+1, len(urls),
			float64(i+1)/float64(len(urls))*100,
			formatDuration(time.Since(startTime)/time.Duration(i+1)*time.Duration(len(urls)-i-1)))
		agentStats := new(rparse.AgentStats)
		for i := range parsers {
			agentStats.Add(parsers[i].rParserAgent.Stats)
		}
		fmt.Printf(" (R slave: %s )", agentStats.String())
	}
	close(workerChan)
	wg.Wait()
	return ret
}

func (p *Parser) ParseProjectTar(tarFile *tar.Reader) error {
	p.currentPackage = model.NewP()

	hasDescription := false
	hasNamespace := false
	for {
		header, err := tarFile.Next()
		if err != nil && err == io.EOF {
			if !hasDescription {
				p.currentPackage.ParseError = append(p.currentPackage.ParseError, model.ParseError{Stage: "DESCRIPTION", Message: "DESCRIPTION file not found"})
			}
			if !hasNamespace {
				p.currentPackage.ParseError = append(p.currentPackage.ParseError, model.ParseError{Stage: "NAMESPACE", Message: "NAMESPACE file not found"})
			}
			return nil
		} else if err != nil {
			return err
		}

		fileName := header.Name
		p.currentPackage.Files = append(p.currentPackage.Files, fileName)
		fileName = fileName[strings.IndexByte(fileName, '/'):]
		if fileName == "/DESCRIPTION" {
			p.catchParseError("DESCRIPTION", header.Name, func() {
				hasDescription = true
				p.ParseDescriptionFile(tarFile)
			})
		} else if fileName == "/NAMESPACE" {
			p.catchParseError("NAMESPACE", header.Name, func() {
				hasNamespace = true
				p.ParseNamespaceFile(tarFile)
			})
		} else {
			ext := filepath.Ext(fileName)
			ext = strings.ToLower(ext)
			if ext == "" {
				ext = "NONE"
			}
			p.currentPackage.FileExtensions[ext]++
			if strings.HasPrefix(fileName, "/R/") && ext == ".r" {
				p.catchParseError("SOURCE_R", header.Name, func() {
					p.ParseRFile(fileName, tarFile)
				})
			}
		}
	}
}

func (p *Parser) ParseProjectURL(url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	gzReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	defer gzReader.Close()
	tarReader := tar.NewReader(gzReader)
	if err := p.ParseProjectTar(tarReader); err != nil {
		return err
	}
	p.currentPackage.URL = url
	return err
}

func (p *Parser) catchParseError(stage string, file string, parseFunc func()) {
	func() {
		defer func() {
			if err := recover(); err != nil {
				perr := model.ParseError{
					Stage:   stage,
					Message: fmt.Sprintf("%v", err),
					Stack:   string(debug.Stack()),
				}
				p.currentPackage.ParseError = append(p.currentPackage.ParseError, perr)
			}
		}()
		parseFunc()
	}()
}
