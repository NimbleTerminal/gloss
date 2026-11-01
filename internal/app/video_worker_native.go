//go:build !js

package app

import (
	"os"

	"nimbleterminal.dev/ntmediabox"
	"nimbleterminal.dev/ntvideo"
)

const mediaWorkerArg = "--gloss-media-worker"

func init() {
	if len(os.Args) == 3 && os.Args[1] == mediaWorkerArg {
		err := ntvideo.ServeWorker(ntmediabox.WorkerConn(), os.Args[2], ntvideo.PlayerOptions{
			DisableHardwareDecoding: true,
			DisableAudio:            true,
		})
		if err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
}

var videoStagePath string

func videoWorkerOption() ntvideo.Option {
	exe, _ := os.Executable()
	return ntvideo.WithWorker(ntvideo.WorkerConfig{
		WorkerPath:       exe,
		WorkerArgs:       []string{mediaWorkerArg},
		StagePath:        videoStagePath,
		AllowUnsandboxed: false,
	})
}
