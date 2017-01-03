package main

import (
	"bufio"
	"log"
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

// Global variables.
var speedArray []float64

func main() {
	// Main variables.
	var progress, eta, lastLine string
	var timeSpeed []string
	var duration, currentSecond, currentSpeed float64
	var encodingStarted, encodingFinished, streamMapping, sigint, errors = false, false, false, false, false
	var r *regexp.Regexp
	var startTime time.Time

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		sigint = true
	}()

	// Convert passed arguments into array.
	args := os.Args
	// Use "-hide_banner" as default.
	ffCommand := []string{"-hide_banner"}
	for i := 1; i < len(args); i++ {
		// Parse all arguments and apply presets if needed.
		ffCommand = append(ffCommand, argsPreset(args[i])...)
	}
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
		} else if r = regexp.MustCompile(`.* (time=.*) bitrate=.*\/s(.*speed=.*)`); r.MatchString(line) {
			timeSpeed = strings.Split(regexp.MustCompile(`.* time=.*?(\d{2}\:\d{2}\:\d{2}\.\d{2}).* speed=.*?(\d+\.\d+|\d+)x`).ReplaceAllString(line, "$1 $2"), " ")
			currentSecond = hhmmssmsToSeconds(timeSpeed[0])
			currentSpeed, _ = strconv.ParseFloat(timeSpeed[1], 64)
			progress = truncPad(strconv.FormatInt(int64(currentSecond/(duration/100.0)), 10), 3, 'r')
			eta = secondsToHHMMSS(getETA(currentSpeed, duration, currentSecond))
			lastLine = r.ReplaceAllString(line, strings.TrimSpace("${1}"+"${2}"))
			line = r.ReplaceAllString(line, "\x1b[33;1m"+progress+"%\x1b[0m eta="+eta+" "+strings.TrimSpace("${1}"+"${2}")+"\r")
		} else if r = regexp.MustCompile(`.*Press \[q\] to stop.*`); r.MatchString(line) {
			line = ""
		} else if encodingStarted {
			if !errors {
				ansi.Print("\n")
			}
			errors = true
			ansi.Printf("\x1b[31;1m" + line + "\x1b[0m\n")
			continue
		} else {
			line = ""
		}
		errors = false
		ansi.Print(line)
	}

	// If at least one file was encoded.
	if encodingFinished {
		// Play bell sound before exiting.
		ansi.Print("\x07")
	}
}
