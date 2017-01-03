# fflite
`fflite` is [FFmpeg](https://www.ffmpeg.org/) wrapper for minimalistic progress visualization while keeping the flexability of CLI use.

## Apart from less obtrusive CLI output there is added functionality:
* Estemated encoding time and progress percentage is shown during encoding.
* ANSI escape sequences (colors) are supported in Windows terminals (cmd, PowerShell). [go-ansi](https://github.com/k0kubun/go-ansi)
* Command presets for less typing.

*It is currently made for personal use and some settings are still hardcoded.*

## Sample output of `fflite`:
![fflite](http://i.imgur.com/54BQEfT.png)

## Same file in [FFmpeg](https://www.ffmpeg.org/)
[![ffmpeg](http://i.imgur.com/VJ8Wj48l.png)](http://i.imgur.com/VJ8Wj48.png)
