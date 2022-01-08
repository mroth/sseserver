// Package debug provides a very very very simplistic replacement to deprecate
// our usage of azer/debug.
package debug

import (
	"fmt"
	"log"
	"os"
	"path"
	"runtime"
	"strings"
)

var Enabled = strings.ToLower(os.Getenv("SCMPUFF_DEBUG")) == "true"

func Debug(v ...interface{}) {
	prefix := fmt.Sprintf("DEBUG(%s):", caller())
	args := append([]interface{}{prefix}, v...)
	log.Println(args...)
}

func caller() string {
	_, filename, _, _ := runtime.Caller(2)
	return strings.Split(path.Base(filename), ".")[0]
}
