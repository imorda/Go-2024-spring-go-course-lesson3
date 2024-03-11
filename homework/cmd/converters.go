package main

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

type Converter interface {
	ConvertChunk(string) string
}

type TrimSpaces struct {
	leadingSpacesProcessed bool
	undecidedSpaces        strings.Builder
}

func (state *TrimSpaces) ConvertChunk(inputChunk string) string {
	if !state.leadingSpacesProcessed {
		inputChunk = strings.TrimLeftFunc(inputChunk, unicode.IsSpace)
		if inputChunk != "" {
			state.leadingSpacesProcessed = true
		}
	}
	if state.leadingSpacesProcessed {
		nonSpaceI := strings.LastIndexFunc(inputChunk, func(x rune) bool { return !unicode.IsSpace(x) })
		if nonSpaceI < 0 {
			state.undecidedSpaces.WriteString(inputChunk)
			return ""
		}

		_, lastNonSpaceWidth := utf8.DecodeRuneInString(inputChunk[nonSpaceI:])
		state.undecidedSpaces.WriteString(inputChunk[:nonSpaceI+lastNonSpaceWidth])
		result := state.undecidedSpaces.String()
		state.undecidedSpaces.Reset()
		state.undecidedSpaces.WriteString(inputChunk[nonSpaceI+lastNonSpaceWidth:])
		return result
	}
	return inputChunk
}

type UpperCase struct{}

func (state UpperCase) ConvertChunk(inputChunk string) string {
	return strings.ToUpper(inputChunk)
}

type LowerCase struct{}

func (state LowerCase) ConvertChunk(inputChunk string) string {
	return strings.ToLower(inputChunk)
}
