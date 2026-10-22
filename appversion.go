package main

import (
	_ "embed"
	"regexp"
)

//go:embed build/config.yml
var buildConfig string

func currentVersion() string {
	match := regexp.MustCompile(`(?m)^  version: "([^"]+)"`).FindStringSubmatch(buildConfig)
	if len(match) != 2 {
		panic("build/config.yml 缺少应用版本号")
	}
	return match[1]
}
