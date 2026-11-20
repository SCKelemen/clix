package clix_test

import (
	"bytes"
	"fmt"
	"go/scanner"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

// The documentation tests check every Markdown file in the repository
// (README.md, docs/*.md, ext/README.md and the other READMEs):
//
//   - TestDocsCompile compiles every ```go block against the module in this
//     working tree, so an example that no longer matches the API fails.
//   - TestDocsLinks checks that relative links point at files that exist
//     and that their #anchors name a heading of the page they point at.
//
// The first line of a Go block says what kind of block it is:
//
//   - "// Abridged" starts a reference listing, such as a struct shown
//     without its unexported fields. It is not compiled, and the comment
//     tells readers that it is not complete.
//   - "// path/to/file.go" followed by a package clause is one file of a
//     program, such as the Quick Start. These files are all written at their
//     paths in one module, example.com/demo, so they can import each other
//     ("example.com/demo/internal/greet").
//   - A package clause makes a block a complete file of its own.
//   - Anything else is a fragment. Its imports are kept; func and type
//     declarations, and var and const declarations outside any braces, are
//     compiled at the top level of the file, and the other lines are the
//     body of a func returning error. Fragments may use the variables in
//     fragmentVars and the packages in fragmentImports without declaring
//     them.
//
// Blocks may import the standard library and this module only. Compiler
// errors are reported at the Markdown file and line of the code.

// fragmentImports are imported by every fragment that does not import them
// itself, each with an expression that uses it.
var fragmentImports = map[string]string{
	"context":                        "context.Background",
	"errors":                         "errors.New",
	"fmt":                            "fmt.Sprint",
	"io":                             "io.Discard",
	"os":                             "os.Exit",
	"strings":                        "strings.Contains",
	"time":                           "time.Second",
	modulePath:                       "clix.NewApp",
	modulePath + "/ext/alias":        "alias.Extension{}",
	modulePath + "/ext/autocomplete": "autocomplete.Extension{}",
	modulePath + "/ext/config":       "config.Extension{}",
	modulePath + "/ext/format":       "format.Extension{}",
	modulePath + "/ext/help":         "help.Extension{}",
	modulePath + "/ext/plugin":       "plugin.Extension{}",
	modulePath + "/ext/prompt":       "prompt.Extension{}",
	modulePath + "/ext/survey":       "survey.End",
	modulePath + "/ext/validation":   "validation.Email",
	modulePath + "/ext/version":      "version.Extension{}",
}

// fragmentVars are the variables a fragment may use without declaring them.
const fragmentVars = `var (
	app       *clix.App
	cmd       *clix.Command
	ctx       *clix.Context
	prompter  clix.Prompter
	theme     clix.PromptTheme
	questions []survey.Question
)
`

const (
	modulePath = "github.com/SCKelemen/clix/v2"
	// docModule is the module the blocks are compiled in.
	docModule = "example.com/demo"
	// abridged starts a Go block that is not compiled.
	abridged = "// Abridged"
)

// docBlock is a fenced code block in a Markdown file.
type docBlock struct {
	doc   string   // the Markdown file, relative to the repository root
	line  int      // the line of the first line of code, 1-based
	lang  string   // the fence's info string, such as "go"
	lines []string // the code, without the fence's indentation
}

func (b docBlock) String() string { return fmt.Sprintf("%s:%d", b.doc, b.line) }

var fencePattern = regexp.MustCompile("^([ \t]*)(```+|~~~+)[ \t]*([^ \t`]*)")

// markdownFiles returns the Markdown files of the repository, relative to
// root, leaving out hidden directories and testdata.
func markdownFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path != root && (strings.HasPrefix(name, ".") || name == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(name, ".md") {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	if len(files) == 0 {
		t.Fatal("found no Markdown files")
	}
	return files
}

// parseMarkdown returns the fenced code blocks of a Markdown file, and its
// other lines with the code blocks blanked out.
func parseMarkdown(doc string, src []byte) (blocks []docBlock, prose []string, err error) {
	lines := strings.Split(strings.ReplaceAll(string(src), "\r\n", "\n"), "\n")
	prose = make([]string, len(lines))
	for i := 0; i < len(lines); i++ {
		m := fencePattern.FindStringSubmatch(lines[i])
		if m == nil {
			prose[i] = lines[i]
			continue
		}
		indent, fence, lang := len(m[1]), m[2], m[3]
		start := i + 1
		end := start
		for end < len(lines) && !isClosingFence(lines[end], fence) {
			end++
		}
		if end == len(lines) {
			return nil, nil, fmt.Errorf("%s:%d: code block is not closed", doc, i+1)
		}
		body := make([]string, 0, end-start)
		for _, l := range lines[start:end] {
			// Remove up to the fence's indentation, as CommonMark does.
			n := 0
			for n < indent && n < len(l) && (l[n] == ' ' || l[n] == '\t') {
				n++
			}
			body = append(body, l[n:])
		}
		blocks = append(blocks, docBlock{doc: doc, line: start + 1, lang: lang, lines: body})
		i = end
	}
	return blocks, prose, nil
}

func isClosingFence(line, fence string) bool {
	s := strings.TrimSpace(line)
	return strings.HasPrefix(s, fence) && strings.Trim(s, fence[:1]) == ""
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("the documentation tests run in the repository root: %v", err)
	}
	return root
}

// TestDocsCompile compiles the Go code blocks of the Markdown files.
func TestDocsCompile(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles the documentation with the go command; skipped with -short")
	}
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("the go command is needed to compile the documentation: %v", err)
	}
	t.Parallel()

	root := repoRoot(t)
	work := t.TempDir()
	if err := writeDocModule(root, work); err != nil {
		t.Fatal(err)
	}

	var compiled, skipped int
	where := map[string]string{}   // generated file or package dir -> the block
	written := map[string]string{} // path of a file block -> the block
	for _, doc := range markdownFiles(t, root) {
		src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(doc)))
		if err != nil {
			t.Fatal(err)
		}
		blocks, _, err := parseMarkdown(doc, src)
		if err != nil {
			t.Error(err)
			continue
		}
		for _, b := range blocks {
			if b.lang != "go" {
				continue
			}
			if len(b.lines) > 0 && strings.HasPrefix(strings.TrimSpace(b.lines[0]), abridged) {
				skipped++
				continue
			}
			compiled++
			rel, content, err := renderBlock(root, b)
			if err != nil {
				t.Errorf("%s: %v", b, err)
				continue
			}
			if prev, ok := written[rel]; ok {
				t.Errorf("%s: %s is also the file of the block at %s", b, rel, prev)
				continue
			}
			written[rel] = b.String()
			where[slashDir(rel)] = b.String()
			dst := filepath.Join(work, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(dst, content, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Logf("compiled %d Go blocks; skipped %d abridged listings", compiled, skipped)
	if compiled == 0 {
		return
	}

	build := exec.Command(goTool, "build", "./...")
	build.Dir = work
	build.Env = append(os.Environ(),
		"GOFLAGS=-mod=mod", // record the requirements that clix's go.mod implies
		"GOWORK=off",
		"GOTOOLCHAIN=local",
		"GOPROXY=off", // everything comes from the module cache, as for this test
	)
	out, err := build.CombinedOutput()
	if err == nil {
		return
	}
	t.Errorf("Go code in the documentation does not compile (%v).\n"+
		"Fix the code, or start a reference listing with %q.\n\n%s",
		err, abridged, rewriteBuildOutput(string(out), root, work, where))
}

// writeDocModule writes the go.mod and go.sum of the module the blocks are
// compiled in: example.com/demo, with clix replaced by the working tree.
func writeDocModule(root, work string) error {
	goMod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return err
	}
	goVersion := "1.27.0"
	for _, l := range strings.Split(string(goMod), "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(l), "go "); ok {
			goVersion = strings.TrimSpace(v)
		}
	}
	mod := fmt.Sprintf("module %s\n\ngo %s\n\nrequire %s v2.0.0\n\nreplace %s => %s\n",
		docModule, goVersion, modulePath, modulePath, strconv.Quote(root))
	if err := os.WriteFile(filepath.Join(work, "go.mod"), []byte(mod), 0o644); err != nil {
		return err
	}
	sum, err := os.ReadFile(filepath.Join(root, "go.sum"))
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(work, "go.sum"), sum, 0o644)
}

var fileBlockPattern = regexp.MustCompile(`^//\s*(\S+\.go)\s*$`)

// renderBlock returns the path, in the module, and the content of the file
// that a Go block compiles as.
func renderBlock(root string, b docBlock) (string, []byte, error) {
	docPath := filepath.Join(root, filepath.FromSlash(b.doc))
	first := ""
	if len(b.lines) > 0 {
		first = strings.TrimSpace(b.lines[0])
	}
	if m := fileBlockPattern.FindStringSubmatch(first); m != nil && hasPackageClause(b.lines[1:]) {
		rel := m[1]
		if !filepath.IsLocal(filepath.FromSlash(rel)) || strings.HasPrefix(rel, "blocks/") {
			return "", nil, fmt.Errorf("%s is not a path a block can be written to", rel)
		}
		if err := checkImports(b.lines); err != nil {
			return "", nil, err
		}
		return rel, []byte(lineDirective(docPath, b.line) + strings.Join(b.lines, "\n") + "\n"), nil
	}
	dir := "blocks/" + blockID(b)
	if hasPackageClause(b.lines) {
		if err := checkImports(b.lines); err != nil {
			return "", nil, err
		}
		return dir + "/block.go", []byte(lineDirective(docPath, b.line) + strings.Join(b.lines, "\n") + "\n"), nil
	}
	content, err := renderFragment(docPath, dir+"/block.go", b)
	return dir + "/block.go", content, err
}

// hasPackageClause reports whether the first line of code, after comments
// and blank lines, is a package clause.
func hasPackageClause(lines []string) bool {
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "//") {
			continue
		}
		return strings.HasPrefix(l, "package ")
	}
	return false
}

func blockID(b docBlock) string {
	id := strings.Map(func(r rune) rune {
		if r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			return r
		}
		return '_'
	}, b.doc)
	return fmt.Sprintf("%s_L%d", id, b.line)
}

// slashDir returns the directory of a slash-separated path.
func slashDir(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[:i]
	}
	return "."
}

func lineDirective(file string, line int) string {
	return fmt.Sprintf("//line %s:%d\n", file, line)
}

// checkImports reports an import that the doc module cannot resolve.
func checkImports(lines []string) error {
	src := []byte(strings.Join(lines, "\n"))
	imports, _, err := splitFragment(src)
	if err != nil {
		return nil // the compiler reports it
	}
	for _, imp := range imports {
		if err := checkImportPath(imp.path); err != nil {
			return err
		}
	}
	return nil
}

func checkImportPath(path string) error {
	first, _, _ := strings.Cut(path, "/")
	if !strings.Contains(first, ".") { // the standard library
		return nil
	}
	if path == modulePath || strings.HasPrefix(path, modulePath+"/") ||
		path == docModule || strings.HasPrefix(path, docModule+"/") {
		return nil
	}
	return fmt.Errorf("imports %s: blocks may import only the standard library and %s; start a listing that needs more with %q", path, modulePath, abridged)
}

// importSpec is an import of a fragment.
type importSpec struct {
	name, path string
	line       int // the line of the spec in the block, 1-based
}

// fragmentPart is a top-level declaration or a statement of a fragment.
type fragmentPart struct {
	decl       bool
	start, end int // lines in the block, 1-based
}

// splitFragment returns the imports of a fragment (or of a file, after its
// package clause), and its declarations and statements.
func splitFragment(src []byte) ([]importSpec, []fragmentPart, error) {
	fset := token.NewFileSet()
	file := fset.AddFile("block.go", -1, len(src))
	var errs scanner.ErrorList
	var s scanner.Scanner
	s.Init(file, src, func(pos token.Position, msg string) { errs.Add(pos, msg) }, 0)

	type tok struct {
		tok  token.Token
		lit  string
		line int
	}
	var toks []tok
	for {
		pos, t, lit := s.Scan()
		if t == token.EOF {
			break
		}
		toks = append(toks, tok{t, lit, fset.Position(pos).Line})
	}
	if errs.Len() > 0 {
		return nil, nil, errs.Err()
	}

	var imports []importSpec
	i := 0
	if len(toks) >= 2 && toks[0].tok == token.PACKAGE {
		i = 2 // package name
		for i < len(toks) && toks[i].tok == token.SEMICOLON {
			i++
		}
	}
	for i < len(toks) && toks[i].tok == token.IMPORT {
		i++
		spec := func() {
			var name string
			if i < len(toks) && (toks[i].tok == token.IDENT || toks[i].tok == token.PERIOD) {
				name = toks[i].lit
				if toks[i].tok == token.PERIOD {
					name = "."
				}
				i++
			}
			if i < len(toks) && toks[i].tok == token.STRING {
				path, err := strconv.Unquote(toks[i].lit)
				if err == nil {
					imports = append(imports, importSpec{name: name, path: path, line: toks[i].line})
				}
				i++
			}
		}
		if i < len(toks) && toks[i].tok == token.LPAREN {
			i++
			for i < len(toks) && toks[i].tok != token.RPAREN {
				if toks[i].tok == token.SEMICOLON {
					i++
					continue
				}
				before := i
				spec()
				if i == before {
					i++ // not an import spec; the compiler reports it
				}
			}
			i++ // )
		} else {
			spec()
		}
		for i < len(toks) && toks[i].tok == token.SEMICOLON {
			i++
		}
	}

	var parts []fragmentPart
	for i < len(toks) {
		start := i
		decl := false
		switch toks[i].tok {
		case token.TYPE, token.VAR, token.CONST:
			decl = true
		case token.FUNC:
			var rest []token.Token
			for _, t := range toks[i+1:] {
				rest = append(rest, t.tok)
			}
			decl = isFuncDecl(rest)
		case token.IMPORT:
			return nil, nil, fmt.Errorf("line %d: imports must come first", toks[i].line)
		}
		depth := 0
		for ; i < len(toks); i++ {
			switch toks[i].tok {
			case token.LPAREN, token.LBRACK, token.LBRACE:
				depth++
			case token.RPAREN, token.RBRACK, token.RBRACE:
				depth--
			}
			if depth == 0 && toks[i].tok == token.SEMICOLON {
				break
			}
		}
		end := len(toks) - 1
		if i < len(toks) {
			end = i
		}
		parts = append(parts, fragmentPart{decl: decl, start: toks[start].line, end: toks[end].line})
		i++
	}
	return imports, parts, nil
}

// isFuncDecl reports whether the tokens after "func" declare a function or
// a method, rather than start a function literal.
func isFuncDecl(rest []token.Token) bool {
	if len(rest) == 0 {
		return false
	}
	if rest[0] == token.IDENT {
		return true
	}
	if rest[0] != token.LPAREN {
		return false
	}
	// A method: (receiver) Name(...). A literal: (params) {, or a result.
	depth := 0
	for k, t := range rest {
		switch t {
		case token.LPAREN:
			depth++
		case token.RPAREN:
			depth--
			if depth == 0 {
				return k+2 < len(rest) && rest[k+1] == token.IDENT && rest[k+2] == token.LPAREN
			}
		}
	}
	return false
}

// fileWriter builds a generated file, keeping count of its lines so that
// //line directives can point back at the generated file itself.
type fileWriter struct {
	buf  bytes.Buffer
	line int // lines written
}

func (w *fileWriter) printf(format string, args ...any) {
	s := fmt.Sprintf(format, args...)
	w.buf.WriteString(s)
	w.line += strings.Count(s, "\n")
}

// renderFragment returns the file a fragment compiles as, with //line
// directives that map its code to the Markdown file.
func renderFragment(docPath, genPath string, b docBlock) ([]byte, error) {
	src := []byte(strings.Join(b.lines, "\n"))
	imports, parts, err := splitFragment(src)
	if err != nil {
		return nil, err
	}
	for _, imp := range imports {
		if err := checkImportPath(imp.path); err != nil {
			return nil, err
		}
	}
	own := map[string]bool{}
	for _, imp := range imports {
		own[imp.path] = true
	}
	docLine := func(blockLine int) int { return b.line + blockLine - 1 }

	var w fileWriter
	w.printf("package doc\n\n")
	if len(parts) == 0 {
		// Only imports: check that they resolve.
		for _, imp := range imports {
			w.printf("%simport _ %s\n", lineDirective(docPath, docLine(imp.line)), strconv.Quote(imp.path))
		}
		return w.buf.Bytes(), nil
	}

	var defaults []string
	for path := range fragmentImports {
		if !own[path] {
			defaults = append(defaults, path)
		}
	}
	sort.Strings(defaults)
	w.printf("import (\n")
	for _, path := range defaults {
		w.printf("\t%s\n", strconv.Quote(path))
	}
	for _, imp := range imports {
		w.printf("%s\t%s %s\n", lineDirective(docPath, docLine(imp.line)), imp.name, strconv.Quote(imp.path))
	}
	w.printf(")\n")
	w.printf("%s\n", lineDirective(genPath, w.line+2))
	for _, path := range defaults {
		w.printf("var _ = %s\n", fragmentImports[path])
	}
	w.printf("\n%s\n", fragmentVars)

	// Each line of the fragment goes where the part it belongs to goes.
	// Comments and blank lines between parts go with the part after them.
	kind := make([]int, len(b.lines)+1) // 1 decl, 2 statement, by block line
	for _, p := range parts {
		k := 2
		if p.decl {
			k = 1
		}
		for l := p.start; l <= p.end && l < len(kind); l++ {
			if kind[l] == 0 {
				kind[l] = k
			}
		}
	}
	next := 2
	for l := len(b.lines); l >= 1; l-- {
		if kind[l] == 0 {
			kind[l] = next
		}
		next = kind[l]
	}
	firstImportFree := 1
	if len(imports) > 0 {
		// Lines up to the end of the import declarations are not code.
		firstImportFree = parts[0].start
		for l := 1; l < firstImportFree; l++ {
			kind[l] = 0
		}
	}

	emit := func(k int) {
		last := -1
		for l := firstImportFree; l <= len(b.lines); l++ {
			if kind[l] != k {
				continue
			}
			if l != last+1 {
				w.printf("%s", lineDirective(docPath, docLine(l)))
			}
			w.printf("%s\n", b.lines[l-1])
			last = l
		}
	}
	emit(1)
	w.printf("%s", lineDirective(genPath, w.line+2))
	w.printf("func fragment() error {\n")
	emit(2)
	w.printf("%s", lineDirective(genPath, w.line+2))
	w.printf("\treturn nil\n}\n")
	return w.buf.Bytes(), nil
}

var buildPosPattern = regexp.MustCompile(`^(\S+?\.(?:go|md)):(\d+)(?::(\d+))?:`)

// rewriteBuildOutput makes the paths in the go command's output relative
// to the repository, and names the block of each failing package.
func rewriteBuildOutput(out, root, work string, where map[string]string) string {
	var b strings.Builder
	for _, l := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if pkg, ok := strings.CutPrefix(l, "# "+docModule+"/"); ok {
			if block, ok := where[pkg]; ok {
				fmt.Fprintf(&b, "# the block at %s\n", block)
				continue
			}
		}
		if m := buildPosPattern.FindStringSubmatchIndex(l); m != nil {
			path := l[m[2]:m[3]]
			if !filepath.IsAbs(path) {
				path = filepath.Join(work, path)
			}
			if rel, err := filepath.Rel(root, path); err == nil && filepath.IsLocal(rel) {
				l = filepath.ToSlash(rel) + l[m[3]:]
			} else if rel, err := filepath.Rel(work, path); err == nil && filepath.IsLocal(rel) {
				block := where[slashDir(filepath.ToSlash(rel))]
				l = fmt.Sprintf("(code generated for the block at %s) %s", block, filepath.ToSlash(rel)+l[m[3]:])
			}
		}
		b.WriteString(l)
		b.WriteByte('\n')
	}
	return b.String()
}

var (
	linkPattern       = regexp.MustCompile(`!?\[[^\]]*\]\(([^()\s]+)(?:\s+"[^"]*")?\)`)
	headingPattern    = regexp.MustCompile(`^ {0,3}(#{1,6})[ \t]+(.*?)[ \t#]*$`)
	inlineCodePattern = regexp.MustCompile("`+[^`]*`+")
	headingLinkText   = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
)

// TestDocsLinks checks the relative links of the Markdown files.
func TestDocsLinks(t *testing.T) {
	root := repoRoot(t)
	anchors := map[string]map[string]bool{} // Markdown file -> its anchors
	prose := map[string][]string{}
	docs := markdownFiles(t, root)
	for _, doc := range docs {
		src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(doc)))
		if err != nil {
			t.Fatal(err)
		}
		_, lines, err := parseMarkdown(doc, src)
		if err != nil {
			t.Error(err)
			continue
		}
		prose[doc] = lines
		anchors[doc] = headingAnchors(lines)
	}
	for _, doc := range docs {
		for i, l := range prose[doc] {
			l = inlineCodePattern.ReplaceAllString(l, "")
			for _, m := range linkPattern.FindAllStringSubmatch(l, -1) {
				target := m[1]
				if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
					continue
				}
				path, anchor, _ := strings.Cut(target, "#")
				resolved := doc
				if path != "" {
					resolved = filepath.ToSlash(filepath.Clean(filepath.Join(filepath.Dir(doc), path)))
					if !filepath.IsLocal(resolved) {
						t.Errorf("%s:%d: link %s leaves the repository", doc, i+1, target)
						continue
					}
					if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(resolved))); err != nil {
						t.Errorf("%s:%d: link %s: %s does not exist", doc, i+1, target, resolved)
						continue
					}
				}
				if anchor == "" {
					continue
				}
				names, ok := anchors[resolved]
				if !ok {
					continue // not a Markdown file
				}
				if !names[anchor] {
					t.Errorf("%s:%d: link %s: %s has no heading #%s", doc, i+1, target, resolved, anchor)
				}
			}
		}
	}
}

// headingAnchors returns the anchors GitHub gives the headings of a
// Markdown file.
func headingAnchors(lines []string) map[string]bool {
	names := map[string]bool{}
	seen := map[string]int{}
	for _, l := range lines {
		m := headingPattern.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		slug := headingSlug(m[2])
		if n := seen[slug]; n > 0 {
			names[fmt.Sprintf("%s-%d", slug, n)] = true
		} else {
			names[slug] = true
		}
		seen[slug]++
	}
	return names
}

// headingSlug returns the anchor of a heading as GitHub makes it: the text
// in lowercase, without punctuation other than hyphens and underscores,
// with spaces replaced by hyphens.
func headingSlug(heading string) string {
	text := headingLinkText.ReplaceAllString(heading, "$1")
	text = strings.NewReplacer("`", "", "*", "").Replace(text)
	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		switch {
		case r == ' ':
			b.WriteByte('-')
		case r == '-' || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		}
	}
	return b.String()
}
