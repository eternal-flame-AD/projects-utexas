package main

import (
	// #include <R.h>
	// #include <Rinternals.h>
	"C"

	"Project2/model"
	"encoding/json"
	"log"
	"os"
)
import (
	"Project2/feature"
	"fmt"
	"io"
	"sync"
)

//export ExtractFeatures
func ExtractFeatures(filenameSexp C.SEXP, nProcsSexp C.SEXP) C.SEXP {
	filename := GoString(filenameSexp)[0]
	nProcs := int(C.asInteger(nProcsSexp))
	f, err := os.Open(filename)
	if err != nil {
		log.Fatalf("Failed to open file %s: %v", filename, err)
	}
	wg := new(sync.WaitGroup)
	inputChan := make(chan *model.P, nProcs)
	outputChan := make(chan model.F, nProcs)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(inputChan)
		dec := json.NewDecoder(f)
		for {
			p := new(model.P)
			if err := dec.Decode(p); err != nil {
				if err == io.EOF {
					break
				}
				log.Panicf("Aborted: could not decode output file: %v", err)
			}
			inputChan <- p
		}
	}()
	for i := 0; i < nProcs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range inputChan {
				f, err := feature.Extract(p)
				if err != nil {
					log.Printf("Error extracting feature: [%s] %s", p.Description.Package, err)
					continue
				}
				outputChan <- *f
			}
		}()
	}
	go func() {
		wg.Wait()
		close(outputChan)
	}()

	var df C.SEXP

	count := 0
	for f := range outputChan {
		//fmt.Printf("output is %v", f.KVPairs())
		row := RDataFrame(MakeRList(f.KVPairs()))
		rProtect(row)
		if count == 0 {
			df = row
		} else {
			rUnprotect(1)
			df = RRbind(df, row)
		}
		rUnprotect(1)
		rProtect(df)
		count++
		fmt.Printf("\rProcessed %d packages", count)
	}
	rUnprotect(1)
	return df
}
