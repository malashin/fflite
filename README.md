# fflite ![version: v0.1.1](https://img.shields.io/badge/version-v0.1.1-green.svg) [![license: GPL v3](https://img.shields.io/badge/license-GPL%20v3-blue.svg)](http://www.gnu.org/licenses/gpl-3.0)
`fflite` is [FFmpeg](https://www.ffmpeg.org/) wrapper for minimalistic progress visualization while keeping the flexability of CLI.

### Apart from less obtrusive CLI output there is added functionality:
* Estemated encoding time and progress percentage is shown during encoding.
* Batch execution if `.txt` file is passed as input file.
* Command presets for less typing.
* BEEP sound at the end of encoding process.
* ANSI escape sequences (colors) are supported in Windows terminals (cmd, PowerShell). [go-ansi](https://github.com/k0kubun/go-ansi)

### Same syntax as [FFmpeg](https://www.ffmpeg.org/):
```
fflite [global_options] {[input_file_options] -i input_file} ... {[output_file_options] output_file} ...
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
