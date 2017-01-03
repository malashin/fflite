package main

import (
	"bytes"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// scanLines is a split function for a Scanner that returns each line of text, stripped of any trailing end-of-line marker.
// The end-of-line markers are: `\r?\n`, '\r', "[y/N]".
// The last non-empty line of input will be returned even if it has no newline.
func scanLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	const ynString = "[y/N] "
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		// We have a full newline-terminated line.
		return i + 1, dropCR(data[0:i]), nil
	}
	if i := bytes.IndexByte(data, '\r'); i >= 0 {
		// We have a full CR-terminated line.
		return i + 1, dropCR(data[0:i]), nil
	}
	if i := strings.Index(string(data), ynString); i >= 0 {
		// We have a full line ending with "[y/N]".
		return i + len(ynString), data[0 : i+len(ynString)], nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), dropCR(data), nil
	}
	// Request more data.
	return 0, nil, nil
}

// dropCR drops a terminal \r from the data.
func dropCR(data []byte) []byte {
	if len(data) > 0 && data[len(data)-1] == '\r' {
		return data[0 : len(data)-1]
	}
	return data
}

// hhmmssmsToSeconds converts timecode (HH:MM:SS.MS) to seconds (SS.MS).
func hhmmssmsToSeconds(hhmmssms string) float64 {
	var hh, mm, ss, ms float64
	var buffer string
	length := len(hhmmssms)
	timecode := []string{}

	for i := length - 1; i >= 0; i-- {
		if hhmmssms[i] == '.' {
			ms, _ = strconv.ParseFloat(buffer, 64)
			buffer = ""
		} else if hhmmssms[i] == ':' {
			timecode = append(timecode, buffer)
			buffer = ""
		} else if i == 0 {
			if buffer != "" {
				timecode = append(timecode, string(hhmmssms[i])+buffer)
			} else {
				timecode = append(timecode, string(hhmmssms[i]))
			}
		} else {
			buffer = string(hhmmssms[i]) + buffer
		}
	}

	length = len(timecode)

	if length == 1 {
		ss, _ = strconv.ParseFloat(timecode[0], 64)
	} else if length == 2 {
		ss, _ = strconv.ParseFloat(timecode[0], 64)
		mm, _ = strconv.ParseFloat(timecode[1], 64)
	} else if length == 3 {
		ss, _ = strconv.ParseFloat(timecode[0], 64)
		mm, _ = strconv.ParseFloat(timecode[1], 64)
		hh, _ = strconv.ParseFloat(timecode[2], 64)
	}

	return hh*3600 + mm*60 + ss + ms/100
}

// round rounds floats into integer numbers.
func round(input float64) int64 {
	if input < 0 {
		return int64(math.Ceil(input - 0.5))
	}
	return int64(math.Floor(input + 0.5))
}

// secondsToHHMMSS converts seconds (SS | SS.MS) to timecode (HH:MM:SS).
func secondsToHHMMSS(seconds string) string {
	s, _ := strconv.ParseFloat(seconds, 64)
	hh := math.Floor(s / 3600)
	mm := math.Floor((s - hh*3600) / 60)
	ss := int64(math.Floor(s-hh*3600-mm*60)) + round(math.Remainder(s, 1.0))

	hhString := strconv.FormatInt(int64(hh), 10)
	mmString := strconv.FormatInt(int64(mm), 10)
	ssString := strconv.FormatInt(int64(ss), 10)

	if hh < 10 {
		hhString = "0" + hhString
	}
	if mm < 10 {
		mmString = "0" + mmString
	}
	if ss < 10 {
		ssString = "0" + ssString
	}
	return hhString + ":" + mmString + ":" + ssString
}

// getETA return remaining time for current file encoding based on average speed.
func getETA(currentSpeed, duration, currentSecond float64) string {
	speedArray = append(speedArray, currentSpeed)
	if len(speedArray) >= 30 {
		speedArray = speedArray[len(speedArray)-30 : len(speedArray)]
	}
	var sum float64
	for _, value := range speedArray {
		sum += value
	}
	if sum == 0 {
		return "N/A"
	}
	return strconv.FormatInt(round((duration-currentSecond)/(sum/float64(len(speedArray)))), 10)
}

// truncPad truncs or pads string to needed length.
// If side is 'r' the sring is padded and aligned to the right side.
// Otherwise it is aligned to the left side.
func truncPad(s string, n int, side byte) string {
	if len(s) > n {
		return s[0:n-3] + "\x1b[30;1m...\x1b[0m"
	}
	if side == 'r' {
		return strings.Repeat(" ", n-len(s)) + s
	}
	return s + strings.Repeat(" ", n-len(s))
}

// argsPreset replaces passed arguments with preset values.
func argsPreset(input string) []string {
	var r *regexp.Regexp
	out := input
	if r = regexp.MustCompile(`^\@crf(\d+)$`); r.MatchString(input) {
		out = r.ReplaceAllString(input, "-an -vcodec libx264 -preset medium -crf ${1} -pix_fmt yuv420p -g 0 -map_metadata -1 -map_chapters -1")
	} else if r = regexp.MustCompile(`^\@ac(\d+)$`); r.MatchString(input) {
		out = r.ReplaceAllString(input, "-vn -acodec ac3 -ab ${1}k -map_metadata -1 -map_chapters -1")
	} else if r = regexp.MustCompile(`^\@nometa$`); r.MatchString(input) {
		out = r.ReplaceAllString(input, "-map_metadata -1 -map_chapters -1")
	}
	return strings.Split(out, " ")
}
