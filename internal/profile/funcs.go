package profile

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"

	"golang.org/x/tools/cover"
)

type AnnotatedFunc struct {
	Name      string
	StartLine int
	Count     int
}

// WalkFuncs parses the source file at diskPath and returns all function
// declarations annotated with call counts from the profile blocks.
func WalkFuncs(p *cover.Profile, diskPath string) ([]AnnotatedFunc, error) {
	src, err := os.ReadFile(diskPath)
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, diskPath, src, 0)
	if err != nil {
		return nil, err
	}

	var funcs []AnnotatedFunc
	ast.Inspect(f, func(n ast.Node) bool {
		fd, ok := n.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			return true
		}
		bodyStart := fset.Position(fd.Body.Lbrace).Line
		bodyEnd := fset.Position(fd.Body.Rbrace).Line
		funcs = append(funcs, AnnotatedFunc{
			Name:      funcName(fd),
			StartLine: fset.Position(fd.Pos()).Line,
			Count:     callCount(bodyStart, bodyEnd, p.Blocks),
		})
		return false
	})

	return funcs, nil
}

// funcName formats a function name, including receiver type for methods.
func funcName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return fd.Name.Name
	}
	recv := fd.Recv.List[0].Type
	switch t := recv.(type) {
	case *ast.StarExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return fmt.Sprintf("(*%s).%s", ident.Name, fd.Name.Name)
		}
	case *ast.Ident:
		return fmt.Sprintf("%s.%s", t.Name, fd.Name.Name)
	}
	return fd.Name.Name
}

// callCount returns the hit count of the first block inside the function body.
func callCount(bodyStart, bodyEnd int, blocks []cover.ProfileBlock) int {
	for _, b := range blocks {
		if b.StartLine >= bodyStart && b.StartLine <= bodyEnd {
			return b.Count
		}
	}
	return 0
}
