package main

import (
	"encoding/csv"
	"flag"
	"io"
	"log"
	"os"
)

var flagPackagesCsv = flag.String("packages", "package.csv", "CSV file with package URLs")
var flagOutput = flag.String("output", "output.json", "Output type (vector or file)")
var flagNumProcs = flag.Int("procs", 8, "Number of parallel processes")

func main() {
	flag.Parse()

	csvFileIO, err := os.Open(*flagPackagesCsv)
	if err != nil {
		log.Fatalf("Error opening CSV file: %s", err)
	}
	defer csvFileIO.Close()
	packageCsv := csv.NewReader(csvFileIO)
	csvHeader, err := packageCsv.Read()
	if err != nil {
		log.Fatalf("Failed to read CSV header: %s", err)
	}
	packageIdx := -1
	urlColIdx := -1
	for i, col := range csvHeader {
		if col == "SourceURL" {
			urlColIdx = i
		} else if col == "Package" {
			packageIdx = i
		}
	}
	if urlColIdx == -1 {
		log.Fatal("CSV file does not have a SourceURL column")
	}
	if packageIdx == -1 {
		log.Fatal("CSV file does not have a Package column")
	}
	names := make([]string, 0, 2<<8)
	urls := make([]string, 0, 2<<8)
	for {
		row, err := packageCsv.Read()
		if err != nil && err != io.EOF {
			log.Fatalf("Failed to read CSV row: %s", err)
		} else if err == io.EOF {
			break
		}
		names = append(names, row[packageIdx])
		urls = append(urls, row[urlColIdx])
	}
	log.Printf("Extracting info from %d packages with %d parallel processes", len(names), *flagNumProcs)
	for i, err := range extractPackages(urls, *flagOutput, *flagNumProcs) {
		if err != "" {
			log.Printf("Failed to extract package %s: %s", names[i], err)
		}
	}
}
