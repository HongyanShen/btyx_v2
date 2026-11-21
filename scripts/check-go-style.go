//go:build ignore

// check-go-style checks the readability rules that gofmt does not enforce.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	violations := 0

	for _, root := range []string{"backend/internal", "backend/ent/schema", "backend/cmd"} {
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkError error) error {
			if walkError != nil {
				return walkError
			}

			if entry.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}

			count, err := checkFile(path)
			violations += count

			return err
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	if violations > 0 {
		fmt.Fprintf(os.Stderr, "发现 %d 处可读性问题\n", violations)
		os.Exit(1)
	}

	fmt.Println("Go 可读性检查通过：函数展开、函数间空行、控制块后逻辑分段。")
}

func checkFile(path string) (int, error) {
	positions := token.NewFileSet()
	file, err := parser.ParseFile(positions, path, nil, parser.ParseComments)
	if err != nil {
		return 0, err
	}

	violations := 0
	report := func(position token.Pos, message string) {
		fmt.Printf("%s: %s\n", positions.Position(position), message)
		violations++
	}

	for index, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Body == nil {
			continue
		}

		if positions.Position(function.Body.Lbrace).Line == positions.Position(function.Body.Rbrace).Line {
			report(function.Pos(), "函数体应展开，不能压在一行")
		}

		if index+1 < len(file.Decls) {
			next := file.Decls[index+1]
			if positions.Position(next.Pos()).Line <= positions.Position(function.End()).Line+1 {
				report(next.Pos(), "函数之间应有空行")
			}
		}
	}

	ast.Inspect(file, func(node ast.Node) bool {
		if literal, ok := node.(*ast.CompositeLit); ok && len(literal.Elts) > 1 {
			for index, element := range literal.Elts {
				if _, keyed := element.(*ast.KeyValueExpr); !keyed {
					break
				}

				if index > 0 && positions.Position(element.Pos()).Line == positions.Position(literal.Elts[index-1].End()).Line {
					report(element.Pos(), "结构体或命名字典字段应逐项展开")
				}
			}
		}

		block, ok := node.(*ast.BlockStmt)
		if !ok {
			return true
		}

		for index, statement := range block.List {
			if index+1 == len(block.List) {
				continue
			}

			switch statement.(type) {
			case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt:
				next := block.List[index+1]
				if positions.Position(next.Pos()).Line <= positions.Position(statement.End()).Line+1 {
					report(next.Pos(), "控制块结束后应与后续逻辑空行分段")
				}
			}
		}

		return true
	})

	return violations, nil
}
