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
	var progress, eta, lastLine, lastArgs string
	var timeSpeed []string
	var duration, currentSecond, currentSpeed, prevSecond float64
	var encodingStarted, encodingFinished, streamMapping, sigint, errors, appendArgs = false, false, false, false, false, false
	var r *regexp.Regexp
	var startTime time.Time
	var prevUptime time.Duration
	var currentUptime time.Duration

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		sigint = true
	}()

	// Convert passed arguments into array.
	args := os.Args
	// If program is executed without arguments.
	// Show usage information.
	if len(args) <= 1 {
		ansi.Print("\nfflite is FFmpeg wrapper for minimalistic progress visualization while keeping the flexability of CLI.\n\n")
		ansi.Print("\x1b[33;1mUsage:\x1b[0m\nfflite [global_options] {[input_file_options] -i input_file} ... {[output_file_options] output_file} ...\n")
		ansi.Print("In order to pass arguments with spaces in it, surround them with escaped doublequotes \\\"input file\\\".\n\n")
		ansi.Print("\x1b[33;1mFFmpeg documentation:\x1b[0m\nwww.ffmpeg.org/ffmpeg-all.html\n\n")
		ansi.Print("\x1b[33;1mGithub page:\x1b[0m\ngithub.com/malashin/fflite\n")
		os.Exit(0)
	}
	// Use "-hide_banner" as default.
	ffCommand := []string{"-hide_banner"}
	// Parse all arguments and apply presets if needed.
	// Arguments surrounded by escaped doublequotes are joined.
	for i := 1; i < len(args); i++ {
		if !appendArgs {
			if args[i][0:1] == "\"" {
				lastArgs += args[i]
				appendArgs = true
				continue
			} else {
				ffCommand = append(ffCommand, argsPreset(args[i])...)
			}
		} else {
			if args[i][len(args[i])-1:] == "\"" {
				lastArgs = lastArgs + " " + args[i]
				lastArgs = strings.Replace(lastArgs, "\"", "", -1)
				ffCommand = append(ffCommand, lastArgs)
				appendArgs = false
			} else {
				lastArgs = lastArgs + " " + args[i]
			}
		}
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
