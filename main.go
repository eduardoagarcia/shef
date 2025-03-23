package main

import (
	"math/rand"
	"time"

	"github.com/eduardoagarcia/shef/internal"
)

func init() {
	rand.New(rand.NewSource(time.Now().UnixNano()))
}

func main() {
	internal.Run()
}
