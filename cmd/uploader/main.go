package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	"github.com/kabili207/matrixemoji/pkg/api"
	"github.com/kabili207/matrixemoji/pkg/models"
	"github.com/sapphi-red/midec"
	_ "github.com/sapphi-red/midec/gif"  // import this to detect Animated GIF
	_ "github.com/sapphi-red/midec/png"  // import this to detect APNG
	_ "github.com/sapphi-red/midec/webp" // import this to detect Animated WebP
	"rsc.io/getopt"
)

func main() {

	serverPtr := flag.String("server", "", "the server URL")
	getopt.Alias("s", "server")

	pathPtr := flag.String("path", "", "the server URL")
	getopt.Alias("p", "path")

	roomPtr := flag.String("room", "", "to room ID")
	getopt.Alias("r", "room")

	namePtr := flag.String("name", "", "name of the emoji pack")
	getopt.Alias("n", "name")

	authPtr := flag.String("auth", "", "auth token")

	getopt.Parse()

	if authPtr == nil {
		if env, ok := os.LookupEnv("SYNAPSE_AUTH_TOKEN"); ok {
			authPtr = &env
		}
	}

	fmt.Println("word:", *serverPtr)
	fmt.Println("numb:", *pathPtr)
	fmt.Println("room:", *roomPtr)
	fmt.Println("name:", *namePtr)
	fmt.Println("auth:", *authPtr)
	fmt.Println("tail:", flag.Args())

	mtxClient := api.NewMatrixClient(*serverPtr, *authPtr)

	packId := mtxClient.EncodePackId(*namePtr)

	emojiPack, err := mtxClient.GetEmotePack(*roomPtr, packId)

	if err != nil {
		fmt.Println("Error fetching emote pack:", err)
		return
	}

	fi, err := os.Stat(*pathPtr)
	if err != nil {
		fmt.Println(err)
		return
	}

	if !fi.IsDir() {
		fmt.Println("Path is not a directory")
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
