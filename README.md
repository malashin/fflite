# fflite [![License: GPL v3](https://img.shields.io/badge/License-GPL%20v3-blue.svg)](http://www.gnu.org/licenses/gpl-3.0)
`fflite` is [FFmpeg](https://www.ffmpeg.org/) wrapper for minimalistic progress visualization while keeping the flexability of CLI.

### Apart from less obtrusive CLI output there is added functionality:
* Estemated encoding time and progress percentage is shown during encoding.
* ANSI escape sequences (colors) are supported in Windows terminals (cmd, PowerShell). [go-ansi](https://github.com/k0kubun/go-ansi)
* Command presets for less typing.
* BEEP sound at the end of encoding process.

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
