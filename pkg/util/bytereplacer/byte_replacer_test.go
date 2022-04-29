package bytereplacer

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestByteReplacer(t *testing.T) {
	input := `foo$^bar@baz*`
	replacer := New(regexp.MustCompile(`[^\w\-/.:]`), '_')

	assert.Equal(t, "foo__bar_baz_", replacer.Replace(input))
}

// BenchmarkByteReplacer results:
// goos: darwin
// goarch: amd64
// cpu: Intel(R) Core(TM) i7-9750H CPU @ 2.60GHz
// BenchmarkByteReplacer
// BenchmarkByteReplacer/byte_replacer
// BenchmarkByteReplacer/byte_replacer-12         	17365294	        64.54 ns/op
// BenchmarkByteReplacer/byte_replacer#01
// BenchmarkByteReplacer/byte_replacer#01-12      	 1255816	       937.3 ns/op
func BenchmarkByteReplacer(b *testing.B) {
	input := `foo$^bar@baz_!bee`
	re := regexp.MustCompile(`[^\w\-/.:]`)
	replacer := New(re, '_')

	b.Run("byte_replacer", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			replacer.Replace(input)
		}
	})

	b.Run("byte_replacer", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			re.ReplaceAllString(input, "_")
		}
	})
}
