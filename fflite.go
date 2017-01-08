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
var version = "v0.1.0"
var speedArray []float64

func main() {
	// Main variables.
	var lastArgs, batchInputName, basename string
	var errorsArray, errors []string
	var batchInputIndex int
	var sigint, appendArgs = false, false
	// Intercept interrupt signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		sigint = true
	}()
	// Convert passed arguments into array.
	args := os.Args
	// If program is executed without arguments.
	if len(args) <= 1 {
		// Show usage information.
		help()
		os.Exit(0)
	}
	// Create slice containing arguments of ffmpeg command.
	// Use "-hide_banner" as default.
	ffCommand := []string{"-hide_banner"}
	// Parse all arguments and apply presets if needed.
	// Arguments surrounded by escaped doublequotes are joined.
	for i := 1; i < len(args); i++ {
		if (args[i] == "-i") && (strings.HasSuffix(args[i+1], ".txt")) {
			if batchInputName == "" {
				batchInputName = args[i+1]
			} else {
				ansi.Print("\x1b[31;1mOnly one .txt file is allowed for batch execution.\x1b[0m\n")
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
				ansi.Print("\x1b[31;1m")
				ansi.Print(err)
				ansi.Print("\x1b[0m\n")
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
					ansi.Print("\n\x1b[42;1mINPUT " + strconv.FormatInt(int64(i)+1, 10) + " of " + strconv.FormatInt(int64(batchArrayLength), 10) + "\x1b[0m\n")
					errors = encodeFile(batchCommand, true)
					// Append errors to errorsArray
					if len(errors) > 0 {
						if len(errorsArray) != 0 {
							errorsArray = append(errorsArray, "\n")
						}
						errorsArray = append(errorsArray, "\x1b[42;1mINPUT "+strconv.FormatInt(int64(i)+1, 10)+":\x1b[0m\x1b[32;1m "+file+"\x1b[0m\n")
						errorsArray = append(errorsArray, errors...)
					}
					// Reset the speedArray and errors
					speedArray = []float64{}
					errors = []string{}
				}
			}
			// Play bell sound.
			ansi.Print("\x07")
		}
	} else {
		errors := encodeFile(ffCommand, false)
		errorsArray = append(errorsArray, errors...)
	}

	// Print out all errors
	if len(errorsArray) > 0 {
		ansi.Print("\n\x1b[41;1mERROR LOG:\x1b[0m\n")
		for _, v := range errorsArray {
			ansi.Print(v)
		}
	}
}
