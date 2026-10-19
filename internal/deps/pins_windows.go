package deps

// pins are the exact builds Fetch installs on Windows, checked by
// SHA-256 before anything is installed. Bump a version by replacing the
// URL and the checksum together, both taken from the project's own
// release.
//
// yt-dlp: https://github.com/yt-dlp/yt-dlp/releases (SHA2-256SUMS).
// It updates itself afterwards with -U; the pin is only the first copy.
//
// ffmpeg: gyan.dev's "essentials" release build, from gyan.dev itself so
// the file and its published checksum come from the same place. One zip
// carries both ffmpeg and ffprobe.
var ffmpegPin = pin{
	url:    "https://www.gyan.dev/ffmpeg/builds/packages/ffmpeg-9.0.2-essentials_build.zip",
	sha256: "60f467265b1e312373dbcd92200c2618a74850f98d3d078e94296bb3fa2047ba",
	files: map[string]string{
		"/bin/ffmpeg.exe":  "ffmpeg.exe",
		"/bin/ffprobe.exe": "ffprobe.exe",
	},
}

var pins = map[string]pin{
	"yt-dlp": {
		url:    "https://github.com/yt-dlp/yt-dlp/releases/download/2026.08.19/yt-dlp.exe",
		sha256: "66674953fe251b89f4d08c5f0e35e0728679bd67ab3d7d05c0562af101dd3e7a",
	},
	"ffmpeg":  ffmpegPin,
	"ffprobe": ffmpegPin,
}
