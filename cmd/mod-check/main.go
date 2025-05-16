package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/olekukonko/tablewriter"
	"os"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

type Module struct {
	Path      string    `json:"Path"`
	Version   string    `json:"Version"`
	Time      time.Time `json:"Time"`
	Dir       string    `json:"Dir"`
	GoMod     string    `json:"GoMod"`
	GoVersion string    `json:"GoVersion"`
	Sum       string    `json:"Sum"`
	GoModSum  string    `json:"GoModSum"`
}

var goVersion string

func main() {
	flag.StringVar(&goVersion, "go", "", "go version")
	flag.Parse()
	if goVersion == "" {
		fmt.Println("You must set --go flag")
		return
	}
	fmt.Println("🔍 检查依赖版本兼容性，指定Go版本:", goVersion)

	// 获取所有依赖
	cmd := exec.Command("go", "list", "-m", "-json", "all")
	out, err := cmd.Output()
	if err != nil {
		fmt.Println("检查失败:", err)
		return
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"依赖项", "依赖版本(建议)", "依赖Go版本"})

	var incompatible []string

	decoder := json.NewDecoder(bytes.NewReader(out))
	for decoder.More() {
		var mod Module
		if err := decoder.Decode(&mod); err != nil {
			continue
		}

		// 下载 .mod 文件查看 Go 版本要求
		modFile := downloadMod(mod.Path, mod.Version)
		if modFile == "" {
			continue
		}
		if mod.GoVersion == "" {
			mod.GoVersion = goVersion
		}
		if semver.Compare("v"+goVersion, "v"+mod.GoVersion) != -1 {
			continue
		}

		version := findLowerVersion(mod.Path, goVersion)
		table.Append([]string{
			mod.Path,
			mod.Version + " => " + version,
			FRed("v" + mod.GoVersion),
		})
		incompatible = append(incompatible, fmt.Sprintf("go get %s@%s", mod.Path, version))
	}
	if len(incompatible) > 0 {
		table.Render()
		fmt.Println("🛠️ 替换建议：")
		for _, mod := range incompatible {
			fmt.Println(mod)
		}
	} else {
		fmt.Println("👉 所有依赖满足")
	}
}

// FRed 红色
func FRed(s string) string {
	return "\033[31m" + s + "\033[0m"
}

func downloadMod(path, version string) string {
	out, _ := exec.Command("go", "mod", "download", "-json", fmt.Sprintf("%s@%s", path, version)).Output()
	var result map[string]interface{}
	_ = json.Unmarshal(out, &result)
	if gomod, ok := result["GoMod"].(string); ok {
		data, err := os.ReadFile(gomod)
		if err == nil {
			return string(data)
		}
	}
	return ""
}

func parseGoVersion(mod string) string {
	for _, line := range strings.Split(mod, "\n") {
		if strings.HasPrefix(line, "go ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "go "))
		}
	}
	return ""
}

func findLowerVersion(path, maxGoVersion string) string {
	cmd := exec.Command("go", "list", "-m", "-versions", path)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	parts := strings.Fields(string(out))
	if len(parts) < 2 {
		return ""
	}
	for i := len(parts) - 1; i >= 1; i-- {
		version := parts[i]
		modFile := downloadMod(path, version)
		goVer := parseGoVersion(modFile)
		if goVer == "" || semver.Compare("v"+goVer, "v"+maxGoVersion) <= 0 {
			return version
		}
	}
	return ""
}
