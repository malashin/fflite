package main

import (
	"bufio"
	"bytes"
	"log"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	ansi "github.com/k0kubun/go-ansi"
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

// stringIndexInSlice returns the index of the first instance of str in slice,
// or -1 if str is not present in slice.
func stringIndexInSlice(slice []string, str string) int {
	for i, v := range slice {
		if v == str {
			return i
		}
	}
	return -1
}

// readLines reads a whole file into memory
// and returns a slice of its lines.
func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
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

// encodeFile starts ffmpeg command witch passed arguments in ffCommand []string array.
// If batchMode is true BELL sound is turned off.
func encodeFile(ffCommand []string, batchMode bool) []string {
	var progress, eta, lastLine, lastLineUsed string
	var timeSpeed, errorsArray []string
	var duration, currentSecond, currentSpeed, prevSecond float64
	var encodingStarted, encodingFinished, streamMapping, sigint, errors = false, false, false, false, false
	var r *regexp.Regexp
	var startTime time.Time
	var prevUptime time.Duration
	var currentUptime time.Duration

	// Intercept Interrupt signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		sigint = true
	}()

	// Print out the final ffmpeg command.
	ansi.Print("\x1b[36;1m> \x1b[30;1m" + "ffmpeg " + strings.Join(ffCommand[1:], " ") + "\x1b[0m\n")
	// Create exec command to start ffmpeg with.
	cmd := exec.Command("ffmpeg", ffCommand...)
	// Pipe stderr (default ffmpeg info channel) to terminal.
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Panic(err)
	}
	// Pipe terminals stdin to executed ffmpeg instance.
	// Used for answering ffmpegs questions.
	cmd.Stdin = os.Stdin
	// Start ffmpeg.
	cmd.Start()
	// Buffer all the messages coming from ffmpegs stderr.
	scanner := bufio.NewScanner(stderr)
	// Split the lines on `\r?\n`, '\r', "[y/N]".
	scanner.Split(scanLines)
	// For each line.
	for scanner.Scan() {
		line := scanner.Text()
		// Check the state of the program.
		if !encodingStarted && regexp.MustCompile(`Stream mapping:`).MatchString(line) {
			streamMapping = true
		}
		if !encodingStarted && regexp.MustCompile(`.*Press \[q\] to stop.*`).MatchString(line) {
			startTime = time.Now()
			prevUptime = time.Since(startTime)
			encodingStarted = true
			streamMapping = false
		}
		if encodingStarted && regexp.MustCompile(`.*video:.*audio.*subtitle.*other streams.*global headers.*`).MatchString(line) {
			ansi.Print(strings.Repeat(" ", len(line)) + "\r")
			if sigint {
				ansi.Print("\x1b[31;1m" + progress + "%\x1b[0m " + lastLine + "\n")
				ansi.Print("\x1b[31;1mSIGINT\x1b[0m\n")
			} else {
				ansi.Print("\x1b[32;1m100%\x1b[0m et=" + secondsToHHMMSS(strconv.FormatFloat(time.Since(startTime).Seconds(), 'f', -1, 64)) + " " + lastLine + "\n")
			}
			encodingStarted = false
			encodingFinished = true
		}
		// Print out stream mapping information.
		if streamMapping {
			ansi.Print("\x1b[30;1m  " + line + "\x1b[0m\n")
			continue
		}
		// Modify the lines using regexp.
		if r = regexp.MustCompile(`Input #(\d+),.*from \'(.*)\'\:`); r.MatchString(line) {
			line = r.ReplaceAllString(line, "\x1b[32m  INPUT ${1}:\x1b[0m \x1b[32;1m${2}\x1b[0m\n")
		} else if r = regexp.MustCompile(`Output #(\d+),.*to \'(.*)\'\:`); r.MatchString(line) {
			line = r.ReplaceAllString(line, "\x1b[33m  OUTPUT ${1}:\x1b[0m \x1b[33;1m${2}\x1b[0m\n")
		} else if r = regexp.MustCompile(`.*(Duration.*)`); r.MatchString(line) {
			duration = hhmmssmsToSeconds(regexp.MustCompile(`.*Duration: (\d{2}\:\d{2}\:\d{2}\.\d{2}).*`).ReplaceAllString(line, "${1}"))
			line = r.ReplaceAllString(line, "  ${1}\n")
		} else if r = regexp.MustCompile(`.*Stream #(\d+\:\d+)(.*?):(.*)`); r.MatchString(line) {
			line = r.ReplaceAllString(line, "    \x1b[36;1m${1}\x1b[0m \x1b[30;1m"+strings.ToUpper("${2}")+"\x1b[0m${3}\n")
		} else if r = regexp.MustCompile(`(.*No such file.*|.*Invalid data.*|.*At least one output file must be specified.*|.*Unrecognized option.*|.*Option not found.*|.*matches no streams.*|.*not supported.*|.*Invalid argument.*|.*Error.*|.*not exist.*|.*-vf\/-af\/-filter.*|.*No such filter.*|.*does not contain.*|.*Not overwriting - exiting.*|.*\[y\/N\].*)`); r.MatchString(line) {
			line = r.ReplaceAllString(line, "\x1b[31;1m${1}\x1b[0m\n")
			errorsArray = append(errorsArray, line)
		} else if r = regexp.MustCompile(`.* (time=.*) bitrate=.*\/s(.*speed=.*)`); r.MatchString(line) {
			timeSpeed = strings.Split(regexp.MustCompile(`.* time=.*?(\d{2}\:\d{2}\:\d{2}\.\d{2}).* speed=.*?(\d+\.\d+|\d+)x`).ReplaceAllString(line, "$1 $2"), " ")
			currentSecond = hhmmssmsToSeconds(timeSpeed[0])
			currentSpeed, _ = strconv.ParseFloat(timeSpeed[1], 64)
			progress = truncPad(strconv.FormatInt(int64(currentSecond/(duration/100.0)), 10), 3, 'r')
			eta = secondsToHHMMSS(getETA(currentSpeed, duration, currentSecond))
			lastLine = strings.TrimSpace(r.ReplaceAllString(line, "${1}${2}"))
			line = strings.TrimSpace(r.ReplaceAllString(line, "${1}${2}"))
			line = "\x1b[33;1m" + progress + "%\x1b[0m eta=" + eta + " " + line + "\r"
		} else if r = regexp.MustCompile(`.* (time=.*) bitrate=.*\/s(.*)`); r.MatchString(line) {
			currentSecond = hhmmssmsToSeconds(regexp.MustCompile(`.*size=.* time=.*?(\d{2}\:\d{2}\:\d{2}\.\d{2}).*`).ReplaceAllString(line, "$1"))
			currentUptime = time.Since(startTime)
			currentSpeed = 0
			if currentUptime-prevUptime > 0 {
				currentSpeed = (currentSecond - prevSecond) / (currentUptime - prevUptime).Seconds()
			}
			progress = truncPad(strconv.FormatInt(int64(currentSecond/(duration/100.0)), 10), 3, 'r')
			eta = secondsToHHMMSS(getETA(currentSpeed, duration, currentSecond))
			lastLine = strings.TrimSpace(r.ReplaceAllString(line, "${1}${2}"))
			line = strings.TrimSpace(r.ReplaceAllString(line, "${1}${2}"))
			line = "\x1b[33;1m" + progress + "%\x1b[0m eta=" + eta + " " + line + " speed=" + strconv.FormatFloat(currentSpeed, 'f', 2, 64) + "x\r"
		} else if r = regexp.MustCompile(`.*Press \[q\] to stop.*`); r.MatchString(line) {
			line = ""
		} else if encodingStarted {
			if !errors && lastLine != "" {
				ansi.Print("\n")
			}
			errors = true
			// Add timecode and errors to array.
			if lastLineUsed != lastLine {
				lastLineUsed = lastLine
				errorsArray = append(errorsArray, "\x1b[33;1m"+progress+"%\x1b[0m "+lastLine+"\n")
			}
			errorsArray = append(errorsArray, "\x1b[31;1m"+line+"\x1b[0m\n")
			ansi.Print("\x1b[31;1m" + line + "\x1b[0m\n")
			continue
		} else {
			line = ""
		}
		errors = false
		ansi.Print(line)
	}

	// If at least one file was encoded.
	if encodingFinished && !batchMode {
		// Play bell sound.
		ansi.Print("\x07")
	}

	return errorsArray
}
