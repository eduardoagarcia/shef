package main

import (
	//"bytes"
	//"encoding/json"
	//"fmt"
	//"log"
	//"math"
	"math/rand"
	//"os"
	//"os/exec"
	//"os/signal"
	//"path/filepath"
	//"sort"
	//"strconv"
	//"strings"
	//"sync"
	//"text/template"
	"time"

	//"github.com/AlecAivazis/survey/v2"
	//"github.com/agnivade/levenshtein"
	"github.com/eduardoagarcia/shef/internal"
	//"github.com/urfave/cli/v2"
	//"gopkg.in/yaml.v3"
)

func init() {
	rand.New(rand.NewSource(time.Now().UnixNano()))
}

func main() {
	internal.Run()
}
