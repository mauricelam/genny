package parse

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/tools/imports"
)

type isExported bool

var header = []byte(`

// This file was automatically generated by genny.
// Any changes will be lost if this file is regenerated.
// see https://github.com/mauricelam/genny

`)

var importBlock = `import (
	%s
)`

var (
	packageKeyword = []byte("package")
	importKeyword  = []byte("import")
	openBrace      = []byte("(")
	closeBrace     = []byte(")")
	space          = " "
	genericPackage = "generic"
	genericType    = "generic.Type"
	genericNumber  = "generic.Number"
	linefeed       = "\r\n"
)
var unwantedLinePrefixes = [][]byte{
	[]byte("//go:generate genny "),
	[]byte("//go:generate $GOPATH/bin/genny "),
}

func generateSpecific(filename string, in io.ReadSeeker, typeSet map[string]string) ([]byte, error) {

	// ensure we are at the beginning of the file
	in.Seek(0, os.SEEK_SET)

	// parse the source file
	fs := token.NewFileSet()
	file, err := parser.ParseFile(fs, filename, in, 0)
	if err != nil {
		return nil, &errSource{Err: err}
	}

	// make sure every generic.Type is represented in the types
	// argument.
	for _, decl := range file.Decls {
		switch it := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range it.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				switch tt := ts.Type.(type) {
				case *ast.SelectorExpr:
					if name, ok := tt.X.(*ast.Ident); ok {
						if name.Name == genericPackage {
							if _, ok := typeSet[ts.Name.Name]; !ok {
								return nil, &errMissingSpecificType{GenericType: ts.Name.Name}
							}
						}
					}
				}
			}
		}
	}

	// go back to the start of the file
	in.Seek(0, os.SEEK_SET)

	var buf bytes.Buffer

	comment := ""
	scanner := bufio.NewScanner(in)
	reInterfaceBegin := regexp.MustCompile(`^\s*type\s+\w+\s+interface\s*\{`)
	reInterfaceEnd := regexp.MustCompile(`^\s*\}`)
	var interfaceLines []string
	interfaceContainsType := false
	for scanner.Scan() {

		l := scanner.Text()

		if reInterfaceBegin.MatchString(l) {
			interfaceLines = []string{l}
		}

		if len(interfaceLines) > 0 && reInterfaceEnd.MatchString(l) {
			if !interfaceContainsType {
				for _, li := range append(interfaceLines, l) {
					buf.WriteString(li)
				}
			}
			interfaceLines, interfaceContainsType = nil, false
			continue
		}

		// does this line contain generic.Type?
		if strings.Contains(l, genericType) || strings.Contains(l, genericNumber) {
			comment = ""
			if len(interfaceLines) > 0 {
				interfaceContainsType = true
			}
			continue
		}

		for t, specificType := range typeSet {

			// does the line contain our type
			if strings.Contains(l, t) {

				var newLine string
				// check each word
				for _, word := range strings.Fields(l) {

					i := 0
					for {
						i = strings.Index(word[i:], t) // find out where

						if i > -1 {

							// if this isn't an exact match
							if i > 0 && isAlphaNumeric(rune(word[i-1])) || i < len(word)-len(t) && isAlphaNumeric(rune(word[i+len(t)])) {
								// replace the word with a capitolized version
								word = strings.Replace(word, t, wordify(specificType, unicode.IsUpper(rune(strings.TrimLeft(word, "*&")[0]))), 1)
							} else {
								// replace the word as is
								word = strings.Replace(word, t, typify(specificType), 1)
							}

						} else {
							newLine = newLine + word + space
							break
						}

					}
				}
				l = newLine
			}
		}

		if comment != "" {
			buf.WriteString(line(comment))
			comment = ""
		}

		// is this line a comment?
		// TODO: should we handle /* */ comments?
		if strings.HasPrefix(l, "//") {
			// record this line to print later
			comment = l
			continue
		}

		// write the line
		if len(interfaceLines) > 0 {
			interfaceLines = append(interfaceLines, l)
		} else {
			buf.WriteString(line(l))
		}
	}

	// write it out
	return buf.Bytes(), nil
}

// Generics parses the source file and generates the bytes replacing the
// generic types for the keys map with the specific types (its value).
func Generics(filename, pkgName string, in io.ReadSeeker, typeSets []map[string]string, importPaths []string, stripTag string) ([]byte, error) {
	localUnwantedLinePrefixes := [][]byte{}
	for _, ulp := range unwantedLinePrefixes {
		localUnwantedLinePrefixes = append(localUnwantedLinePrefixes, ulp)
	}

	if stripTag != "" {
		localUnwantedLinePrefixes = append(localUnwantedLinePrefixes, []byte(fmt.Sprintf("// +build %s", stripTag)))
	}

	packageLine := ""
	var collectedImports stringArraySet
	totalOutput := []byte{}

	for _, typeSet := range typeSets {

		// generate the specifics
		parsed, err := generateSpecific(filename, in, typeSet)
		if err != nil {
			return nil, err
		}

		totalOutput = append(totalOutput, parsed...)
	}

	// clean up the code line by line
	packageFound := false
	insideImportBlock := false
	var outputLines []string
	scanner := bufio.NewScanner(bytes.NewReader(totalOutput))
	for scanner.Scan() {

		// end of imports block?
		if insideImportBlock {
			if bytes.HasSuffix(scanner.Bytes(), closeBrace) {
				insideImportBlock = false
			} else {
				collectedImports = collectedImports.append(line(scanner.Text()))
			}

			continue
		}

		if bytes.HasPrefix(scanner.Bytes(), packageKeyword) {
			if packageFound {
				continue
			} else {
				packageFound = true
				packageLine = line(scanner.Text())
				continue
			}
		} else if bytes.HasPrefix(scanner.Bytes(), importKeyword) {
			if bytes.HasSuffix(scanner.Bytes(), openBrace) {
				insideImportBlock = true
			} else {
				importLine := strings.TrimSpace(line(scanner.Text()))
				importLine = strings.TrimSpace(importLine[6:])
				collectedImports = collectedImports.append(importLine)
			}

			continue
		}

		// check all unwantedLinePrefixes - and skip them
		skipline := false
		for _, prefix := range localUnwantedLinePrefixes {
			if bytes.HasPrefix(scanner.Bytes(), prefix) {
				skipline = true
				continue
			}
		}

		if skipline {
			continue
		}

		outputLines = append(outputLines, line(scanner.Text()))
	}

	cleanOutputLines := []string{
		string(header),
		packageLine,
		fmt.Sprintln("import ("),
	}
	for _, importLine := range collectedImports {
		cleanOutputLines = append(cleanOutputLines, fmt.Sprintln(importLine))
	}
	cleanOutputLines = append(cleanOutputLines, fmt.Sprintln(")"))

	cleanOutputLines = append(cleanOutputLines, outputLines...)

	cleanOutput := strings.Join(cleanOutputLines, "")

	output := []byte(cleanOutput)
	var err error

	// change package name
	if pkgName != "" {
		output = changePackage(bytes.NewReader([]byte(output)), pkgName)
	}
	if len(importPaths) > 0 {
		output = addImports(bytes.NewReader(output), importPaths)
	}
	// fix the imports
	output, err = imports.Process(filename, output, nil)
	if err != nil {
		return nil, &errImports{Err: err}
	}

	return output, nil
}

func line(s string) string {
	return fmt.Sprintln(strings.TrimRight(s, linefeed))
}

// isAlphaNumeric gets whether the rune is alphanumeric or _.
func isAlphaNumeric(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// wordify turns a type into a nice word for function and type
// names etc.
// If s matches format `<Title>:<Type>` then <Title> is returned
func wordify(s string, exported bool) string {
	if sepIdx := strings.Index(s, ":"); sepIdx >= 0 {
		return s[:sepIdx]
	}
	s = strings.TrimRight(s, "{}")
	s = strings.TrimLeft(s, "*&")
	s = strings.Replace(s, ".", "", -1)
	if !exported {
		return s
	}
	return strings.ToUpper(string(s[0])) + s[1:]
}

// typify gets type name from string.
// if string contains ":" then right part is returned otherwise string itself is returned
func typify(s string) string {
	if sepIdx := strings.Index(s, ":"); sepIdx >= 0 {
		return s[sepIdx+1:]
	}
	return s
}

func changePackage(r io.Reader, pkgName string) []byte {
	var out bytes.Buffer
	sc := bufio.NewScanner(r)
	done := false

	for sc.Scan() {
		s := sc.Text()

		if !done && strings.HasPrefix(s, "package") {
			parts := strings.Split(s, " ")
			parts[1] = pkgName
			s = strings.Join(parts, " ")
			done = true
		}

		fmt.Fprintln(&out, s)
	}
	return out.Bytes()
}

func addImports(r io.Reader, importPaths []string) []byte {
	var out bytes.Buffer
	sc := bufio.NewScanner(r)
	done := false

	for sc.Scan() {
		s := sc.Text()

		if !done && strings.HasPrefix(s, "package") {
			fmt.Fprintln(&out, s)
			for _, imp := range importPaths {
				fmt.Fprintf(&out, "import \"%s\"\n", imp)
			}
			done = true
			continue
		}

		fmt.Fprintln(&out, s)
	}
	return out.Bytes()
}
