package main

import (
    "log"
    "net/http"
    "encoding/json"
    "bytes"
    "io/ioutil"
)

type Notification struct {
    Title string `json:"title"`
    Body string `json:"body"`
}

type GcmRequest struct {
    RegistrationIds []string `json:"registration_ids"`
}

type GcmResponse struct {
    MulticastId int `json:"multicast_id"`
    Successes int `json:"success"`
    Failures int `json:"failure"`
    CanonicalIds int `json:"canonical_ids"`
    Results []GcmResult `json:"results"`
}

type GcmResult struct {
    MessageId string `json:"message_id"`
}

func notifySubscribers(subscribers []string, notificationBody string, actionUser string) {
    var registrationIds []string = make([]string, 0)
    var notification Notification = Notification{Title: "Match Point", Body: notificationBody}
    for key := range(subscribers) {
        if subscribers[key] == actionUser {
            continue;
        }

        var user User = getUserWithPassword(subscribers[key])
        for notifyKey := range(user.NotifyIds) {
            registrationIds = append(registrationIds, user.NotifyIds[notifyKey])
        }

        user.Notification = notification

        res, err := usersTable.Get(user.Name).Update(user).Run(session)
        defer res.Close()
        if err != nil {
            log.Println(err.Error())
            return
        }
    }

    sendNotifications(registrationIds)
}

func sendNotifications(registrationIds []string) {
    if len(registrationIds) == 0 {
        return
    }

    var requestBody GcmRequest;
    requestBody.RegistrationIds = registrationIds

    requestBodyBytes, err := json.Marshal(requestBody)
    if err != nil {
        log.Println(err)
    }

    req, err := http.NewRequest(http.MethodPost, "https://android.googleapis.com/gcm/send", bytes.NewBuffer(requestBodyBytes));
    if err != nil {
        log.Println(err.Error())
        return
    }

    req.Header.Set("Authorization", "key=" + gcmSenderSecret)
    req.Header.Set("Content-Type", "application/json")

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        log.Println(err.Error())
        return
    }

    defer resp.Body.Close()

    var response GcmResponse
    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        log.Println(err.Error())
        return
    }

    err = json.Unmarshal(body, &response)
    if err != nil {
        log.Println(err.Error())
        return
    }

    //TODO Use response
}

func getNotificationData(w http.ResponseWriter, r *http.Request) {
    var (
        userName = getCookieValue(r, "userName");
        passwordHash = getCookieValue(r, "passwordHash");
    )

    if valid, _ := authenticateUser(User{Name: userName, Password: passwordHash}); !valid {
        return
    }

    var user User = getUser(userName)

    notificationData, err := json.Marshal(user.Notification)
    if err != nil {
        log.Println(err)
    }

    w.Write(notificationData)
}

func getCookieValue(r *http.Request, name string) string {
    cookie, err := r.Cookie(name)
    if err != nil {
        return ""
    }

    return cookie.Value
}
