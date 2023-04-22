package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/gorilla/mux"
	"github.com/spf13/cobra"
)

var (
	playlist string
	port     int
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "boombox",
		Short: "audio streaming server for simple minds",
		Run:   runServer,
	}

	rootCmd.Flags().StringVar(&playlist, "playlist", "", "Path to the playlist file (m3u)")
	rootCmd.Flags().IntVar(&port, "port", 12401, "Port on which the server will run")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
func runServer(cmd *cobra.Command, args []string) {
	flag.Parse()

	if playlist == "" {
		log.Fatal("Please provide a valid path to a playlist file (m3u)")
	}

	router := mux.NewRouter()
	router.HandleFunc("/stream", streamHandler)

	log.Printf("Starting server on port %d\n", port)
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(port), router))
}

func streamHandler(w http.ResponseWriter, r *http.Request) {
	f, err := os.Open(playlist)
	if err != nil {
		log.Fatal(err)
	}

	streamer, format, err := mp3.Decode(f)
	if err != nil {
		log.Fatal(err)
	}
	defer streamer.Close()

	speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))

	done := make(chan struct{})
	speaker.Play(beep.Seq(streamer, beep.Callback(func() {
		close(done)
	})))

	select {
	case <-done:
	case <-r.Context().Done():
	}
}
