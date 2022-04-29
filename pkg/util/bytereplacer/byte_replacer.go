package bytereplacer

import (
	"math"
	"regexp"
	"strings"
)

// Replacer performs replacements in a string
type Replacer interface {
	Replace(string) string
}

var _ Replacer = &strings.Replacer{}

// New creates a new Replacer that replaces the bytes matched by the regexp provided
// by the replacement provided, but it does not run the regexp
func New(re *regexp.Regexp, replacement byte) *strings.Replacer {
	var oldnew []string
	for i := 0; i <= math.MaxUint8; i++ {
		s := string(byte(i))
		if re.MatchString(s) {
			oldnew = append(oldnew, s, string(replacement))
		}
	}
	// The strings.NewReplacer we're instantiating will be optimized to use strings.byteReplacer as we're passing in one char strings
	return strings.NewReplacer(oldnew...)
}
