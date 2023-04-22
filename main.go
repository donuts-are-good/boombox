package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/gorilla/mux"
	"github.com/spf13/cobra"
)

var (
	playlist      string
	port          int
	listenerCount int
	listenerMutex sync.Mutex
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "boombox",
		Short: "audio streaming server for simple minds",
		Run:   runServer,
	}

	rootCmd.Flags().StringVar(&playlist, "playlist", "", "Path to the playlist file (m3u)")
	rootCmd.Flags().IntVar(&port, "port", 42001, "Port on which the server will run")

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
	listenerMutex.Lock()
	listenerCount++
	listenerMutex.Unlock()

	defer func() {
		listenerMutex.Lock()
		listenerCount--
		listenerMutex.Unlock()
	}()

	pathMap := make(map[string]struct{})

	for {
		paths, err := parseM3u(playlist)
		if err != nil {
			log.Println("error parsing m3u: ", err)
		}

		for _, path := range paths {
			if _, ok := pathMap[path]; !ok {
				pathMap[path] = struct{}{}

				listenerMutex.Lock()
				log.Printf("Listeners connected: %d  Playing track: %s\n", listenerCount, path)
				listenerMutex.Unlock()

				f, err := os.Open(path)
				if err != nil {
					log.Println("error opening file: ", err)
					continue
				}

				streamer, format, err := mp3.Decode(f)
				if err != nil {
					log.Println("error decoding file: ", err)
					f.Close()
					continue
				}

				speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))

				done := make(chan struct{})
				speaker.Play(beep.Seq(streamer, beep.Callback(func() {
					close(done)
				})))

				select {
				case <-done:
					streamer.Close()
					f.Close()
				case <-r.Context().Done():
					streamer.Close()
					f.Close()
					return
				}
			}
		}
	}
}

func parseM3u(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var paths []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		path := line
		if !filepath.IsAbs(path) {
			dir := filepath.Dir(filename)
			path = filepath.Join(dir, line)
		}

		paths = append(paths, path)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return paths, nil
}
