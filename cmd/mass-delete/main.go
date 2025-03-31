package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"strings"
	"time"

	"rsc.io/getopt"

	"github.com/kabili207/matrix-tools/pkg/api"
	"github.com/kabili207/matrix-tools/pkg/models"
)

func main() {

	serverPtr := flag.String("server", "", "the server URL")
	getopt.Alias("s", "server")

	roomPtr := flag.String("room", "", "room ID")
	getopt.Alias("r", "room")

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

	if paramError {
		return
	}

	now := time.Now()
	r := rand.New(rand.NewSource(now.UnixMicro()))

	mtxClient := api.NewMatrixClient(*serverPtr, *authPtr)

	events, err := mtxClient.GetRoomEvents(*roomPtr, "")

	if err != nil {
		fmt.Println("Error fetching messages:", err)
		return
	}

	err = deleteEvents(events, mtxClient, *roomPtr, r)
	if err != nil {
		fmt.Println("Error redacting messages:", err)
		return
	}

	next := events.End
	for next != "" {

		events, err := mtxClient.GetRoomEvents(*roomPtr, next)

		if err != nil {
			fmt.Println("Error fetching messages:", err)
			return
		}

		err = deleteEvents(events, mtxClient, *roomPtr, r)
		if err != nil {
			fmt.Println("Error redacting messages:", err)
			return
		}
		next = events.End
	}

}

func deleteEvents(events *models.MessageResponse, mtxClient api.MatrixClient, roomId string, r *rand.Rand) error {

	for _, evt := range events.Chunk {
		if evt.EventType == "m.room.message" && len(evt.Content) != 0 {
			txid := fmt.Sprintf("redact_%d", r.Int63())
			err := mtxClient.RedactEvent(roomId, evt.EventId, txid)

			if err != nil {
				return err
			}
		}

	}
	return nil
}
