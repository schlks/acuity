package main

import (
	"fmt"
	"strings"
	"unicode"
	"strconv"
)

func naturalCompare(a, b string) int {
	aRunes, bRunes := []rune(strings.ToLower(a)), []rune(strings.ToLower(b))
	i, j := 0, 0

	for i < len(aRunes) && j < len(bRunes) {
		aIsDigit := unicode.IsDigit(aRunes[i])
		bIsDigit := unicode.IsDigit(bRunes[j])

		if aIsDigit && bIsDigit {
			aStart := i
			for i < len(aRunes) && unicode.IsDigit(aRunes[i]) { i++ }
			bStart := j
			for j < len(bRunes) && unicode.IsDigit(bRunes[j]) { j++ }

			aVal, _ := strconv.ParseUint(string(aRunes[aStart:i]), 10, 64)
			bVal, _ := strconv.ParseUint(string(bRunes[bStart:j]), 10, 64)

			if aVal != bVal {
				if aVal < bVal { return -1 }
				return 1
			}
		} else {
			if aRunes[i] != bRunes[j] {
				if aRunes[i] < bRunes[j] { return -1 }
				return 1
			}
			i++
			j++
		}
	}

	if len(aRunes) == len(bRunes) { return 0 }
	if len(aRunes) < len(bRunes) { return -1 }
	return 1
}

func main() {
	fmt.Println("img_2.jpg vs img_10.jpg:", naturalCompare("img_2.jpg", "img_10.jpg"))
	fmt.Println("img_10.jpg vs img_2.jpg:", naturalCompare("img_10.jpg", "img_2.jpg"))
}
