//go:build windows

package main

import (
	"fmt"
	"os"
)

func run() error {
	args := os.Args[1:]

	// Windows 只支持 config、version、help 和 --prompt 模式
	switch {
	case len(args) == 0:
		fmt.Fprintln(os.Stderr, "Windows 暂不支持交互模式")
		fmt.Fprintln(os.Stderr, "用法: pg --prompt \"你的问题\"")
		os.Exit(1)
	case args[0] == "config":
		runConfig(args[1:])
	case args[0] == "version" || args[0] == "-version" || args[0] == "--version":
		printVersion()
	case args[0] == "help" || args[0] == "-help" || args[0] == "--help":
		printHelp()
	default:
		fmt.Fprintln(os.Stderr, "Windows 暂不支持交互模式")
		fmt.Fprintln(os.Stderr, "用法: pg --prompt \"你的问题\"")
		os.Exit(1)
	}
	return nil
}
