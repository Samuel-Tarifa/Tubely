package media

import (
	"bytes"
	"encoding/json"
	"os/exec"
)

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func GetVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
	stdout := bytes.Buffer{}
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	res := jsonCmd{}
	if err = json.Unmarshal(stdout.Bytes(), &res); err != nil {
		return "", err
	}
	width := res.Streams[0].Width
	height := res.Streams[0].Height
	if abs(width*9-16*height) < 100 {
		return "16:9", nil
	}
	if abs(width*16-9*height) < 100 {
		return "9:16", nil
	}
	return "other", nil
}
