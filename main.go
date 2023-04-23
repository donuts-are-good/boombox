package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

func main() {
	// Get the M3U file path and port number from the command-line arguments.
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "usage: %s <m3u file> <port>\n", os.Args[0])
		os.Exit(1)
	}
	m3uPath := os.Args[1]
	port, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid port number: %s\n", os.Args[2])
		os.Exit(1)
	}

	// Parse the M3U file to get the audio stream URL.
	streamURL, err := parseM3u(m3uPath)
	if err != nil {
		log.Fatal("failed to parse M3U file: ", err)
	}

	// Open a TCP listener on the specified port.
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal("failed to open TCP listener: ", err)
	}
	defer ln.Close()

	// Accept incoming client connections.
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println("failed to accept client connection: ", err)
			continue
		}

		// Send the Shoutcast response headers.
		io.WriteString(conn, "ICY 200 OK\r\n")
		io.WriteString(conn, "Content-Type: audio/mpeg\r\n")
		io.WriteString(conn, "Cache-Control: no-cache\r\n")
		io.WriteString(conn, fmt.Sprintf("Content-Length: %d\r\n\r\n", -1))

		// Start streaming the audio data to the client.
		go func(conn net.Conn) {
			defer conn.Close()

			// Open a new HTTP request to the audio stream URL.
			resp, err := http.Get(streamURL)
			if err != nil {
				log.Println("failed to open audio stream: ", err)
				return
			}
			defer resp.Body.Close()

			// Copy the audio data from the HTTP response to the client connection.
			_, err = io.Copy(conn, resp.Body)
			if err != nil {
				log.Println("failed to stream audio data: ", err)
				return
			}
		}(conn)
	}
}

func parseM3u(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		line = strings.TrimPrefix(line, "file://")
		u, err := url.Parse(line)

		if err != nil {
			log.Println("error parsing audio stream URL: ", err)
			continue
		}

		return u.String(), nil
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return "", fmt.Errorf("no audio stream URL found in M3U file")
}
