package main

import (
	"html/template"
	"os"
)

func main() {
	t := template.Must(template.New("").Parse(`<img src="/image?path={{.}}">`))
	t.Execute(os.Stdout, "/data/my image.jpg")
}
