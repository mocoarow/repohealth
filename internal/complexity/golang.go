package complexity

import (
	"cmp"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"slices"

	"github.com/fzipp/gocyclo"
	"github.com/uudashr/gocognit"

	"github.com/mocoarow/repohealth/internal/config"
	"github.com/mocoarow/repohealth/internal/gosrc"
)

// CycloNotMeasured is the Cyclo of a function in a language without a cyclomatic measure.
const CycloNotMeasured = 0

// Function is the complexity of one function.
type Function struct {
	File   string
	Line   int
	Name   string
	Cyclo  int
	Cognit int
}

// Go measures the hand-written functions in the files gosrc.Counted accepts.
func Go(fsys fs.FS, cfg config.Config, files []string) ([]Function, error) {
	funcs := []Function{}

	for _, file := range files {
		if !gosrc.Counted(cfg, file) {
			continue
		}

		found, err := measureFile(fsys, file)
		if err != nil {
			return nil, err
		}

		funcs = append(funcs, found...)
	}

	slices.SortFunc(funcs, func(a, b Function) int {
		return cmp.Or(cmp.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line))
	})

	return funcs, nil
}

func measureFile(fsys fs.FS, file string) ([]Function, error) {
	src, err := fs.ReadFile(fsys, file)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", file, err)
	}

	fset := token.NewFileSet()

	f, err := parser.ParseFile(fset, file, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", file, err)
	}

	funcs := []Function{}
	if ast.IsGenerated(f) {
		return funcs, nil
	}

	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		funcs = append(funcs, Function{
			File:   file,
			Line:   fset.Position(fn.Pos()).Line,
			Name:   funcName(fn),
			Cyclo:  gocyclo.Complexity(fn),
			Cognit: gocognit.Complexity(fn),
		})
	}

	return funcs, nil
}

func funcName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}

	return "(" + recvName(fn.Recv.List[0].Type) + ")." + fn.Name.Name
}

func recvName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return "*" + recvName(t.X)
	case *ast.IndexExpr:
		return recvName(t.X)
	case *ast.IndexListExpr:
		return recvName(t.X)
	default:
		return types.ExprString(expr)
	}
}
