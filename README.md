# fflite ![version: v0.1.49](https://img.shields.io/badge/version-v0.1.49-green.svg) [![license: GPL v3](https://img.shields.io/badge/license-GPL%20v3-blue.svg)](http://www.gnu.org/licenses/gpl-3.0)
`fflite` is [FFmpeg](https://www.ffmpeg.org/) wrapper for minimalistic progress visualization while keeping the flexibility of CLI.

### Apart from less obtrusive CLI output there is added functionality:
* Estimated encoding time and progress percentage is shown during encoding.
* Batch execution if `.txt` filelist, `"list:file1 file2 \"file 3\""` or a glob pattern is passed as input file, only one is allowed (`fflite -i *.mp4`).
* Once the first input file is specified input and output files can be named using `[prefix?]old::new` pattern. This will take the first input name and replace `old` string with the `new` string. If `?` is present, everything before `?` will be used as a prefix for new filenames (`fflite -i film_video.mp4 -i folder?video.mp4::audio.ac3`).
* Command presets for less typing.
* Error logging.
* Crop detection mode (`fflite crop[crop_number:crop_limit] -i input_file`). If `fflite crop[digit]` is passed it will be treated as `crop_limit` if digit is less then one, `crop_number` otherwise.
* BEEP sound at the end of encoding process.
* ANSI escape sequences (colors) are supported in Windows terminals (cmd, PowerShell). [go-ansi](https://github.com/k0kubun/go-ansi)

### Same syntax as [FFmpeg](https://www.ffmpeg.org/):
```
fflite [fflite_option] [global_options] {[input_file_options] -i input_file} ... {[output_file_options] output_file} ...
```
[FFmpeg documentation](https://www.ffmpeg.org/ffmpeg-all.html)

*It is currently made for personal use and some settings, like presets, are still hardcoded.*

## Installation
```
go get -u github.com/malashin/fflite
```
* `$GOPATH/bin` must be added to your $PATH environment variable.
* You need to have [FFmpeg](https://www.ffmpeg.org/) installed and accessable from $PATH environment variable.

## Sample output of `fflite`:
![fflite](http://i.imgur.com/bz0b0Xp.png)

## Same file in [FFmpeg](https://www.ffmpeg.org/)
[![ffmpeg](http://i.imgur.com/VJ8Wj48l.png)](http://i.imgur.com/VJ8Wj48.png)
