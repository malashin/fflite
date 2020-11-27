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
	"golang.org/x/crypto/ssh/terminal"
)

// Global variables.
var version = "v0.1.61"

var presets = map[string]string{
	`^\@crf(\d+)$`:   "-an -vcodec libx264 -preset medium -crf ${1} -pix_fmt yuv420p -g 0 -map_metadata -1 -map_chapters -1",
	`^\@ac(\d+)$`:    "-vn -acodec ac3 -ab ${1}k -map_metadata -1 -map_chapters -1",
	`^\@flac(\d+)$`:  "-vn -acodec flac -compression_level ${1} -map_metadata -1 -map_chapters -1",
	`^\@alac(\d+)$`:  "-vn -acodec alac -compression_level ${1} -map_metadata -1 -map_chapters -1",
	`^\@nometa$`:     "-map_metadata -1 -map_chapters -1",
	`^\@check(\d+)$`: "-map ${1} -scodec srt -dcodec copy -f null NUL",
	`^\@jpg$`:        "-q:v 0 -pix_fmt rgb24 -map_metadata -1",
	`^\@dcpscale$`:   "-loglevel error -stats -an -vcodec libx264 -preset medium -crf 10 -pix_fmt yuv420p -g 0 -vf scale=1920:-2,pad=1920:1080:0:(oh-ih)/2,setsar=1/1 -map_metadata -1 -map_chapters -1",
	`^\@dcpscale2$`:  "-loglevel error -stats -an -vcodec libx264 -preset medium -crf 10 -pix_fmt yuv420p -g 0 -vf scale=1920:-2,setsar=1/1 -map_metadata -1 -map_chapters -1",
	`^\@dcpcrop$`:    "-loglevel error -stats -an -vcodec libx264 -preset medium -crf 10 -pix_fmt yuv420p -g 0 -vf crop=1920:ih:(iw-1920)/2:0,pad=1920:1080:0:(oh-ih)/2,setsar=1/1 -map_metadata -1 -map_chapters -1",
	`^\@sdpal$`:      "-vf scale=720:576,setsar=64/45,unsharp=3:3:0.3:3:3:0",
}

var regexpMap = map[string]*regexp.Regexp{
	"streamMapping":    regexp.MustCompile(`Stream mapping:`),
	"encodingFinished": regexp.MustCompile(`.*video:.*audio:.*subtitle:.*global headers:.*`),
	"input":            regexp.MustCompile(`Input #(\d+),.*from \'(.*)\'\:`),
	"output":           regexp.MustCompile(`Output #(\d+),.*to \'(.*)\'\:`),
	"duration":         regexp.MustCompile(`.*(Duration.*)`),
	"durationHHMMSSMS": regexp.MustCompile(`.*Duration: (\d{2}\:\d{2}\:\d{2}\.\d{2}).*`),
	"stream":           regexp.MustCompile(`.*Stream #(\d+\:\d+)(.*?)\: (.*)`),
	"handler":          regexp.MustCompile(`.*handler_name\ +\:\ +(.+)`),
	"errors":           regexp.MustCompile(`(.*No such file.*|.*Invalid data.*|.*Unrecognized option.*|.*Option not found.*|.*matches no streams.*|.*not supported.*|.*Invalid argument.*|.*Error.*|.*not exist.*|.*-vf\/-af\/-filter.*|.*No such filter.*|.*does not contain.*|.*Not overwriting - exiting.*|.*denied.*|.*\[y\/N\].*|.*Trailing options were found on the commandline.*|.*unconnected output.*|.*Cannot create the link.*|.*Media type mismatch.*|.*moov atom not found.|.*Cannot find a matching stream.*|.*Unknown encoder.*|.*experimental codecs are not enabled.*|.*Alternatively use the non experimental encoder.*|.*Failed to configure.*|.*do not match the corresponding output.*|.*cannot be used together.*|.*Invalid out channel name.*|.*Protocol not found.*|.*Invalid loglevel.*|\"quiet\"|\"panic\"|\"fatal\"|\"error\"|\"warning\"|\"info\"|\"verbose\"|\"debug\"|\"trace\"|.*Unable to parse.*|.*already exists. Exiting.*|.*unable to load.*|.*\, line \d+\).*|.*error.*|.*Too many inputs specified.*|.*Import: couldn't open.*|.*failed.*|.*Invalid duration specification.*)`),
	"warnings":         regexp.MustCompile(`(.*Warning:.*|.*Past duration.*too large.*|.*Starting second pass.*|.*At least one output file must be specified.*|.*fontselect:.*|.*Bitrate .* is extremely low, maybe you mean.*|.*parameter is set too low.*|.*Opening.*for reading.*|.*No channel layout for.*|.*Invalid.*index.*|.*EOF timestamp not reliable.*|.*Expected number.*but found.*|.*is not an encoding option*)`),

	// "encoding":         regexp.MustCompile(`.*(time=.*) bitrate=.*(?:\/s|N\/A)(?: |.*)(dup=.*)* *(speed=.*x) *`),
	// "encodingNoSpeed":  regexp.MustCompile(`.*(time=.*) bitrate=.*(?:\/s|N\/A)(?: |.*)(dup=.*)* *`),
	"encoding":        regexp.MustCompile(`.*(time=.*) (bitrate=.*(?:\/s|N\/A))(?: |.*)(dup=.*)* *(speed=.*x) *`),
	"encodingNoSpeed": regexp.MustCompile(`.*(time=.*) (bitrate=.*(?:\/s|N\/A))(?: |.*)(dup=.*)* *`),

	"timeSpeed":       regexp.MustCompile(`.*time=.*?(\d{2}\:\d{2}\:\d{2}\.\d{2}).* speed=.*?(\d+\.\d+|\d+)x`),
	"currentSecond":   regexp.MustCompile(`.*size=.* time=.*?(\d{2}\:\d{2}\:\d{2}\.\d{2}).*`),
	"hide":            regexp.MustCompile(`(.*Press \[q\] to stop.*|.*Last message repeated.*)`),
	"crop":            regexp.MustCompile(`.*cropdetect.*(crop=(-?\d+):(-?\d+):(-?\d+):(-?\d+)).*`),
	"cropMode":        regexp.MustCompile(`crop(.*)`),
	"fileNameReplace": regexp.MustCompile(`^(?:(.*)(?:\?))?(.*)\:\:(.*)$`),
	"filterMapRange1": regexp.MustCompile(`\[(\d+)-(\d+):(\d+)\]`),
	"filterMapRange2": regexp.MustCompile(`\[(\d+):(\d+)-(\d+)\]`),
	"filterMapRange3": regexp.MustCompile(`\[(\d+)-(\d+):(\d+)-(\d+)\]`),
}

var singlekeys = []string{"-L", "-version", "-buildconf", "-formats", "-muxers", "-demuxers", "-devices", "-codecs", "-decoders", "-encoders", "-bsfs", "-protocols", "-filters", "-pix_fmts", "-layouts", "-sample_fmts", "-colors", "-hwaccels", "-report", "-y", "-n", "-ignore_unknown", "-filter_threads", "-filter_complex_threads", "-stats", "-copy_unknown", "-benchmark", "-benchmark_all", "-stdin", "-dump", "-hex", "-vsync", "-frame_drop_threshold", "-async", "-copyts", "-start_at_zero", "-debug_ts", "-intra", "-sameq", "-same_quant", "-deinterlace", "-psnr", "-vstats", "-vstats_version", "-qphist", "-hwaccel_lax_profile_check", "-isync", "-override_ffserver", "-seek_timestamp", "-apad", "-reinit_filter", "-discard", "-disposition", "-accurate_seek", "-re", "-shortest", "-copyinkf", "-copypriorss", "-thread_queue_size", "-find_stream_info", "-autorotate", "-vn", "-dn", "-intra", "-sameq", "-same_quant", "-deinterlace", "-psnr", "-vstats", "-vstats_version", "-qphist", "-force_fps", "-an", "-guess_layout_max", "-sn", "-fix_sub_duration"}

var hideHandlers = []string{
	"VideoHandler",
	"SoundHandler",
	"DataHandler",
	"Apple Video Media Handler",
	"Apple Sound Media Handler",
	"Apple Alias Data Handler",
	"Time Code Media Handler",
	"Core Media Video",
	"Core Media Audio",
	"Core Media Time Code",
}

var isTerminal = true
var exitStatus = 0

func main() {
	// Main variables.
	var batchInputName, firstInput string
	var errors, errorsArray []string
	var sigint, ffmpeg, nologs, cwdlogs, crop, sync, mute, isBatchInputFile bool
	var cropDetectNumber int
	var cropDetectLimit float64

	cwd, err := os.Getwd()
	if err != nil {
		consolePrint("\x1b[31;1os.Getwd(): " + err.Error() + "\x1b[0m\n")
		os.Exit(1)
	}

	// Intercept interrupt signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		sigint = true
	}()

	// Check if programs output is terminal.
	if !terminal.IsTerminal(int(os.Stdout.Fd())) {
		isTerminal = false
	}

	// Convert passed arguments into array.
	args := os.Args[1:]
	// If program is executed without arguments.
	if len(args) < 1 {
		// Show usage information.
		help()
		os.Exit(0)
	}

	ffmpeg, nologs, cwdlogs, crop, cropDetectNumber, cropDetectLimit, sync, mute, args = parseOptions(args)

	// Create slice containing arguments of ffmpeg command.
	ffCommand := []string{}

	// Parse all arguments and apply presets if needed.
	for i := 0; i < len(args); i++ {
		if i+1 < len(args) {
			if (args[i] == "-i") && (firstInput == "") {
				firstInput = args[i+1]
			}

			if (args[i] == "-i") && (strings.HasSuffix(args[i+1], ".txt")) {
				if batchInputName == "" {
					batchInputName = args[i+1]
					isBatchInputFile = true
				} else {
					consolePrint("\x1b[31;1mOnly one .txt file or glob pattern is allowed for batch execution.\x1b[0m\n")
					os.Exit(1)
				}
			} else if (args[i] == "-i") && (strings.ContainsAny(args[i+1], "*?[")) && !(strings.Contains(args[i+1], "://")) {
				// If file with that name exists, it is not a glob pattern.
				if _, err := os.Stat(args[i+1]); err == nil {
					ffCommand = append(ffCommand, argsPreset(args[i])...)
					continue
				}
				if batchInputName == "" {
					batchInputName = args[i+1]
					isBatchInputFile = false
				} else {
					consolePrint("\x1b[31;1mOnly one .txt file or glob pattern is allowed for batch execution.\x1b[0m\n")
					os.Exit(1)
				}
			} else if (args[i] == "-i") && (strings.HasPrefix(args[i+1], "list:")) {
				batchInputName = args[i+1]
				isBatchInputFile = false
			}

			// Convert -filter_complex inputs from [0-1:1] to [0:1][1:1] or [0:0-1] to [0:0][0:1] or [0-1:2-3] to [0:2][0:3][1:2][1:3].
			if args[i] == "-filter_complex" {
				f, err := convertFilterComplexInputs(args[i+1])
				if err != nil {
					consolePrint("\x1b[31;1convertFilterComplexInputs: " + err.Error() + "\x1b[0m\n")
					os.Exit(1)
				}
				args[i+1] = f
			}
		}
		ffCommand = append(ffCommand, argsPreset(args[i])...)
	}

	// If .txt file or glob pattern is passed as input start batch process.
	// Input will be replaced with each line from that file.
	if batchInputName != "" {
		// Get index of batch file.
		batchInputIndex := stringIndexInSlice(ffCommand, batchInputName)
		batchArray, err := sliceFromFileOrGlob(batchInputName, isBatchInputFile)
		if err != nil {
			consolePrint("\x1b[31;1m", err, "\x1b[0m\n")
			os.Exit(1)
		}
		batchArrayLength := len(batchArray)
		if batchArrayLength < 1 {
			if isBatchInputFile {
				consolePrint("\x1b[31;1mERROR: \"" + batchInputName + "\" is empty.\x1b[0m\n")
			} else {
				consolePrint("\x1b[31;1mERROR: No files matching \"" + batchInputName + "\" pattern.\x1b[0m\n")
			}
			os.Exit(1)
		}
		if !isBatchInputFile {
			consolePrint("\x1b[30;1mINPUT(", batchArrayLength, "): ", strings.Join(batchArray, ", "), "\x1b[0m\n")
		}
		// For each file.
		for i, file := range batchArray {
			filename := ""
			firstInput = ""
			if !sigint {
				// Strip extension.
				basename := file[0 : len(file)-len(filepath.Ext(file))]
				batchCommand := make([]string, len(ffCommand), (cap(ffCommand)+1)*2)
				copy(batchCommand, ffCommand)
				// Replace batch input file with filename.
				batchCommand[batchInputIndex] = file
				// Iterate over all arguments.
				for i := 0; i < len(batchCommand); i++ {
					if i+1 < len(batchCommand) {
						// For each input filename except the first one.
						if (batchCommand[i] == "-i") && (firstInput != "") && (regexpMap["fileNameReplace"].MatchString(batchCommand[i+1])) {
							// Replace input filename if it contains "[prefix?]old::new" pattern.
							match := regexpMap["fileNameReplace"].FindStringSubmatch(batchCommand[i+1])
							batchCommand[i+1] = match[1] + strings.Replace(firstInput, match[2], match[3], -1)
						}
						if (batchCommand[i] == "-i") && (firstInput == "") {
							firstInput = batchCommand[i+1]
						}
					}
					// For each output filename.
					if !(strings.HasPrefix(batchCommand[i], "-")) && (batchCommand[i] != "NUL") && (!(strings.HasPrefix(batchCommand[i-1], "-")) || batchCommand[i-1] == "-1" || contains(singlekeys, batchCommand[i-1])) {
						// Replace filename if it contains "[prefix?]old::new" pattern, append the output to input otherwise.
						if regexpMap["fileNameReplace"].MatchString(batchCommand[i]) {
							match := regexpMap["fileNameReplace"].FindStringSubmatch(batchCommand[i])
							// consolePrint("\nDEBUG:", match, "\n")
							batchCommand[i] = match[1] + strings.Replace(filepath.Base(firstInput), match[2], match[3], -1)
						} else {
							batchCommand[i] = basename + "_" + batchCommand[i]
						}
					}
				}
				consolePrint("\n\x1b[42;1mINPUT " + strconv.FormatInt(int64(i)+1, 10) + " of " + strconv.FormatInt(int64(batchArrayLength), 10) + "\x1b[0m\n")
				switch {
				// Run cropDetect if crop mode is enabled.
				case crop:
					cropDetect(firstInput, cropDetectNumber, cropDetectLimit)
					continue
				// Run audioSync if sync mode is enabled.
				case sync:
					errors, filename = audioSync(batchCommand, true)
				default:
					errors, filename = encodeFile(batchCommand, true, ffmpeg, mute)
				}
				// Append errors to errorsArray.
				if len(errors) > 0 {
					if len(errorsArray) != 0 {
						errorsArray = append(errorsArray, "\n")
					}
					errorsArray = append(errorsArray, "\x1b[42;1mINPUT "+strconv.FormatInt(int64(i)+1, 10)+":\x1b[0m\x1b[32;1m "+filename+"\x1b[0m\n")
					errorsArray = append(errorsArray, errors...)

					logpath := firstInput + ".#err"
					if cwdlogs {
						logpath = filepath.Join(cwd, filepath.Base(firstInput)) + ".#err"
					}

					if nologs {
						continue
					}

					writeStringArrayToFile(logpath, []string{"INPUT: " + filename + "\n"}, 0775)
					writeStringArrayToFile(logpath, errors, 0775)
				}
			}
		}
		// Play bell sound.
		bell(mute)
	} else {
		filename := ""
		firstInput = ""
		// For each output filename.
		for i := 0; i < len(ffCommand); i++ {
			if i+1 < len(ffCommand) {
				// For each input filename except the first one.
				if (ffCommand[i] == "-i") && (firstInput != "") && (regexpMap["fileNameReplace"].MatchString(ffCommand[i+1])) {
					// Replace input filename if it contains "[prefix?]old::new" pattern.
					match := regexpMap["fileNameReplace"].FindStringSubmatch(ffCommand[i+1])
					ffCommand[i+1] = match[1] + strings.Replace(firstInput, match[2], match[3], -1)
				}
				if (ffCommand[i] == "-i") && (firstInput == "") {
					firstInput = ffCommand[i+1]
				}
			}
			if i > 0 {
				if !(strings.HasPrefix(ffCommand[i], "-")) && (ffCommand[i] != "NUL") && (!(strings.HasPrefix(ffCommand[i-1], "-")) || ffCommand[i-1] == "-1") && (regexpMap["fileNameReplace"].MatchString(ffCommand[i])) {
					// Replace output filename if it contains "[prefix?]old::new" pattern.
					match := regexpMap["fileNameReplace"].FindStringSubmatch(ffCommand[i])
					ffCommand[i] = match[1] + strings.Replace(firstInput, match[2], match[3], -1)
				}
			}
		}
		switch {
		// Run cropDetect if crop mode is enabled.
		case crop:
			cropDetect(firstInput, cropDetectNumber, cropDetectLimit)
			return
		// Run audioSync if sync mode is enabled.
		case sync:
			errors, filename = audioSync(ffCommand, false)
		default:
			errors, filename = encodeFile(ffCommand, false, ffmpeg, mute)
		}
		// Append errors to errorsArray.
		if len(errors) > 0 {
			errorsArray = append(errorsArray, "\x1b[42;1mINPUT:\x1b[0m\x1b[32;1m "+filename+"\x1b[0m\n")
			errorsArray = append(errorsArray, errors...)
			if nologs {
				return
			}

			logpath := firstInput + ".#err"
			if cwdlogs {
				logpath = filepath.Join(cwd, filepath.Base(firstInput)) + ".#err"
			}

			if nologs {
				return
			}

			writeStringArrayToFile(logpath, errorsArray, 0775)
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
	os.Exit(exitStatus)
}
