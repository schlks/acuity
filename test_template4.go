package main

import (
	"html/template"
	"os"
)

func main() {
	t := template.Must(template.New("").Parse(`<img src="/image?path={{urlquery .}}">`))
	t.Execute(os.Stdout, "/data/my image.jpg")
}
