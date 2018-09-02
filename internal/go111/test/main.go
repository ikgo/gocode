package main

import (
	"fmt"
	"log"
	
	"golang.org/x/tools/go/packages"
)

func main() {
	var src []string
	src = append(src, "github.com/ikgo/core/logger")
	cfg := &packages.Config{
		Mode: packages.LoadTypes,
		Dir: "/projects/ikgo/services",
	}
	pkgs, err := packages.Load(cfg, src...)
	if err != nil {
		log.Fatal(err)
	}
	for _, pkg := range pkgs {
		fmt.Print(pkg.ID, pkg.GoFiles)
	}
}