package main

import (
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	ansi "github.com/k0kubun/go-ansi"
)

// Global variables.
var version = "v0.1.27"
var presets = map[string]string{
	`^\@crf(\d+)$`:  "-an -vcodec libx264 -preset medium -crf ${1} -pix_fmt yuv420p -g 0 -map_metadata -1 -map_chapters -1",
	`^\@ac(\d+)$`:   "-vn -acodec ac3 -ab ${1}k -map_metadata -1 -map_chapters -1",
	`^\@flac(\d+)$`: "-vn -acodec flac -compression_level ${1} -map_metadata -1 -map_chapters -1",
	`^\@nometa$`:    "-map_metadata -1 -map_chapters -1",
	`^\@check$`:     "-f null NUL",
	`^\@jpg$`:       "-q:v 0 -pix_fmt rgb24 -map_metadata -1",
	`^\@dcpscale$`:  "-loglevel error -stats -an -vcodec libx264 -preset medium -crf 13 -pix_fmt yuv420p -g 0 -vf scale=1920:trunc(ih/(iw/1920)),pad=1920:1080:0:(oh-ih)/2,setsar=1/1 -map_metadata -1 -map_chapters -1",
	`^\@dcpcrop$`:   "-loglevel error -stats -an -vcodec libx264 -preset medium -crf 13 -pix_fmt yuv420p -g 0 -vf crop=1920:ih:(iw-1920)/2:0,pad=1920:1080:0:(oh-ih)/2,setsar=1/1 -map_metadata -1 -map_chapters -1",
	`^\@sdpal$`:     "-vf scale=720:576,setsar=64/45,unsharp=3:3:0.3:3:3:0",
}
var regexpMap = map[string]*regexp.Regexp{
	"streamMapping":         regexp.MustCompile(`Stream mapping:`),
	"streamMappingFinished": regexp.MustCompile(`.*Press \[q\] to stop.*`),
	"encodingFinished":      regexp.MustCompile(`.*video:.*audio:.*subtitle:.*global headers:.*`),
	"input":                 regexp.MustCompile(`Input #(\d+),.*from \'(.*)\'\:`),
	"output":                regexp.MustCompile(`Output #(\d+),.*to \'(.*)\'\:`),
	"duration":              regexp.MustCompile(`.*(Duration.*)`),
	"durationHHMMSSMS":      regexp.MustCompile(`.*Duration: (\d{2}\:\d{2}\:\d{2}\.\d{2}).*`),
	"stream":                regexp.MustCompile(`.*Stream #(\d+\:\d+)(.*?):(.*)`),
	"errors":                regexp.MustCompile(`(.*No such file.*|.*Invalid data.*|.*At least one output file must be specified.*|.*Unrecognized option.*|.*Option not found.*|.*matches no streams.*|.*not supported.*|.*Invalid argument.*|.*Error.*|.*not exist.*|.*-vf\/-af\/-filter.*|.*No such filter.*|.*does not contain.*|.*Not overwriting - exiting.*|.*denied.*|.*\[y\/N\].*|.*Trailing options were found on the commandline.*|.*unconnected output.*|.*Cannot create the link.*|.*Media type mismatch.*|.*moov atom not found.|.*Cannot find a matching stream.*|.*Unknown encoder.*|.*experimental codecs are not enabled.*|.*Alternatively use the non experimental encoder.*|.*Failed to configure.*|.*do not match the corresponding output.*|.*cannot be used together.*|.*Invalid out channel name.*)`),
	"warnings":              regexp.MustCompile(`(.*Warning:.*|.*Past duration.*too large.*|.*Starting second pass.*)`),
	"encoding":              regexp.MustCompile(`.* (time=.*) bitrate=.*(?:\/s|N\/A)(?: |.*)(dup=.*speed=.*|speed=.*)`),
	"encodingNoSpeed":       regexp.MustCompile(`.* (time=.*) bitrate=.*(\/s|N\/A)(.*)`),
	"timeSpeed":             regexp.MustCompile(`.* time=.*?(\d{2}\:\d{2}\:\d{2}\.\d{2}).* speed=.*?(\d+\.\d+|\d+)x`),
	"currentSecond":         regexp.MustCompile(`.*size=.* time=.*?(\d{2}\:\d{2}\:\d{2}\.\d{2}).*`),
	"hide":                  regexp.MustCompile(`(.*Press \[q\] to stop.*|.*Last message repeated.*)`),
	"crop":                  regexp.MustCompile(`.*cropdetect.*(crop=(\d+):(\d+):(\d+):(\d+)).*`),
}

func main() {
	// Main variables.
	var batchInputName, firstInput string
	var errorsArray []string
	var sigint, ffmpeg, nologs, crop bool
	// Intercept interrupt signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		sigint = true
	}()
	// Convert passed arguments into array.
	args := os.Args[1:]
	// If program is executed without arguments.
	if len(args) < 1 {
		// Show usage information.
		help()
		os.Exit(0)
	}
	ffmpeg, nologs, crop, args = parseOptions(args)
	// Create slice containing arguments of ffmpeg command.
	ffCommand := []string{}
	// Parse all arguments and apply presets if needed.
	// Arguments surrounded by escaped doublequotes are joined.
	for i := 0; i < len(args); i++ {
		if i+1 < len(args) {
			if (args[i] == "-i") && (strings.HasSuffix(args[i+1], ".txt")) {
				if batchInputName == "" {
					batchInputName = args[i+1]
				} else {
					consolePrint("\x1b[31;1mOnly one .txt file is allowed for batch execution.\x1b[0m\n")
					os.Exit(1)
				}
			}
			if (args[i] == "-i") && (firstInput == "") {
				firstInput = args[i+1]
				if crop {
					cropDetect(firstInput)
					os.Exit(1)
				}
			}
		}
		ffCommand = append(ffCommand, argsPreset(args[i])...)
	}
	// If .txt file is passed as input start batch process.
	// .txt input will be replaced with each line from that file.
	if batchInputName != "" {
		// Get index of batch file.
		batchInputIndex := stringIndexInSlice(ffCommand, batchInputName)
		if batchInputIndex != -1 {
			// Create array of files from batch file.
			batchArray, err := readLines(batchInputName)
			if err != nil {
				consolePrint("\x1b[31;1m")
				consolePrint(err)
				consolePrint("\x1b[0m\n")
				os.Exit(1)
			}
			batchArrayLength := len(batchArray)
			if batchArrayLength < 1 {
				consolePrint("\x1b[31;1mERROR: \"" + batchInputName + "\" is empty.\x1b[0m\n")
				os.Exit(1)
			}
			// For each file.
			for i, file := range batchArray {
				if !sigint {
					// Strip extension.
					basename := file[0 : len(file)-len(filepath.Ext(file))]
					batchCommand := make([]string, len(ffCommand), (cap(ffCommand)+1)*2)
					copy(batchCommand, ffCommand)
					// Append basename to each output file.
					for i := 1; i < len(batchCommand); i++ {
						if !(strings.HasPrefix(batchCommand[i], "-")) && (!(strings.HasPrefix(batchCommand[i-1], "-")) || batchCommand[i-1] == "-1") {
							batchCommand[i] = basename + "_" + batchCommand[i]
						}
					}
					// Replace batch input file with filename.
					batchCommand[batchInputIndex] = file
					consolePrint("\n\x1b[42;1mINPUT " + strconv.FormatInt(int64(i)+1, 10) + " of " + strconv.FormatInt(int64(batchArrayLength), 10) + "\x1b[0m\n")
					errors := encodeFile(batchCommand, true, ffmpeg)
					// Append errors to errorsArray.
					if len(errors) > 0 {
						if len(errorsArray) != 0 {
							errorsArray = append(errorsArray, "\n")
						}
						errorsArray = append(errorsArray, "\x1b[42;1mINPUT "+strconv.FormatInt(int64(i)+1, 10)+":\x1b[0m\x1b[32;1m "+file+"\x1b[0m\n")
						errorsArray = append(errorsArray, errors...)
						if !nologs {
							writeStringArrayToFile(file+".#err", []string{"INPUT: " + file + "\n"}, 0775)
							writeStringArrayToFile(file+".#err", errors, 0775)
						}
					}
				}
			}
			// Play bell sound.
			consolePrint("\x07")
		}
	} else {
		errors := encodeFile(ffCommand, false, ffmpeg)
		// Append errors to errorsArray.
		if len(errors) > 0 {
			errorsArray = append(errorsArray, "\x1b[42;1mINPUT:\x1b[0m\x1b[32;1m "+firstInput+"\x1b[0m\n")
			errorsArray = append(errorsArray, errors...)
			if !nologs {
				writeStringArrayToFile(firstInput+".#err", errorsArray, 0775)
			}
		}
	}

	// Print out all errors.
	if len(errorsArray) > 0 {
		consolePrint("\n\x1b[41;1mERROR LOG:\x1b[0m\n")
		for _, v := range errorsArray {
			consolePrint(v)
		}
	}

	// Show cursor in case its hidden before exit.
	ansi.CursorShow()
}
