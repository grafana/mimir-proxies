package ddprom

import "strings"

func newEscaper(escapedChars []escapedChar) escaper {
	return escaper{
		escapedChars: escapedChars,
	}
}

type escapedChar struct {
	char    byte
	escaped string
}

type escaper struct {
	escapedChars []escapedChar
}

func (em *escaper) escape(str string) string {
	for _, e := range em.escapedChars {
		str = strings.ReplaceAll(str, string(e.char), e.escaped)
	}
	return str
}

func (em *escaper) unescape(str string) string {
	sb := strings.Builder{}
	for i := 0; i < len(str); {
		if char, l, ok := em.escapedPrefix(str[i:]); ok {
			sb.WriteByte(char)
			i += l
		} else {
			sb.WriteByte(str[i])
			i++
		}
	}
	return sb.String()
}

func (em *escaper) escapedPrefix(str string) (char byte, l int, ok bool) {
	for _, e := range em.escapedChars {
		if strings.HasPrefix(str, e.escaped) {
			return e.char, len(e.escaped), true
		}
	}
	return 0, 0, false
}
