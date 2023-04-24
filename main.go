package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type StreamWriter struct {
	sync.RWMutex
	clients map[chan []byte]bool
}

type ChanReader struct {
	ch chan []byte
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run main.go <IP:Port> <PlaylistFile>")
		return
	}

	addr := os.Args[1]
	playlistFile := os.Args[2]

	file, err := os.Open(playlistFile)
	if err != nil {
		fmt.Println("error opening playlist file:", err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var playlist []string
	for scanner.Scan() {
		playlist = append(playlist, strings.TrimSpace(scanner.Text()))
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("error reading playlist file:", err)
		return
	}

	stream := NewStreamWriter()
	go streamPlaylist(stream, playlist)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		clientChan := stream.AddClient()
		defer stream.RemoveClient(clientChan)

		w.Header().Set("Content-Type", "audio/mpeg")
		io.Copy(w, &ChanReader{ch: clientChan})
	})

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Println("error starting server:", err)
		return
	}
	fmt.Printf("%s started at %s\n", playlistFile, addr)

	http.Serve(ln, nil)
}

func (sw *StreamWriter) Write(p []byte) (n int, err error) {
	sw.RLock()
	defer sw.RUnlock()

	for client := range sw.clients {
		select {
		case client <- p:
		default:
		}
	}

	return len(p), nil
}

func (sw *StreamWriter) AddClient() chan []byte {
	sw.Lock()
	defer sw.Unlock()

	client := make(chan []byte, 100)
	sw.clients[client] = true
	return client
}

func (sw *StreamWriter) RemoveClient(client chan []byte) {
	sw.Lock()
	defer sw.Unlock()

	delete(sw.clients, client)
	close(client)
}

func NewStreamWriter() *StreamWriter {
	return &StreamWriter{
		clients: make(map[chan []byte]bool),
	}
}

func (r *ChanReader) Read(p []byte) (n int, err error) {
	data, ok := <-r.ch
	if !ok {
		return 0, io.EOF
	}
	return copy(p, data), nil
}

func streamPlaylist(sw *StreamWriter, playlist []string) {
	bufSize := 8192
	buffer := make([]byte, bufSize)
	ticker := time.NewTicker(20 * time.Millisecond)

	for {
		for _, file := range playlist {
			fmt.Printf("%d:%d now playing: %s\n", time.Now().Hour(), time.Now().Minute(), file)

			audioFile, err := os.Open(file)
			if err != nil {
				fmt.Println("error opening audio file:", err)
				continue
			}

			for {
				<-ticker.C
				n, err := audioFile.Read(buffer)
				if err != nil && err != io.EOF {
					fmt.Println("error reading audio file:", err)
					break
				}

				if n == 0 {
					break
				}

				sw.Write(buffer[:n])
			}

			audioFile.Close()
		}
	}
}
