package main

import (
    "github.com/gorilla/mux"
    "log"
    "net/http"
    "os"
    r "github.com/dancannon/gorethink"
    "encoding/json"
    "bytes"
    "io/ioutil"
    "regexp"
    "strconv"
)

var (
    gcmSenderId = os.Getenv("MATCH_POINT_GCM_ID")
    gcmSenderSecret = os.Getenv("MATCH_POINT_GCM_SECRET")
    AuthKey = os.Getenv("MATCH_POINT_RETHINKDB_AUTH")
    databaseAddress = os.Getenv("MATCH_POINT_RETHINKDB_ADDRESS")
    databaseName = os.Getenv("MATCH_POINT_DATABASE")
    session *r.Session = getRethinkDbSession()
    activitiesTable = r.DB(databaseName).Table("activities")
    usersTable = r.DB(databaseName).Table("users")
)

type Activity struct {
    Id string `json:"id" gorethink:",omitempty"`
    Name string `json:"name"`
    Users []User `json:"users"`
    Subscribers []string `json:"subscribers"`
    MinUsers int `json:"minUsers"`
    MaxUsers int `json:"maxUsers"`
    Active bool `json:"active"`
    Initiator User `json:"initiator"`
}

type ActivityChangeResponse struct {
    NewValue Activity `gorethink:"new_val"`
    OldValue Activity `gorethink:"old_val"`
}

type User struct {
    Name string `json:"name" gorethink:"id,omitempty"`
    Password string `json:"password"`
    Salt string `json:"salt"`
    IsAdmin bool `json:"isAdmin"`
    NotifyIds []string `json:"notifyIds"`
    Notification Notification `json:"notification"`
}

type Notification struct {
    Title string `json:"title"`
    Body string `json:"body"`
}

type Message struct {
    Action string `json:"action"`
    Activities []Activity `json:"activities"`
    NewActivity Activity `json:"newActivity"`
    OldActivity Activity `json:"oldActivity"`
    Id string `json:"id"`
    User User `json:"user"`
    ErrorMessage string `json:"errorMessage"`
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

func main() {
    r := mux.NewRouter()
    r.HandleFunc("/ws", Socket(handleMessage))
    r.HandleFunc("/manifest.json", buildManifest)
    r.HandleFunc("/notification", getNotificationData)
    r.PathPrefix("/").Handler(http.FileServer(http.Dir("./public/")))
    http.Handle("/", r)
    http.ListenAndServe("127.0.0.1:3000", nil)
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

func getRethinkDbSession() *r.Session {
    var session *r.Session

    session, err := r.Connect(r.ConnectOpts{
        Address: databaseAddress,
        AuthKey: AuthKey,
    })
    if err != nil {
        log.Fatalln(err.Error())
    }

    return session;
}

func handleMessage(msg *Message, sender chan<- *Message, done <-chan bool) {
    switch msg.Action {
    case "subscribe-all-activities":
        go initializeActivities(sender)
        go listenForChanges(sender, done)
    case "add-activity":
        if verifyAdmin(msg.User, sender) {
            res, err := activitiesTable.Insert(msg.NewActivity).Run(session)
            defer res.Close()
            if err != nil {
                log.Println(err.Error())
            }
        }
    case "delete-activity":
        if verifyAdmin(msg.User, sender) {
            res, err := activitiesTable.Get(msg.Id).Delete().Run(session)
            defer res.Close()
            if err != nil {
                log.Println(err.Error())
            }
        }
    case "join-activity":
        if verifyAuth(msg.User, sender) {
            appendUser(msg.Id, msg.User)
        }
    case "leave-activity":
        if verifyAuth(msg.User, sender) {
            removeUserFromActivity(msg.Id, msg.User)
        }
    case "subscribe-activity":
        if verifyAuth(msg.User, sender) {
            subscribeUser(msg.Id, msg.User)
        }
    case "unsubscribe-activity":
        if verifyAuth(msg.User, sender) {
            unsubscribeUser(msg.Id, msg.User)
        }
    case "reset-activity":
        if valid, isAdmin := authenticateUser(msg.User); valid {
            msg.User.IsAdmin = isAdmin
            resetActivity(msg.Id, msg.User, sender)
        }
    case "getSalt":
        getSalt(sender, msg.User)
    case "login":
        login(sender, msg.User)
    case "newUser":
        newUser(sender, msg.User)
    case "add-notify-id":
        if verifyAuth(msg.User, sender) {
            addNotifyId(msg.User)
        }
    }
}

func verifyAdmin(user User, sender chan<- *Message) bool {
    if valid, isAdmin := authenticateUser(user); valid && isAdmin {
        return true
    } else {
        var m Message
        if !valid {
            m.Action = "auth-error"
            m.ErrorMessage = "Authentication Error"
        } else if !isAdmin {
            m.Action = "error"
            m.ErrorMessage = "You are not authorized to perform this action."
        }

        sender <- &m
        return false
    }
}

func verifyAuth(user User, sender chan<- *Message) bool {
    if valid, _ := authenticateUser(user); valid {
        return true
    } else {
        var m Message
        m.Action = "auth-error"
        m.ErrorMessage = "Authentication Error"

        sender <- &m
        return false
    }
}

func getSalt(sender chan<- *Message, user User) {
    var (
        m Message
        dbUser User = getUser(user.Name)
    )

    if dbUser.Salt != "" {
        m.Action = "user-salt"
        m.User = dbUser
    } else {
        m.Action = "login-error"
        m.ErrorMessage = "Invalid credentials"
    }

    sender <- &m
}

func login(sender chan<- *Message, user User) {
    var m Message

    if valid, _ := authenticateUser(user); valid {
        m.Action = "login"
        m.User = getUser(user.Name)
    } else {
        m.Action = "login-error"
        m.ErrorMessage = "Invalid credentials"
    }

    sender <- &m
}

func authenticateUser(user User) (bool, bool) {
    var dbUser User = getUserWithPassword(user.Name)
    return dbUser.Salt != "" && dbUser.Password == user.Password, dbUser.IsAdmin
}

func newUser(sender chan<- *Message, user User) {
    var (
        m Message
        dbUser User = getUser(user.Name)
    )

    if dbUser.Salt == "" && user.Name != "" && user.Name != "notauser" {
        res, err := usersTable.Insert(user).Run(session)
        defer res.Close()
        if err != nil {
            log.Println(err.Error())
            return
        }

        dbUser = getUser(user.Name)

        m.Action = "login"
        m.User = dbUser
    } else {
        m.Action = "login-error"
        if dbUser.Salt != "" {
            m.ErrorMessage = "Username already in use"
        } else if user.Name == "" || user.Name == "notauser" {
            m.ErrorMessage = "Invalid username"
        }
    }

    sender <- &m
}

func getUserWithPassword(userId string) User {
    var user User
    res, err := usersTable.Get(userId).Run(session)
    defer res.Close()
    if err != nil {
        log.Println(err.Error())
        return user
    }

    res.One(&user)
    return user
}

func getUser(userId string) User {
    var user User = getUserWithPassword(userId)
    user.Password = ""
    return user
}

func getActivity(activityId string) Activity {
    var activity Activity
    res, err := activitiesTable.Get(activityId).Run(session)
    defer res.Close()
    if err != nil {
        log.Println(err.Error())
        return activity
    }

    res.One(&activity)
    return activity
}

func removeUserFromActivity(activityId string, user User) {
    var a Activity = getActivity(activityId)
    var users []User;
    for key := range(a.Users) {
        if a.Users[key].Name != user.Name {
            users = append(users, a.Users[key])
        }
    }

    a.Users = users
    if len(a.Users) == 1 {
      a.Initiator = a.Users[0]
    }

    res, err := activitiesTable.Get(activityId).Update(a).Run(session)
    defer res.Close()
    if err != nil {
        log.Println(err.Error())
        return
    }
}

func unsubscribeUser(activityId string, user User) {
    var a Activity = getActivity(activityId)
    var subscribers []string;
    for key := range(a.Subscribers) {
        if a.Subscribers[key] != user.Name {
            subscribers = append(subscribers, a.Subscribers[key])
        }
    }

    a.Subscribers = subscribers

    res, err := activitiesTable.Get(activityId).Update(a).Run(session)
    defer res.Close()
    if err != nil {
        log.Println(err.Error())
        return
    }
}

func resetActivity(activityId string, user User, sender chan<- *Message) {
    var a Activity = getActivity(activityId)
    if a.Initiator.Name == user.Name || user.IsAdmin {
        a.Users = make([]User, 0)
        a.Initiator.Name = "notauser"
        res, err := activitiesTable.Get(activityId).Update(a).Run(session)
        defer res.Close()
        if err != nil {
            log.Println(err.Error())
            return
        }
    } else {
        var m Message
        m.Action = "error"
        m.ErrorMessage = "You are not authorized to perform this action."

        sender <- &m
    }
}

func addNotifyId(user User) {
    var dbUser User = getUserWithPassword(user.Name)
    if len(user.NotifyIds) != 1 {
        return
    }

    var newNotifyId = user.NotifyIds[0]
    for key := range(dbUser.NotifyIds) {
        if dbUser.NotifyIds[key] == newNotifyId {
            return
        }
    }

    dbUser.NotifyIds = append(dbUser.NotifyIds, newNotifyId)

    res, err := usersTable.Get(dbUser.Name).Update(dbUser).Run(session)
    defer res.Close()
    if err != nil {
        log.Println(err.Error())
        return
    }
}

func appendUser(id string, user User) {
    var a Activity = getActivity(id)
    if len(a.Users) >= a.MaxUsers {
        return
    }

    for key := range(a.Users) {
        if a.Users[key].Name == user.Name {
            return
        }
    }
    a.Users = append(a.Users, user)
    if len(a.Users) == 1 {
      a.Initiator = a.Users[0]
      notifySubscribers(a.Subscribers, user.Name + " wants you to come play " + a.Name, user.Name)
    }

    if len(a.Users) == a.MaxUsers {
      notifySubscribers(a.Subscribers, a.Name + " is now full", user.Name)
    }

    res, err := activitiesTable.Get(id).Update(a).Run(session)
    defer res.Close()
    if err != nil {
        log.Println(err.Error())
        return
    }
}

func subscribeUser(id string, user User) {
    var a Activity = getActivity(id)

    for key := range(a.Subscribers) {
        if a.Subscribers[key] == user.Name {
            return
        }
    }

    a.Subscribers = append(a.Subscribers, user.Name)

    res, err := activitiesTable.Get(id).Update(a).Run(session)
    defer res.Close()
    if err != nil {
        log.Println(err.Error())
        return
    }
}

func initializeActivities(sender chan<- *Message) {
    var m Message
    m.Action = "initialize-activities"
    res, err := activitiesTable.Run(session)
    defer res.Close()
    if err != nil {
        log.Println(err.Error())
        return
    }
    res.All(&m.Activities);

    sender <- &m
}

func listenForChanges(sender chan<- *Message, done <-chan bool) {
    var m Message
    m.Action = "update-activity"
    res, err := activitiesTable.Changes().Run(session)
    defer res.Close()
    if err != nil {
        log.Println(err.Error())
        return
    }
    var row ActivityChangeResponse
    go func() {
        for res.Next(&row) {
            m.NewActivity = row.NewValue
            m.OldActivity = row.OldValue
            sender <- &m
        }
    }()

    <-done

    res.Close()
}
