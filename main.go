// © 2022 Vlad-Stefan Harbuz <vlad@vladh.net>
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	// "github.com/davecgh/go-spew/spew"
	"github.com/dhowden/tag"
	"github.com/julienschmidt/httprouter"
)

type state struct {
	Playing map[string]Song
}

type Song struct {
	Path string `json:"path"`
	Title string `json:"title"`
	Artist string `json:"artist"`
	Album string `json:"album"`
	AlbumArtist string `json:"albumartist"`
	FileType string `json:"filetype"`
}

type Station struct {
	Id string
	Name string
	Paths []string
}

type Config struct {
	MusicRoot string
	Stations []Station
}

type ErrResponse struct {
	Err string `json:"err"`
}

type RandomResponse = Song

var gState state


func isSong(path string) bool {
	return strings.HasSuffix(path, ".flac") ||
		strings.HasSuffix(path, ".mp3") ||
		strings.HasSuffix(path, ".m4a")
}


func getConfig() Config {
	var config Config

	configDir := os.Getenv("XDG_CONFIG_HOME")
	if len(configDir) == 0 {
		homedir, err := os.UserHomeDir()
		if err != nil { panic(err) }
		configDir = homedir + "/.config/"
	}
	configPath := configDir + "/radio-api/config.toml"

	_, err := toml.DecodeFile(configPath, &config)
	if err != nil { log.Fatal(err) }
	return config
}


func getSong(path string) (Song, error) {
	var err error

	f, err := os.Open(path)
	if err != nil { return Song{}, err}

	m, err := tag.ReadFrom(f)
	if err != nil { return Song{}, err }

	defer f.Close()
	return Song{
		Path: path,
		Title: m.Title(),
		Artist: m.Artist(),
		Album: m.Album(),
		AlbumArtist: m.AlbumArtist(),
		FileType: string(m.FileType()),
	}, nil
}


func getSongsInDirs(dirs []string) ([]string, error) {
	var err error
	songPaths := make([]string, 0)
	config := getConfig()

	for _, dir := range dirs {
		fullDir := config.MusicRoot + "/" + dir
		err = filepath.Walk(fullDir, func(path string, fileinfo os.FileInfo, err error) error {
			if err != nil { return err }
			if isSong(path) {
				songPaths = append(songPaths, path)
			}
			return nil
		})
		if err != nil { return nil, err }
	}

	return songPaths, nil
}


func getStation(config Config, id string) Station {
	for _, candidate := range config.Stations {
		if candidate.Id == id {
			return candidate
		}
	}
	return Station{}
}


func handleNp(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var err error

	config := getConfig()
	station := getStation(config, ps.ByName("stationId"))

	if len(station.Id) == 0 {
		resData := ErrResponse{Err: "Invalid station"}
		err = json.NewEncoder(w).Encode(resData)
		if err != nil { log.Printf("ERROR: %s\n", err) }
		return
	}

	err = json.NewEncoder(w).Encode(gState.Playing[station.Id])
	if err != nil { log.Printf("ERROR: %s\n", err) }
}


func handleNpSet(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var err error
	var song Song

	config := getConfig()
	station := getStation(config, ps.ByName("stationId"))

	decoder := json.NewDecoder(r.Body)
	err = decoder.Decode(&song)
	if err != nil { log.Printf("ERROR: %s\n", err) }
	log.Printf("%+v\n", song)

	gState.Playing[station.Id] = song
}


func handleRandom(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var err error

	config := getConfig()
	station := getStation(config, ps.ByName("stationId"))

	if len(station.Id) == 0 {
		resData := ErrResponse{Err: "Invalid station"}
		err = json.NewEncoder(w).Encode(resData)
		if err != nil { log.Printf("ERROR: %s\n", err) }
		return
	}

	songPaths, err := getSongsInDirs(station.Paths)
	if err != nil {
		resData := ErrResponse{Err: "Could not get songs for station"}
		err = json.NewEncoder(w).Encode(resData)
		if err != nil { log.Printf("ERROR: %s\n", err) }
		return
	}

	chosenPath := songPaths[rand.Intn(len(songPaths))]
	song, err := getSong(chosenPath)
	if err != nil { log.Printf("ERROR: %s\n", err) }

	err = json.NewEncoder(w).Encode(song)
	if err != nil { log.Printf("ERROR: %s\n", err) }
}


func main() {
	rand.Seed(time.Now().Unix())
	log.SetFlags(0)

	gState.Playing = make(map[string]Song)

	router := httprouter.New()
	router.GET("/api/:stationId/np", handleNp)
	router.POST("/api/:stationId/np", handleNpSet)
	router.GET("/api/:stationId/random", handleRandom)

	log.Printf("Listening on port 8100\n")
	log.Fatal(http.ListenAndServe(":8100", router))
}
