package wordset

import (
	"unicode/utf8"
)

type WordSet struct {
	letters map[rune]int
}

func MakeWordSet(word string) (WordSet) {
	newset := NewWordSet()
	for _, k := range word { newset.letters[k] += 1 }
	return newset
}

func NewWordSet() (WordSet) {
	var w WordSet
	w.letters = make(map[rune]int)
	return w
}

func CopyWordSet(other WordSet) (WordSet) {
	var w WordSet
	w.letters = make(map[rune]int)
	for k, v := range other.letters { w.letters[k] = v }
	return w
}

func (this WordSet) Subtract(other WordSet) (WordSet) {
	newset := CopyWordSet(this)
	for k, v := range other.letters { newset.letters[k] -= v }
	return newset
}

func (this WordSet) Magnitudes() (int, int, int, int) {
	positive, negative := 0, 0
	for _, v := range this.letters {
		if v > 0 {
			positive += v
		} else {
			negative += v
		}
	}
	//     positive  negative  net                  absolute
	return positive, negative, positive + negative, positive - negative
}

func Utf8Split(str string, at int) (string, string) {
	i := 0
	out1 := ""
	out2 := ""
	for _, r := range str {
		if i < at {
			out1 += string(r)
		} else {
			out2 += string(r)
		}
		i += 1
	}

	return out1, out2
}

func Levenshtein(str1, str2 string) int {
	s1len := utf8.RuneCountInString(str1)
	column := make([]int, s1len+1)
 
	for y := 1; y <= s1len; y++ {
		column[y] = y
	}

	x := 0
	for _, rs2 := range str2 {
		column[0] = x + 1
		lastkey := x
		y := 0
		for _, rs1 := range str1 {
			oldkey := column[y + 1]
			var incr int
			if rs1 != rs2 {
				incr = 1
			}

			column[y + 1] = minimum(column[y+1]+1, column[y]+1, lastkey+incr)
			lastkey = oldkey
			y += 1
		}
		x += 1
	}
	return column[s1len]
}
 
func minimum(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
	} else {
		if b < c {
			return b
		}
	}
	return c
}
