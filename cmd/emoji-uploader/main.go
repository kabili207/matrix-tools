package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	"github.com/kabili207/matrix-tools/pkg/api"
	"github.com/kabili207/matrix-tools/pkg/models"
	"github.com/sapphi-red/midec"
	_ "github.com/sapphi-red/midec/gif"  // import this to detect Animated GIF
	_ "github.com/sapphi-red/midec/png"  // import this to detect APNG
	_ "github.com/sapphi-red/midec/webp" // import this to detect Animated WebP
	"rsc.io/getopt"
)

func pathIsDir(path string) (bool, error) {
	fi, err := os.Stat(path)
	if err == nil {
		return fi.IsDir(), nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func main() {

	serverPtr := flag.String("server", "", "the server URL")
	getopt.Alias("s", "server")

	pathPtr := flag.String("path", "", "path of the emoji to upload")
	getopt.Alias("p", "path")

	roomPtr := flag.String("room", "", "room ID")
	getopt.Alias("r", "room")

	namePtr := flag.String("name", "", "name of the emoji pack")
	getopt.Alias("n", "name")

	authPtr := flag.String("auth", "", "auth token")

	getopt.Parse()

	paramError := false

	if serverPtr == nil || *serverPtr == "" {
		// TODO: Prompt user for server URL instead
		fmt.Fprintln(os.Stderr, "Homeserver URL is required")
		paramError = true
	} else if _, err := url.ParseRequestURI(*serverPtr); err != nil {
		fmt.Fprintln(os.Stderr, "Not a valid homeserver URL\n\tMust be a full URL such as https://example.com:8443")
		paramError = true
	}

	if authPtr == nil || *authPtr == "" {
		if env, ok := os.LookupEnv("SYNAPSE_AUTH_TOKEN"); ok {
			authPtr = &env
		} else {
			// TODO: Prompt user for credentials instead
			fmt.Fprintln(os.Stderr, "Auth token required. Please pass the --auth parameter or set the SYNAPSE_AUTH_TOKEN environment variable")
			paramError = true
		}
	}

	if roomPtr == nil || *roomPtr == "" {
		// TODO: Prompt user for room ID instead
		fmt.Fprintln(os.Stderr, "Room ID is required")
		paramError = true
	} else if !strings.HasPrefix(*roomPtr, "!") {
		fmt.Fprintf(os.Stderr, "Invalid room ID: %s\n", *roomPtr)
		if strings.HasPrefix(*roomPtr, "\\") {
			fmt.Fprintln(os.Stderr, "\tDid you escape the '!' properly? Many shells are very picky about the '!' character")
		}
		paramError = true
	}

	if namePtr == nil || *namePtr == "" {
		// TODO: Prompt user for pack name instead
		fmt.Fprintln(os.Stderr, "Pack name is required")
		paramError = true
	}

	if pathPtr == nil || *pathPtr == "" {
		// TODO: Prompt user for pack name instead
		fmt.Fprintln(os.Stderr, "Path to emoji is required")
		paramError = true
	} else if ok, err := pathIsDir(*pathPtr); !ok || err != nil {
		fmt.Fprintln(os.Stderr, "Invalid path")
		paramError = true
	}

	if paramError {
		return
	}

	mtxClient := api.NewMatrixClient(*serverPtr, *authPtr)

	packId := mtxClient.EncodePackId(*namePtr)

	emojiPack, err := mtxClient.GetEmotePack(*roomPtr, packId)

	if err != nil {
		fmt.Println("Error fetching emote pack:", err)
		return
	}

	var images []string
	t := find(*pathPtr, ".png")
	images = append(images, t...)

	t = find(*pathPtr, ".gif")
	images = append(images, t...)

	if emojiPack.Images == nil {
		emojiPack.Images = map[string]models.PackImage{}
	}

	emojiPack.Pack.DisplayName = *namePtr

	for _, v := range images {
		info, err := getImageInfo(v)
		if err != nil {

			fmt.Fprintf(os.Stderr, "%s: %v\n", info.Path, err)
		}
		relPath, _ := filepath.Rel(*pathPtr, info.Path)

		if relPath == "logo.png" && emojiPack.Pack.AvatarUrl == nil {
			fmt.Printf("Uploading %s...\n", relPath)
			contentUrl, err := mtxClient.UploadFile(info.Name, info.MimeType, info.Bytes)
			if err != nil {
				fmt.Printf("Error uploading %s:\n %s\n", relPath, err)
				return
			}
			emojiPack.Pack.AvatarUrl = &contentUrl
			continue
		}

		emojiName := strings.TrimSuffix(filepath.Base(info.Name), filepath.Ext(info.Name))
		if _, ok := emojiPack.Images[emojiName]; !ok {

			fmt.Printf("Uploading %s...\n", relPath)

			contentUrl, err := mtxClient.UploadFile(info.Name, info.MimeType, info.Bytes)

			if err != nil {
				fmt.Printf("Error uploading %s:\n %s\n", relPath, err)
				return
			}

			usage := "emoticon"
			if info.Width > 128 || info.Height > 128 {
				usage = "sticker"
			}
			size := len(info.Bytes)
			pi := models.PackImage{
				Url:   contentUrl,
				Usage: []string{usage},
				Info: &models.PackImageInfo{
					Width:    &info.Width,
					Height:   &info.Height,
					Size:     &size,
					MimeType: info.MimeType,
				},
			}
			emojiPack.Images[emojiName] = pi
		}
	}

	_, err = mtxClient.PutEmotePack(*roomPtr, packId, emojiPack)

	if err != nil {
		fmt.Println("Error uploading emote pack:", err)
		return
	}

}

type imageInfo struct {
	Path       string
	Name       string
	MimeType   string
	IsAnimated bool
	Width      int
	Height     int
	Bytes      []byte
}

func find(root, ext string) []string {
	var a []string
	filepath.WalkDir(root, func(s string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if filepath.Ext(d.Name()) == ext {
			a = append(a, s)
		}
		return nil
	})
	return a
}

func getImageInfo(path string) (*imageInfo, error) {

	info := imageInfo{Path: path, Name: filepath.Base(path)}

	file_bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	mtype := mimetype.Detect(file_bytes)
	info.MimeType = mtype.String()

	reader := bytes.NewReader(file_bytes)
	anim, err := midec.IsAnimated(reader)
	if err != nil {
		return nil, err
	}

	if anim {
		info.IsAnimated = true
		if info.MimeType == "image/png" || info.MimeType == "image/vnd.mozilla.apng" {
			info.MimeType = "image/apng"
		}
	}

	reader.Seek(0, 0)
	im, _, err := image.DecodeConfig(reader)
	if err != nil {
		return nil, err
	}

	info.Width = im.Width
	info.Height = im.Height

	info.Bytes = file_bytes

	return &info, nil
}
