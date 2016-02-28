package main

import (
    "log"
    "net/http"
    "encoding/json"
    "io/ioutil"
    "regexp"
    "strconv"
)

type Manifest struct {
    Name string `json:"name"`
    GcmSenderId string `json:"gcm_sender_id"`
    Icons []ManifestIcon `json:"icons"`
}

type ManifestIcon struct {
    Source string `json:"src"`
    Size string `json:"sizes"`
    Type string `json:"type"`
    Density float64 `json:"density"`
}

func buildManifest(w http.ResponseWriter, r *http.Request) {
    var manifest Manifest = Manifest{
        Name: "Match Point",
        GcmSenderId: gcmSenderId,
        Icons: make([]ManifestIcon, 0),
    }

    files, err := ioutil.ReadDir("public/images")
    if err != nil {
        log.Println(err.Error());
    }

    for key := range(files) {
        fileName := files[key].Name()
        re := regexp.MustCompile("soccerIcon-([0-9]+)x([0-9]+)-([0-9\\.]+)x.png")
        matches := re.FindAllStringSubmatch(fileName, -1)
        if matches != nil {
            density, _ := strconv.ParseFloat(matches[0][3], 64)
            manifest.Icons = append(manifest.Icons, ManifestIcon{
                Source: "/images/" + fileName,
                Size: matches[0][1] + "x" + matches[0][2],
                Type: "image/png",
                Density: density,
            })
        }
    }

    json, err := json.Marshal(manifest)
    if err != nil {
        log.Println(err)
    }

    w.Write(json)
}
