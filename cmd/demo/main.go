package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	ingestPath := flag.String("ingest", "", "自喂数据文件路径 (.json 或 .csv)，走「接入器→标准化」全链路")
	flag.Parse()

	if *ingestPath != "" {
		if err := runIngestDemo(*ingestPath); err != nil {
			fmt.Fprintf(os.Stderr, "演示失败: %v\n", err)
			os.Exit(1)
		}
		return
	}
	runScriptedDemo()
}
