package main

import (
  "fmt"
  "os"
  "sync"
  "os/exec"
  "context"
  "log"
  "bufio"
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

func loadDownloadedTracks(archiveFilePath string) map[string]struct{} {
  downloaded := map[string]struct{}{}
  file, err := os.Open(archiveFilePath)
  if err != nil { // file will be created by youtube-dl
    return downloaded
  }
  defer file.Close()

  scanner := bufio.NewScanner(file)

  for scanner.Scan() {
    line := scanner.Text()
    l := strings.Split(line, " ")
    origin := l[0]
    eid := l[1]
    if origin == "youtube" {
      downloaded[eid] = struct{}{}
    }
  }

  if err := scanner.Err(); err != nil {
    log.Fatal(err)
  }

  return downloaded
}

func downloadTrack(ctx context.Context, track *Track) {
  if track.Playlist == nil {
    track.Playlist = &Playlist{
      Name: "Default",
    }
  }

  playlistpath := filepath.Join(track.UserName, strings.TrimSpace(track.Playlist.Name))
  archiveFilePath := filepath.Join(playlistpath, "downloaded.txt")

  // TODO : dont reload the file each time, share it between workers
  downloaded := loadDownloadedTracks(archiveFilePath)

  eidSplit := strings.Split(track.EId, "/")
  if eidSplit[1] != "yt" {
    return
  }

  trackYtID := eidSplit[2]

  if _, ok := downloaded[trackYtID]; ok {
    println("Tracks already downloaded : ", track.Name)
    return
  }

  fullTrackPath := filepath.Join(playlistpath, "%(title)s.%(ext)s")

  os.MkdirAll(playlistpath, 0700)

  url := fmt.Sprintf("https://www.youtube.com/watch?v=%s", trackYtID)
  options := []string{"--download-archive", archiveFilePath, "--no-post-overwrites", "-i", "-x", "-o", fullTrackPath, "--audio-format", "mp3", "--audio-quality", "320K", url}

  cmd := exec.CommandContext(ctx, "youtube-dl", options...)

//  cmd.Stdout = os.Stdout
//  cmd.Stderr = os.Stdout

  cmd.Stdout = ioutil.Discard
  cmd.Stderr = ioutil.Discard

  if err := cmd.Run(); err != nil {
    log.Printf("Error on %s : %s", track.Name, err.Error())
  }

  println("Track downloaded : ", track.Name)
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
    // 52e2620f7e91c862b2b3f66a (mr rien)
    os.Exit(-1)
  }

  ctx, cancel := context.WithCancel(context.Background())
  defer cancel()

//  user := os.Args[1]
  user := "rien"
  playlist_id := "171"
//  url := fmt.Sprintf("https://openwhyd.org/u/%s?format=json&limit=10000000000", user)
  url := fmt.Sprintf("https://openwhyd.org/%s/playlist/%s?format=json&limit=1000000000000000000", user, playlist_id)
  tracks := getTracks(url)

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
