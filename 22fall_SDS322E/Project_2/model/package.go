package model

type P struct {
	URL         string
	Description struct {
		Package     string
		Title       string
		Version     string
		License     string
		Description string
		Imports     []string
		Depends     []string
		Suggests    []string
		BiocViews   []string
	}
	Namespace struct {
		Calls   []NamespaceCall `json:"-"`
		Exports []string
		Imports []string
	}
	// Number of files per extension
	FileExtensions map[string]uint
	// number of R function calls
	// a function call is tokenized as:
	// [ SYMBOL_PACKAGE (package.name) NS_GET (::) ] SYMBOL_FUNCTION_CALL '(' ... ')'
	RFiles     []RFile
	Files      []string `json:"-"`
	FetchError string
	ParseError []ParseError
}

type ParseError struct {
	Stage   string
	File    string
	Message string
	Stack   string
}

type NamespaceCall struct {
	Name string
	Args []string
	Opts map[string]string
}

type RFile struct {
	Name    string
	NTokens int
	Stats   map[string]interface{}
}

func NewP() *P {
	return &P{
		FileExtensions: make(map[string]uint),
	}
}
