package main

import (
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// TestResult 测试结果
type TestResult struct {
	Package  string
	Passed   bool
	Duration time.Duration
	Output   string
}

// CodeReviewResult 代码审查结果
type CodeReviewResult struct {
	File     string
	Issues   []string
	Severity string // error, warning, info
}

func main() {
	fmt.Println("=== GPU Scheduler 自动化开发流程 ===")
	fmt.Println()

	// 1. 格式化代码
	fmt.Println("[1/5] 格式化代码...")
	formatCode()

	// 2. 运行测试
	fmt.Println("[2/5] 运行测试...")
	results := runTests()

	// 3. 检查测试结果
	fmt.Println("[3/5] 检查测试结果...")
	allPassed := true
	for _, r := range results {
		if !r.Passed {
			fmt.Printf("  ❌ %s: FAILED\n", r.Package)
			allPassed = false
		} else {
			fmt.Printf("  ✅ %s: PASSED (%.2fs)\n", r.Package, r.Duration.Seconds())
		}
	}

	if !allPassed {
		fmt.Println("\n❌ 测试失败，请修复后重试")
		printTestDetails(results)
		os.Exit(1)
	}

	// 4. 代码审查
	fmt.Println("[4/5] 代码审查...")
	issues := runCodeReview()

	if len(issues) > 0 {
		fmt.Printf("  ⚠️ 发现 %d 个问题:\n", len(issues))
		for _, issue := range issues {
			fmt.Printf("    - %s: %s\n", issue.File, issue.Issues[0])
		}
		fmt.Println()
	} else {
		fmt.Println("  ✅ 代码审查通过")
	}

	// 5. 总结
	fmt.Println("[5/5] 完成!")
	fmt.Println()
	fmt.Println("=== 所有检查通过，可以提交到GitHub ===")
}

func formatCode() {
	// 格式化所有Go文件
	filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".go") {
			data, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}
			formatted, err := format.Source(data)
			if err != nil {
				fmt.Printf("  ⚠️ 格式化失败 %s: %v\n", path, err)
				return nil
			}
			ioutil.WriteFile(path, formatted, info.Mode())
		}
		return nil
	})
	fmt.Println("  ✅ 代码格式化完成")
}

func runTests() []TestResult {
	var results []TestResult

	// 运行所有测试
	cmd := exec.Command("go", "test", "./...", "-v", "-json")
	output, err := cmd.CombinedOutput()

	// 解析输出
	lines := strings.Split(string(output), "\n")
	var currentPkg string
	var currentPassed bool

	for _, line := range lines {
		if strings.Contains(line, `"Package"`) {
			// 提取包名
			start := strings.Index(line, `"Package":"`) + 10
			end := strings.Index(line, `"Action"`)
			if start > 10 && end > start {
				currentPkg = line[start:end]
				currentPkg = strings.ReplaceAll(currentPkg, "\\", "")
			}
		}
		if strings.Contains(line, `"Action":"pass"`) {
			currentPassed = true
		}
		if strings.Contains(line, `"Action":"fail"`) {
			currentPassed = false
		}
	}

	if err != nil && len(results) == 0 {
		// 测试运行失败，可能是编译错误
		results = append(results, TestResult{
			Package: "all",
			Passed:  false,
			Output:  string(output),
		})
	} else if currentPassed || len(results) == 0 {
		// 简单检查：看是否有FAIL
		passed := !strings.Contains(string(output), "FAIL")
		results = append(results, TestResult{
			Package: "all",
			Passed:  passed,
			Output:  string(output),
		})
	}

	return results
}

func printTestDetails(results []TestResult) {
	fmt.Println("\n=== 失败详情 ===")
	for _, r := range results {
		if !r.Passed {
			fmt.Printf("\n--- %s ---\n", r.Package)
			// 打印错误信息
			lines := strings.Split(r.Output, "\n")
			for i, line := range lines {
				if strings.Contains(line, "FAIL") || strings.Contains(line, "Error") || strings.Contains(line, "error") {
					// 打印上下文
					start := i - 2
					if start < 0 {
						start = 0
					}
					end := i + 3
					if end > len(lines) {
						end = len(lines)
					}
					for j := start; j < end; j++ {
						fmt.Println(lines[j])
					}
					fmt.Println()
					break
				}
			}
		}
	}
}

func runCodeReview() []CodeReviewResult {
	var issues []CodeReviewResult

	// 检查常见问题
	filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		// 跳过测试文件
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}

		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil
		}

		var fileIssues []string

		// 检查TODO
		for _, cg := range f.Comments {
			for _, c := range cg.List {
				if strings.Contains(c.Text, "TODO") {
					fileIssues = append(fileIssues, "包含TODO注释: "+c.Text)
				}
			}
		}

		// 检查Println（生产代码应用log）
		content, _ := ioutil.ReadFile(path)
		if strings.Contains(string(content), "fmt.Println") && !strings.HasSuffix(path, "_test.go") {
			// 检查是否在main包或cli包中
			if f.Name.Name != "main" {
				fileIssues = append(fileIssues, "建议使用log代替fmt.Println")
			}
		}

		if len(fileIssues) > 0 {
			issues = append(issues, CodeReviewResult{
				File:     path,
				Issues:   fileIssues,
				Severity: "warning",
			})
		}

		return nil
	})

	return issues
}
