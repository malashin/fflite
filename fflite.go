package main

import (
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	ansi "github.com/k0kubun/go-ansi"
)

// Global variables.
var version = "v0.1.5"
var speedArray []float64
var presets = map[string]string{
	`^\@crf(\d+)$`: "-an -vcodec libx264 -preset medium -crf ${1} -pix_fmt yuv420p -g 0 -map_metadata -1 -map_chapters -1",
	`^\@ac(\d+)$`:  "-vn -acodec ac3 -ab ${1}k -map_metadata -1 -map_chapters -1",
	`^\@nometa$`:   "-map_metadata -1 -map_chapters -1",
	`^\@check$`:    "-f null NUL",
}

func main() {
	// Main variables.
	var lastArgs, batchInputName, basename string
	var errorsArray, errors []string
	var batchInputIndex int
	var sigint, appendArgs, ffmpeg = false, false, false
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
	// If 1st argument is "ffmpeg" run the same command in ffmpeg instead of fflite.
	if args[0] == "ffmpeg" {
		ffmpeg = true
		args = args[1:]
	}
	// Create slice containing arguments of ffmpeg command.
	// Use "-hide_banner" as default.
	ffCommand := []string{"-hide_banner"}
	// Parse all arguments and apply presets if needed.
	// Arguments surrounded by escaped doublequotes are joined.
	for i := 0; i < len(args); i++ {
		if (len(args) > 2) && (args[i] == "-i") && (strings.HasSuffix(args[i+1], ".txt")) {
			if batchInputName == "" {
				batchInputName = args[i+1]
			} else {
				consolePrint("\x1b[31;1mOnly one .txt file is allowed for batch execution.\x1b[0m\n")
				os.Exit(1)
			}
		}
		if !appendArgs {
			if (args[i][0:1] == "\"") && !(args[i][len(args[i])-1:] == "\"") {
				lastArgs += args[i]
				appendArgs = true
			} else if (args[i][0:1] == "\"") && (args[i][len(args[i])-1:] == "\"") {
				ffCommand = append(ffCommand, argsPreset(strings.Replace(args[i], "\"", "", -1))...)
			} else {
				ffCommand = append(ffCommand, argsPreset(args[i])...)
			}
		} else {
			if args[i][len(args[i])-1:] == "\"" {
				lastArgs = lastArgs + " " + args[i]
				ffCommand = append(ffCommand, strings.Replace(lastArgs, "\"", "", -1))
				appendArgs = false
			} else {
				lastArgs = lastArgs + " " + args[i]
			}
		}
	}
	// If .txt file is passed as input start batch process.
	// .txt input will be replaced with each line from that file.
	if batchInputName != "" {
		// Get index of batch file.
		batchInputIndex = stringIndexInSlice(ffCommand, batchInputName)
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
			// For each file.
			for i, file := range batchArray {
				if !sigint {
					// Strip extension.
					basename = file[0 : len(file)-len(filepath.Ext(file))]
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
					errors = encodeFile(batchCommand, true, ffmpeg)
					// Append errors to errorsArray.
					if len(errors) > 0 {
						if len(errorsArray) != 0 {
							errorsArray = append(errorsArray, "\n")
						}
						errorsArray = append(errorsArray, "\x1b[42;1mINPUT "+strconv.FormatInt(int64(i)+1, 10)+":\x1b[0m\x1b[32;1m "+file+"\x1b[0m\n")
						errorsArray = append(errorsArray, errors...)
					}
					// Reset the speedArray and errors.
					speedArray = []float64{}
					errors = []string{}
				}
			}
			// Play bell sound.
			consolePrint("\x07")
		}
	} else {
		errors := encodeFile(ffCommand, false, ffmpeg)
		errorsArray = append(errorsArray, errors...)
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
