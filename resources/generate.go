package main

import (
	"os"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/tsujio/game-util/resourceutil"
)

func main() {
	resourceutil.ForceSaveDecodedAudio(os.Args[1], audio.NewContext(48000))
}
