package main

import (
  "fmt"
  "os"
  "sync"
  "os/exec"
  "context"
  "log"
  "strings"
  "net/http"
  "io/ioutil"
  "path/filepath"
  "encoding/json"
)

type Playlist struct {
  Name string `mapstructure:"name" bson:"name" json:"name"`
}

type Track struct {
  EId         string    `mapstructure:"eId" bson:"eId" json:"eId"`
  Name        string    `mapstructure:"name" bson:"name" json:"name"`
  Playlist    *Playlist `mapstructure:"pl" bson:"pl" json:"pl"`
  UserName    string    `mapstructure:"uNm" bson:"uNm" json:"uNm"`
}

func getTracks(path string) []*Track {
  resp, err := http.Get(path)
  if err != nil {
    log.Fatal(err.Error())
  }

  defer resp.Body.Close()

  content, err := ioutil.ReadAll(resp.Body)
  if err != nil {
    log.Fatal(err.Error())
  }

  tracks := []*Track{}

  err = json.Unmarshal(content, &tracks)
  if err != nil {
    log.Fatal(err.Error())
  }

  return tracks
}

func downloadTrack(ctx context.Context, track *Track) {
  playlistpath := filepath.Join(track.UserName, strings.TrimSpace(track.Playlist.Name))
  fullTrackPath := filepath.Join(playlistpath, "%(title)s.%(ext)s")

  os.MkdirAll(playlistpath, 0700)

  url := fmt.Sprintf("https://www.youtube.com/watch?v=%s", strings.Split(track.EId, "/")[2])
  options := []string{"-i", "-x", "-o", fullTrackPath, "--audio-format", "mp3", "--audio-quality", "320K", url}

  cmd := exec.CommandContext(ctx, "youtube-dl", options...)
  cmd.Stdout = os.Stdout
  cmd.Stderr = os.Stdout

  if err := cmd.Run(); err != nil {
    log.Printf("Error: %s", err.Error())
  }
}

func worker(parentCtx context.Context, wg *sync.WaitGroup, ch chan *Track) {
  ctx, cancel := context.WithCancel(parentCtx)
  defer cancel()

  for {
    select {
    case track :=<-ch:
      wg.Add(1)
      downloadTrack(ctx, track)
      wg.Done()
    case <-ctx.Done():
      return
    }
  }
}

func main() {
  if len(os.Args) < 2 {
    fmt.Printf("usage: ./whyd2HD USER-ID (example : 5095275a7e91c862b2a83f49)")
    os.Exit(-1)
  }

  ctx, cancel := context.WithCancel(context.Background())
  defer cancel()

  tracks := getTracks(fmt.Sprintf("https://openwhyd.org/u/%s?format=json&limit=10000000000", os.Args[1]))

  ch := make(chan *Track)
  wg := sync.WaitGroup{}

  for i := 0; i < 5; i++ {
    go worker(ctx, &wg, ch)
  }

  for _, track := range tracks {
    if track.Name != "" {
      ch <- track
    }
  }

  wg.Wait()
}
