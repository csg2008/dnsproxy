package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strings"
)

var version = map[string]interface{}{}
var appName string
var buildRev string
var buildDate string
var buildVersion string

func main() {
	var showVer = flag.Bool("v", false, "show application version and exit")
	var profile = flag.String("p", "", "write cpu profile to file")
	var version = map[string]string{
		"rev":     buildRev,
		"date":    buildDate,
		"version": buildVersion,
	}

	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: ", strings.TrimRight(filepath.Base(os.Args[0]), filepath.Ext(os.Args[0])), " [OPTIONS]")
		fmt.Fprintln(os.Stderr, "Welcome to use dns proxy service")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Options:")

		flag.PrintDefaults()
	}

	flag.Parse()

	if *showVer {
		fmt.Println(appName + " " + "Ver: " + buildVersion + " build: " + buildDate + " Rev:" + buildRev)
		return
	}

	// todo 计划改为可以指定生成性能分析文件类型，方便问题排查
	if *profile != "" {
		f, err := os.Create(*profile)
		if err != nil {
			fmt.Println("proxy: create cpu pprof file failed,", err)
			return
		}

		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	var proxy = NewProxy(version)
	var err = proxy.Run()
	if nil != err {
		fmt.Println(err)
	}
}
