package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	ansi "github.com/k0kubun/go-ansi"
)

// help returns usage information and programm version.
func help() {
	consolePrint("fflite is FFmpeg wrapper for minimalistic progress visualization while keeping the flexability of CLI.\n")
	consolePrint("fflite version \x1b[33;1m" + version + "\x1b[0m.\n")
	consolePrint("\n\x1b[33;1mUsage:\x1b[0m\n")
	consolePrint("    It uses the same syntax as FFmpeg:\n\n")
	consolePrint("    fflite [fflite_option] [global_options] {[input_file_options] -i input_file} ... {[output_file_options] output_file} ...\n\n")
	consolePrint("    For batch execution pass \".txt\" filelist, \"list:file1 file2 \"file 3\"\" or a glob pattern as input.\n")
	consolePrint("    Once the first input file is specified input and output files can be named using `[prefix?]old::new` pattern. This will take the first input name and replace `old` string with the `new` string. If `?` is present, everything before `?` will be used as a prefix for new filenames (`fflite -i film_video.mp4 -i folder?video.mp4::audio.ac3`).\n")
	consolePrint("    Input ranges can be passed to -filter_complex. \"[0-1:1]\" becomes \"[0:1][1:1]\"; \"[0:0-1]\" becomes \"[0:0][0:1]\"; \"[0-1:2-3]\" becomes \"[0:2][0:3][1:2][1:3]\" and so on. Example: \"-filter_complex [0:1-6]amerge=inputs=6[a]\" becomes \"-filter_complex [0:1][0:2][0:3][0:4][0:5][0:6]amerge=inputs=6[a]\".\n")
	consolePrint("    Preset arguments are replaced with specific strings.\n")
	consolePrint("\n\x1b[33;1mOptions:\x1b[0m\n")
	consolePrint("    ffmpeg       original ffmpeg text output\n")
	consolePrint("    version      print fflite version and check for updates\n")
	consolePrint("    update       update fflite version using \"go get\"\n")
	consolePrint("    nologs       do not create \".#err\" error log files\n")
	consolePrint("    cwdlogs      save \".#err\" error log files in the current work directory\n")
	consolePrint("    crop         audomated cropDetect module \"fflite crop[crop_number:crop_limit] -i input_file\"\n")
	consolePrint("    sync         sync 2nd input audio files duration to the duration on the first input \"fflite sync -i input_file -i input_file\"\n")
	consolePrint("    mute         removes bell sound at the end of ecoding\n")
	consolePrint("\n\x1b[33;1mPresets:\x1b[0m\n")
	// Find maximum length of preset keys.
	length := 0
	for key := range presets {
		if len(key[2:len(key)-1]) > length {
			length = len(key[2 : len(key)-1])
		}
	}
	// Sort all presets alphabetically.
	var keys []string
	for k := range presets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	// Print out all presets.
	for _, key := range keys {
		consolePrint("    " + key[2:len(key)-1] + strings.Repeat(" ", length-len(key[2:len(key)-1])) + "    " + presets[key] + "\n")
	}
	consolePrint("\n\x1b[33;1mFFmpeg documentation:\x1b[0m\n")
	consolePrint("    www.ffmpeg.org/ffmpeg-all.html\n")
	consolePrint("\n\x1b[33;1mGithub page:\x1b[0m\n")
	consolePrint("    github.com/malashin/fflite\n")
}

// contains reports whether string is in string slice.
func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// scanLines is a split function for a Scanner that returns each line of text, stripped of any trailing end-of-line marker.
// The end-of-line markers are: `\r?\n`, '\r', "[y/N]".
// The last non-empty line of input will be returned even if it has no newline.
func scanLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	const ynString = "[y/N] "
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, '\r'); (i >= 0) && (bytes.IndexByte(data, '\n') != i+1) {
		// We have a full CR-terminated line.
		return i + 1, dropCR(data[0:i]), nil
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

// hhmmssmsToSeconds converts timecode (H:M:S.MS) to seconds float64 (S.MS).
func hhmmssmsToSeconds(hhmmssms string) float64 {
	var hh, mm, ss, ms float64
	var buffer string
	length := len(hhmmssms)
	timecode := []string{}

	for i := length - 1; i >= 0; i-- {
		if hhmmssms[i] == '.' {
			buffer = "." + buffer
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

	return hh*3600 + mm*60 + ss + ms
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
func getETA(currentSpeed, duration, currentSecond float64, speedArray []float64) (string, []float64) {
	speedArray = append(speedArray, currentSpeed)
	if len(speedArray) >= 30 {
		speedArray = speedArray[len(speedArray)-30 : len(speedArray)]
	}
	var sum float64
	for _, value := range speedArray {
		sum += value
	}
	if sum == 0 {
		return "N/A", speedArray
	}
	return strconv.FormatInt(round((duration-currentSecond)/(sum/float64(len(speedArray)))), 10), speedArray
}

// truncPad truncs or pads string to needed length.
// If side is 'r' the string is padded and aligned to the right side.
// Otherwise it is aligned to the left side.
func truncPad(s string, n int, side byte) string {
	len := utf8.RuneCountInString(s)
	if len > n {
		return string([]rune(s)[0:n-3]) + "\x1b[30;1m...\x1b[0m"
	}
	if side == 'r' {
		return strings.Repeat(" ", n-len) + s
	}
	return s + strings.Repeat(" ", n-len)
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

// sliceFromFileOrGlob returns slice of strings, each string is a line in input file if batchFile is true.
// Otherwise input is read as a glob pattern.
func sliceFromFileOrGlob(input string, batchFile bool) ([]string, error) {
	if batchFile {
		return readLines(input)
	}

	if strings.HasPrefix(input, "list:") {
		input = strings.Replace(input, "list:", "", 1)
		input = strings.TrimSpace(input)
		r := csv.NewReader(strings.NewReader(input))
		r.Comma = ' '
		fields, err := r.Read()
		if err != nil {
			return []string{}, err
		}
		return fields, nil
	}

	return filepath.Glob(input)
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

// consolePrint prints str to console while cursor is hidden.
func consolePrint(str ...interface{}) {
	if !isTerminal {
		for _, s := range str {
			fmt.Print(stripEscapesFromString(fmt.Sprintf("%v", s)))
		}
		return
	}
	ansi.CursorHide()
	ansi.Print(str...)
	ansi.CursorShow()
}

// bell rings bell send by typing bell ANSI code to terminal.
func bell(mute bool) {
	if mute {
		return
	}
	if !isTerminal {
		return
	}
	consolePrint("\x07")
}

// isWarningSpamming checks if warning message comes up too often and omits it if needed.
func isWarningSpamming(array []string, str string, spamList map[string]bool) bool {
	if !spamList[str] {
		count := 0
		limit := 10
		for _, v := range array {
			if v == str {
				count++
			}
		}
		if count >= limit {
			spamList[str] = true
			consolePrint("\n     \x1b[33;1mOmitting further warnings: \x1b[33m" + str + "\x1b[0m\n")
			return true
		}
		return false
	}
	return true
}

func parseInput(line string) string {
	return regexpMap["input"].ReplaceAllString(line, "\x1b[32m  INPUT ${1}:\x1b[0m \x1b[32;1m${2}\x1b[0m\n")
}

func parseOutput(line string) string {
	return regexpMap["output"].ReplaceAllString(line, "\x1b[33m  OUTPUT ${1}:\x1b[0m \x1b[33;1m${2}\x1b[0m\n")
}

func parseDuration(line string) (string, float64) {
	duration := hhmmssmsToSeconds(regexpMap["durationHHMMSSMS"].ReplaceAllString(line, "${1}"))
	line = regexpMap["duration"].ReplaceAllString(line, "  ${1}\n")
	return line, duration
}

func parseStream(line string) string {
	lng := regexpMap["stream"].ReplaceAllString(line, "${2}")
	if lng == "" {
		return regexpMap["stream"].ReplaceAllString(line, "    \x1b[36;1m${1}\x1b[0m ${3}\n")
	}
	return regexpMap["stream"].ReplaceAllString(line, "    \x1b[36;1m${1}\x1b[0m \x1b[30;1m${2}\x1b[0m ${3}\n")
}

func parseErrors(line string, lastLineFull string, batchMode bool, errorsArray []string) (string, []string) {
	if (lastLineFull != "") && (lastLineFull[len(lastLineFull)-1]) == '\r' {
		consolePrint("\n")
	}
	line = regexpMap["errors"].ReplaceAllString(line, "     \x1b[31;1m${1}\x1b[0m\n")
	if batchMode {
		errorsArray = append(errorsArray, line)
	}
	return line, errorsArray
}

func parseWarnings(line string, lastLineFull string, warningArray []string, warningSpam map[string]bool) (string, []string) {
	line = strings.TrimSpace(regexpMap["warnings"].ReplaceAllString(line, "${1}"))
	if isWarningSpamming(warningArray, line, warningSpam) {
		line = ""
		return line, warningArray
	}
	warningArray = append(warningArray, line)
	if (lastLineFull != "") && (lastLineFull[len(lastLineFull)-1]) == '\r' {
		consolePrint("\n")
	}
	line = regexpMap["warnings"].ReplaceAllString(line, "     \x1b[33;1m"+line+"\x1b[0m\n")
	return line, warningArray
}

func parseEncoding(line string, lastLineFull string, duration float64, speedArray []float64) (string, string, string, []float64) {
	timeSpeed := strings.Split(regexpMap["timeSpeed"].ReplaceAllString(line, "$1 $2"), " ")
	currentSecond := hhmmssmsToSeconds(timeSpeed[0])
	currentSpeed, _ := strconv.ParseFloat(timeSpeed[1], 64)
	progress := "N\\A"
	eta := "N\\A"
	line = strings.TrimSpace(regexpMap["encoding"].ReplaceAllString(line, "${1} ${3} \x1b[33;1m${2}\x1b[0m"))
	if strings.Contains(line, "dup=0 ") {
		line = strings.Replace(line, "dup=0 ", "", -1)
	}
	if strings.Contains(line, "drop=0 ") {
		line = strings.Replace(line, "drop=0 ", "", -1)
	}
	lastLine := line
	if duration > 0 {
		progress = truncPad(strconv.FormatInt(int64(currentSecond/(duration/100.0)), 10), 3, 'r')
		eta, speedArray = getETA(currentSpeed, duration, currentSecond, speedArray)
		eta = secondsToHHMMSS(eta)
		line = "\x1b[33;1m" + progress + "%\x1b[0m eta=" + eta + " " + line
	} else {
		line = "\x1b[33;1m" + progress + "\x1b[0m " + line
	}
	if (len(lastLineFull) > 0) && (lastLineFull[len(lastLineFull)-1] == '\r') && (len(line) < len(strings.TrimSpace(lastLineFull))) {
		line += strings.Repeat(" ", len(strings.TrimSpace(lastLineFull))-len(line))
	}
	line += "\r"
	return line, lastLine, progress, speedArray
}

func parseEncodingNoSpeed(line string, lastLineFull string, duration float64, startTime time.Time, prevUptime time.Duration, prevSecond float64, speedArray []float64) (string, string, string, []float64) {
	currentSecond := hhmmssmsToSeconds(regexpMap["currentSecond"].ReplaceAllString(line, "$1"))
	currentUptime := time.Since(startTime)
	currentSpeed := 0.0
	if currentUptime-prevUptime > 0 {
		currentSpeed = (currentSecond - prevSecond) / (currentUptime - prevUptime).Seconds()
	}
	progress := "N\\A"
	eta := "N\\A"
	line = strings.TrimSpace(regexpMap["encodingNoSpeed"].ReplaceAllString(line, "${1} speed="+strconv.FormatFloat(currentSpeed, 'f', 2, 64)+"x \x1b[33;1m${2}\x1b[0m"))
	if strings.Contains(line, "dup=0 ") {
		line = strings.Replace(line, "dup=0 ", "", -1)
	}
	if strings.Contains(line, "drop=0 ") {
		line = strings.Replace(line, "drop=0 ", "", -1)
	}
	lastLine := line
	if duration > 0 {
		progress := truncPad(strconv.FormatInt(int64(currentSecond/(duration/100.0)), 10), 3, 'r')
		eta, speedArray = getETA(currentSpeed, duration, currentSecond, speedArray)
		eta = secondsToHHMMSS(eta)
		line = "\x1b[33;1m" + progress + "%\x1b[0m eta=" + eta + " " + line
	} else {
		line = "\x1b[33;1m" + progress + "\x1b[0m " + line + " speed=" + strconv.FormatFloat(currentSpeed, 'f', 2, 64) + "x"
	}
	if (len(lastLineFull) > 0) && (lastLineFull[len(lastLineFull)-1] == '\r') && (len(line) < len(strings.TrimSpace(lastLineFull))) {
		line += strings.Repeat(" ", len(strings.TrimSpace(lastLineFull))-len(line))
	}
	line += "\r"
	return line, lastLine, progress, speedArray
}

func parseEncodingErrors(line string, lastLineFull string, lastLineUsed string, lastLine string, errorsArray []string, progress string) (string, string, []string) {
	if (lastLineFull != "") && (lastLineFull[len(lastLineFull)-1]) == '\r' {
		consolePrint("\n")
	}
	// Add timecode and errors to array.
	if lastLineUsed != lastLine {
		lastLineUsed = lastLine
		errorsArray = append(errorsArray, "\x1b[33;1m"+progress+"%\x1b[0m "+regexpMap["timeSpeed"].ReplaceAllString(lastLine, "time=${1}")+"\n")
	}
	line = "     \x1b[31;1m" + line + "\x1b[0m\n"
	errorsArray = append(errorsArray, line)
	return line, lastLineUsed, errorsArray
}

func parseFinish(line string, sigint bool, progress string, lastLine string, startTime time.Time) (bool, bool) {
	consolePrint(strings.Repeat(" ", len(line)) + "\r")
	if sigint {
		consolePrint("\x1b[31;1m" + progress + "%\x1b[0m " + lastLine + "\n")
		consolePrint("\x1b[31;1mSIGINT\x1b[0m\n")
	} else {
		consolePrint("\x1b[32;1m100%\x1b[0m et=" + secondsToHHMMSS(strconv.FormatFloat(time.Since(startTime).Seconds(), 'f', -1, 64)) + " " + lastLine + "\n")
	}
	encodingStarted := false
	encodingFinished := true
	return encodingStarted, encodingFinished
}

func stripEscapesFromString(str string) string {
	return regexp.MustCompile(`(\x1b\[\d+(;\d+)*m)`).ReplaceAllString(str, "")
}

func writeStringArrayToFile(filename string, strArray []string, perm os.FileMode) {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, perm)
	if err != nil {
		log.Panic(err)
	}
	defer f.Close()
	for _, v := range strArray {
		if _, err = f.WriteString(stripEscapesFromString(v)); err != nil {
			log.Panic(err)
		}
	}
}

// argsPreset replaces passed arguments with preset values.
func argsPreset(input string) []string {
	out := []string{input}
	for key, value := range presets {
		if r := regexp.MustCompile(key); r.MatchString(input) {
			out = strings.Split(r.ReplaceAllString(input, value), " ")
		}
	}
	return out
}

func getUpstreamVersion() string {
	resp, err := http.Get("https://raw.githubusercontent.com/malashin/fflite/master/fflite.go")
	if err != nil {
		consolePrint("\x1b[31;1m", err, "\x1b[0m\n")
		return ""
	}
	defer resp.Body.Close()
	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		consolePrint("\x1b[31;1m")
		consolePrint(err)
		consolePrint("\x1b[0m\n")
		return ""
	}
	r := regexp.MustCompile(`var version = "(.*)"`)
	version := r.FindString(string(bytes))
	version = r.ReplaceAllString(version, "$1")
	return version
}

func updateVersion() error {
	upstreamVersion := getUpstreamVersion()
	if upstreamVersion == "" {
		return nil
	}
	if version == upstreamVersion {
		consolePrint("fflite version \x1b[32;1m" + version + "\x1b[0m.\n")
		consolePrint("\x1b[32;1mYour fflite is up to date.\x1b[0m\n")
		return nil
	}
	consolePrint("fflite version is \x1b[31;1m" + version + "\x1b[0m.\n")
	consolePrint("Latest version is \x1b[33;1m" + upstreamVersion + "\x1b[0m.\n")
	consolePrint("\x1b[31;1mYour fflite is out of date.\x1b[0m\n")
	consolePrint("\x1b[30;1mgo get -u -v github.com/malashin/fflite\x1b[0m\n")
	cmd := exec.Command("go", "get", "-u", "-v", "github.com/malashin/fflite")
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Start()
	scanner := bufio.NewScanner(stderr)
	scanner.Split(scanLines)
	for scanner.Scan() {
		consolePrint(scanner.Text() + "\n")
	}
	return nil
}

func parseOptions(input []string) (ffmpeg bool, nologs bool, cwdlogs bool, crop bool, cropDetectNumber int, cropDetectLimit float64, sync bool, mute bool, args []string) {
	switch {
	// "ffmpeg" run the same command in ffmpeg instead of fflite.
	case input[0] == "ffmpeg":
		ffmpeg = true
		args = input[1:]
	// "nologs" don't save error log files.
	case input[0] == "nologs":
		nologs = true
		args = input[1:]
	// "cwdlogs" save error log files in the current work directory.
	case input[0] == "cwdlogs":
		cwdlogs = true
		args = input[1:]
	// "crop" runs cropDetect on input file.
	case regexpMap["cropMode"].MatchString(input[0]):
		crop = true
		args = input[1:]
		cropDetectNumber = 5      // default values
		cropDetectLimit = 0.10625 // default values
		cropModeValues := regexpMap["cropMode"].FindStringSubmatch(input[0])
		// If crop argument was passed with crop values.
		if cropModeValues[1] != "" {
			values := strings.Split(cropModeValues[1], ":")
			// If there is no ":" in the crop values.
			if len(values) == 1 {
				v, err := strconv.ParseFloat(values[0], 64)
				if err != nil {
					consolePrint("\x1b[31;1m", err, "\x1b[0m\n")
					return
				}
				// If crop value is less then 1 use it as cropDetect limit, cropDetect number otherwise.
				if v < 1 {
					cropDetectLimit = v
				} else {
					cropDetectNumber = int(round(v))
				}
			} else {
				// Parse crop values if they are separated with ":".
				i, err := strconv.ParseInt(values[0], 10, 64)
				cropDetectNumber = int(i)
				if err != nil {
					consolePrint("\x1b[31;1m", err, "\x1b[0m\n")
					return
				}
				cropDetectLimit, err = strconv.ParseFloat(values[1], 64)
				if err != nil {
					consolePrint("\x1b[31;1m", err, "\x1b[0m\n")
					return
				}
			}
		}
	// "sync" speeds up or slows down audio file for it's duration to match video files duration.
	case input[0] == "sync":
		sync = true
		args = input[1:]
	case input[0] == "mute":
		mute = true
		args = input[1:]
	// "update" check upstream version.
	case input[0] == "version":
		upstreamVersion := getUpstreamVersion()
		if version != upstreamVersion {
			consolePrint("fflite version is \x1b[31;1m" + version + "\x1b[0m.\n")
			consolePrint("Latest version is \x1b[33;1m" + upstreamVersion + "\x1b[0m.\n")
			consolePrint("\x1b[31;1mYour fflite is out of date.\x1b[0m\n")
			consolePrint("Use this command to update it:\n")
			consolePrint("\x1b[30;1mfflite update\x1b[0m\n")
		} else {
			consolePrint("fflite version \x1b[32;1m" + version + "\x1b[0m.\n")
			consolePrint("\x1b[32;1mYour fflite is up to date.\x1b[0m\n")
		}
		os.Exit(0)
	case input[0] == "update":
		err := updateVersion()
		if err != nil {
			consolePrint("\x1b[31;1m", err, "\x1b[0m\n")
		}
		os.Exit(0)
	default:
		args = input
	}
	return
}

// cropDetect parses the input file for the necessary cropping parameters.
func cropDetect(firstInput string, cropDetectCount int, cropDetectLimit float64) {
	cropDetectDur := "2" // One second in ffmpeg format
	cropDetectParams := strconv.FormatFloat(cropDetectLimit, 'f', -1, 64) + ":2:0"
	cmd := exec.Command("ffmpeg", "-i", firstInput)
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil && fmt.Sprint(err) != "exit status 1" {
		consolePrint("\x1b[31;1m", err, "\x1b[0m\n")
	}
	output := string(regexpMap["durationHHMMSSMS"].Find(stdoutStderr))
	duration := hhmmssmsToSeconds(regexpMap["durationHHMMSSMS"].ReplaceAllString(output, "${1}"))
	consolePrint("\x1b[32;1m", firstInput, "\x1b[0m\n")
	consolePrint("\x1b[30;1m", "Running cropDetect ", cropDetectCount, " times, with the following parameters ", cropDetectParams, "\x1b[0m\n")
	for i := 1; i <= cropDetectCount; i++ {
		var cropArrayLocal []crop
		tempDur := duration * float64(i) / (float64(cropDetectCount) + 1.0)
		ffCommand := []string{"-ss",
			strconv.FormatFloat(tempDur, 'f', -1, 64),
			"-i",
			firstInput,
			"-vf",
			"cropdetect=" + cropDetectParams,
			"-t",
			cropDetectDur,
			"-an",
			"-f",
			"null",
			"nul"}
		cmd := exec.Command("ffmpeg", ffCommand...)
		stdoutStderr, err := cmd.CombinedOutput()
		if err != nil {
			consolePrint("\x1b[31;1m", err, "\x1b[0m\n")
		}
		cropLines := regexpMap["crop"].FindAll(stdoutStderr, -1)
		for _, v := range cropLines {
			w, _ := strconv.Atoi(regexpMap["crop"].ReplaceAllString(string(v), "${2}"))
			h, _ := strconv.Atoi(regexpMap["crop"].ReplaceAllString(string(v), "${3}"))
			x, _ := strconv.Atoi(regexpMap["crop"].ReplaceAllString(string(v), "${4}"))
			y, _ := strconv.Atoi(regexpMap["crop"].ReplaceAllString(string(v), "${5}"))
			crop := crop{w, h, x, y}
			cropArrayLocal = append(cropArrayLocal, crop)
		}
		if len(cropArrayLocal) == 0 {
			consolePrint("\x1b[31;1m", "", "\x1b[0m\n")
			return
		}
		crop := cropArrayLocal[0]
		for _, v := range cropArrayLocal {
			if v.w > crop.w || v.h > crop.h {
				crop = v
			}
		}
		consolePrint("\x1b[30;1m", secondsToHHMMSS(strconv.FormatFloat(tempDur, 'f', -1, 64)), " crop=\x1b[0m", crop.w, "\x1b[30;1m:\x1b[0m", crop.h, "\x1b[30;1m:\x1b[0m", crop.x, "\x1b[30;1m:\x1b[0m", crop.y, "\n")
	}
}

type crop struct {
	w int
	h int
	x int
	y int
}

func audioSync(args []string, batchMode bool) (errors []string, input2 string) {
	var input1 string
	// Find two inputs.
	for i := 0; i < len(args); i++ {
		if i+1 < len(args) {
			if (args[i] == "-i") && (input1 == "") {
				input1 = args[i+1]
				continue
			}
			if (args[i] == "-i") && (input1 != "") && (input2 == "") {
				input2 = args[i+1]
				continue
			}
		}
	}
	if input2 == "" {
		consolePrint("\x1b[31;1mERROR: sync mode requires two input files.\x1b[0m\n")
		return
	}
	cmd := exec.Command("ffmpeg", "-i", input1, "-i", input2)
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil && fmt.Sprint(err) != "exit status 1" {
		consolePrint("\x1b[31;1m", err, "\x1b[0m\n")
		return
	}
	durations := regexpMap["durationHHMMSSMS"].FindAll(stdoutStderr, -1)
	if len(durations) < 2 {
		consolePrint("\x1b[31;1mERROR: cannot determine durations for input files.\x1b[0m\n")
		return
	}
	duration1String := regexpMap["durationHHMMSSMS"].ReplaceAllString(string(durations[0]), "${1}")
	duration2String := regexpMap["durationHHMMSSMS"].ReplaceAllString(string(durations[1]), "${1}")
	duration1 := hhmmssmsToSeconds(duration1String)
	duration2 := hhmmssmsToSeconds(duration2String)
	rate := round(48000 * duration2 / duration1)
	if rate == 48000 {
		consolePrint("\x1b[32m" + input1 + "\x1b[0m Duration: " + duration1String + "\n")
		consolePrint("\x1b[32m" + input2 + "\x1b[0m Duration: " + duration2String + "\n")
		consolePrint("\x1b[32;1mAudioSync is not needed.\x1b[0m\n")
		return
	}
	basename := input2[0 : len(input2)-len(filepath.Ext(input2))]
	errors, _ = encodeFile([]string{"-i",
		input2,
		"-af",
		"asetrate=" + strconv.FormatInt(rate, 10) + ",aresample=48000",
		"-vn",
		"-acodec",
		"flac",
		"-compression_level",
		"0",
		"-map_metadata",
		"-1",
		"-map_chapters",
		"-1",
		basename + "_SYNC.flac"}, batchMode, false, false)
	return
}

// "filterMapRange1":  regexp.MustCompile(`\[(\d+)-(\d+):(\d+)\]`),
// "filterMapRange2":  regexp.MustCompile(`\[(\d+):(\d+)-(\d+)\]`),
func convertFilterComplexInputs(in string) (string, error) {
	if regexpMap["filterMapRange1"].MatchString(in) {
		maps := regexpMap["filterMapRange1"].FindAllString(in, -1)
		for _, a := range maps {
			b := regexpMap["filterMapRange1"].FindStringSubmatch(a)

			input1, err := strconv.Atoi(b[1])
			if err != nil {
				return "", err
			}
			input2, err := strconv.Atoi(b[2])
			if err != nil {
				return "", err
			}
			track, err := strconv.Atoi(b[3])
			if err != nil {
				return "", err
			}

			if input1 == input2 {
				continue
			}

			c := ""
			if input1 < input2 {
				for i := input1; i <= input2; i++ {
					c += "[" + strconv.Itoa(i) + ":" + strconv.Itoa(track) + "]"
				}
				in = strings.ReplaceAll(in, b[0], c)
				continue
			}

			if input1 > input2 {
				for i := input1; i >= input2; i-- {
					c += "[" + strconv.Itoa(i) + ":" + strconv.Itoa(track) + "]"
				}
				in = strings.ReplaceAll(in, b[0], c)
				continue
			}
		}
	}

	if regexpMap["filterMapRange2"].MatchString(in) {
		maps := regexpMap["filterMapRange2"].FindAllString(in, -1)
		for _, a := range maps {
			b := regexpMap["filterMapRange2"].FindStringSubmatch(a)

			input, err := strconv.Atoi(b[1])
			if err != nil {
				return "", err
			}
			track1, err := strconv.Atoi(b[2])
			if err != nil {
				return "", err
			}
			track2, err := strconv.Atoi(b[3])
			if err != nil {
				return "", err
			}

			if track1 == track2 {
				continue
			}

			c := ""
			if track1 < track2 {
				for t := track1; t <= track2; t++ {
					c += "[" + strconv.Itoa(input) + ":" + strconv.Itoa(t) + "]"
				}
				in = strings.ReplaceAll(in, b[0], c)
				continue
			}

			if track1 > track2 {
				for t := track1; t >= track2; t-- {
					c += "[" + strconv.Itoa(input) + ":" + strconv.Itoa(t) + "]"
				}
				in = strings.ReplaceAll(in, b[0], c)
				continue
			}
		}
	}

	if regexpMap["filterMapRange3"].MatchString(in) {
		maps := regexpMap["filterMapRange3"].FindAllString(in, -1)
		for _, a := range maps {
			b := regexpMap["filterMapRange3"].FindStringSubmatch(a)

			input1, err := strconv.Atoi(b[1])
			if err != nil {
				return "", err
			}
			input2, err := strconv.Atoi(b[2])
			if err != nil {
				return "", err
			}
			track1, err := strconv.Atoi(b[3])
			if err != nil {
				return "", err
			}
			track2, err := strconv.Atoi(b[4])
			if err != nil {
				return "", err
			}

			if input1 == input2 && track1 == track2 {
				continue
			}

			c := ""
			if input1 < input2 {
				for i := input1; i <= input2; i++ {
					if track1 < track2 {
						for t := track1; t <= track2; t++ {
							c += "[" + strconv.Itoa(i) + ":" + strconv.Itoa(t) + "]"
						}
						continue
					}
					if track1 > track2 {
						for t := track1; t >= track2; t-- {
							c += "[" + strconv.Itoa(i) + ":" + strconv.Itoa(t) + "]"
						}
						continue
					}
				}
				in = strings.ReplaceAll(in, b[0], c)
				continue
			}

			if input1 > input2 {
				for i := input1; i >= input2; i-- {
					if track1 < track2 {
						for t := track1; t <= track2; t++ {
							c += "[" + strconv.Itoa(i) + ":" + strconv.Itoa(t) + "]"
						}
						continue
					}
					if track1 > track2 {
						for t := track1; t >= track2; t-- {
							c += "[" + strconv.Itoa(i) + ":" + strconv.Itoa(t) + "]"
						}
						continue
					}
				}
				in = strings.ReplaceAll(in, b[0], c)
				continue
			}
		}
	}

	return in, nil
}

// encodeFile starts ffmpeg command with passed arguments in ffCommand []string array.
func encodeFile(ffCommand []string, batchMode, ffmpeg, mute bool) (errorsArray []string, firstInput string) {
	var printCommand, progress, lastLine, lastLineUsed, lastLineFull string
	var warningArray []string
	var duration, prevSecond float64
	var speedArray []float64
	var encodingStarted, encodingFinished, streamMapping, sigint bool
	var startTime time.Time
	var prevUptime time.Duration
	var warningSpam map[string]bool
	warningSpam = make(map[string]bool)

	// Intercept Interrupt signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		sigint = true
	}()

	// Print out the final ffmpeg command and add quotes to arguments that contain spaces.
	printCommand = "\x1b[36;1m> \x1b[30;1m" + "ffmpeg"
	for _, v := range ffCommand {
		if strings.Contains(v, " ") {
			v = "\"" + v + "\""
		}
		printCommand += " " + v
	}
	printCommand += "\x1b[0m\n"
	consolePrint(printCommand)

	// Find the first input.
	for i := 0; i < len(ffCommand); i++ {
		if i+1 < len(ffCommand) {
			if (ffCommand[i] == "-i") && (firstInput == "") {
				firstInput = ffCommand[i+1]
			}
		}
	}

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
	// Pipe ffmpegs stdout to fflite to allow piping of output.
	cmd.Stdout = os.Stdout
	// Start ffmpeg.
	cmd.Start()
	// Buffer all the messages coming from ffmpegs stderr.
	scanner := bufio.NewScanner(stderr)
	// Split the lines on `\r?\n`, '\r', "[y/N]".
	scanner.Split(scanLines)
	// For each line.
	for scanner.Scan() {
		line := scanner.Text()
		if !ffmpeg {
			// Check the state of the program.
			switch {
			case !encodingStarted && regexpMap["streamMapping"].MatchString(line):
				streamMapping = true
			case !encodingStarted && streamMapping && !strings.Contains(line, "->"):
				streamMapping = false
			case !encodingStarted && (regexpMap["encoding"].MatchString(line) || regexpMap["encodingNoSpeed"].MatchString(line)) && regexpMap["currentSecond"].ReplaceAllString(line, "$1") != "00:00:00.00":
				startTime = time.Now()
				prevUptime = time.Since(startTime)
				streamMapping = false
				encodingStarted = true
			case encodingStarted && regexpMap["encodingFinished"].MatchString(line):
				encodingStarted, encodingFinished = parseFinish(line, sigint, progress, lastLine, startTime)
			}
			// Modify the lines using regexp.
			switch {
			case streamMapping:
				line = "\x1b[30;1m  " + line + "\x1b[0m\n"
			case regexpMap["input"].MatchString(line):
				line = parseInput(line)
			case regexpMap["output"].MatchString(line):
				line = parseOutput(line)
			case regexpMap["duration"].MatchString(line):
				line, duration = parseDuration(line)
			case regexpMap["stream"].MatchString(line):
				line = parseStream(line)
			case regexpMap["warnings"].MatchString(line):
				line, warningArray = parseWarnings(line, lastLineFull, warningArray, warningSpam)
			case regexpMap["hide"].MatchString(line):
				line = ""
			case encodingStarted:
				switch {
				case regexpMap["encoding"].MatchString(line):
					line, lastLine, progress, speedArray = parseEncoding(line, lastLineFull, duration, speedArray)
				case regexpMap["encodingNoSpeed"].MatchString(line):
					line, lastLine, progress, speedArray = parseEncodingNoSpeed(line, lastLineFull, duration, startTime, prevUptime, prevSecond, speedArray)
				default:
					line, lastLineUsed, errorsArray = parseEncodingErrors(line, lastLineFull, lastLineUsed, lastLine, errorsArray, progress)
				}
			case regexpMap["errors"].MatchString(line):
				line, errorsArray = parseErrors(line, lastLineFull, batchMode, errorsArray)
			default:
				line = ""
			}
			lastLineFull = line
			if line != "" {
				consolePrint(line)
			}
		} else {
			// If not in ffmpeg mode, don't modify the output.
			consolePrint(line + "\n")
		}
	}
	// Wait for ffmpeg to finish.
	cmd.Wait()
	if !cmd.ProcessState.Success() {
		exitStatus = 1
	}
	// If at least one file was encoded.
	if encodingFinished && !batchMode {
		// Play bell sound.
		bell(mute)
	}
	return
}
