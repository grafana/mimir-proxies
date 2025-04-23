package whisperconverter

import (
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
)

func TestCommandPass1(t *testing.T) {
	tests := []struct {
		name                      string
		startDate                 string
		endDate                   string
		expectedIntermediateFiles []string
	}{
		{
			name:      "allDates",
			startDate: "2022-04-28",
			endDate:   "2022-05-04",
			expectedIntermediateFiles: []string{
				"2022-04-28.intermediate",
				"2022-04-29.intermediate",
				"2022-04-30.intermediate",
				"2022-05-01.intermediate",
				"2022-05-02.intermediate",
				"2022-05-03.intermediate",
				"2022-05-04.intermediate",
				"processedMetrics.intermediate",
			},
		},
		{
			name:      "limitedDates",
			startDate: "2022-05-02",
			endDate:   "2022-05-04",
			expectedIntermediateFiles: []string{
				"2022-05-02.intermediate",
				"2022-05-03.intermediate",
				"2022-05-04.intermediate",
				"processedMetrics.intermediate",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tmpInDir, err := os.MkdirTemp("/tmp", test.name+"-in-*")
			require.NoError(t, err)
			defer func() {
				_ = os.RemoveAll(tmpInDir)
			}()

			tmpIntermediateDir, err := os.MkdirTemp("/tmp", test.name+"-intermediate-*")
			require.NoError(t, err)
			defer func() {
				_ = os.RemoveAll(tmpInDir)
			}()

			asdfTimes, err := ToTimes([]string{
				"2022-05-01",
				"2022-05-02",
				"2022-05-03",
				"2022-05-04",
			})
			require.NoError(t, err)

			err = CreateWhisperFile(tmpInDir+"/asdf.wsp", asdfTimes)
			require.NoError(t, err)

			qwerTimes, err := ToTimes([]string{
				"2022-04-28",
				"2022-04-29",
				"2022-04-30",
			})
			require.NoError(t, err)

			err = CreateWhisperFile(tmpInDir+"/qwer.wsp", qwerTimes)
			require.NoError(t, err)

			dates := make([]time.Time, 0)
			startDate, err := ToTime(test.startDate)
			require.NoError(t, err)
			endDate, err := ToTime(test.endDate)
			require.NoError(t, err)
			for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
				dates = append(dates, d)
			}

			c := NewWhisperConverter(
				"",
				tmpInDir,
				regexp.MustCompile(`\.wsp$`),
				2,
				1,
				0,
				labels.FromStrings(),
				dates,
				log.NewNopLogger(),
			)

			require.NoError(t, c.CommandPass1("", tmpIntermediateDir, true))

			actualFiles, err := ListFilesInDir(tmpIntermediateDir)
			require.NoError(t, err)

			require.ElementsMatch(t, test.expectedIntermediateFiles, actualFiles)
		})
	}
}
