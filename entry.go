package main

import "regexp"

type Entry struct {
	string
	regex *regexp.Regexp
}

func NewEntry(host string) Entry {
	return Entry{host, regexp.MustCompile(host)}
}

func (e *Entry) MatchesString(str string) bool {
	return e.regex.MatchString(str)
}
