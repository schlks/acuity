package main

import (
	"fmt"
	"github.com/mallardduck/go-http-helpers/pkg/query"
	"net/http"
)

func main() {
	r, _ := http.NewRequest("GET", "/image?path=%2Fdata%2Fmy+image.jpg", nil)
	val := query.String(r, "path", "")
	fmt.Println(val)
}
