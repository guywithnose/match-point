package main

import (
    "github.com/gorilla/mux"
    "log"
    "net/http"
    r "github.com/dancannon/gorethink"
)

var (
    session *r.Session = getRethinkDbSession()
    activitiesTable = r.DB(databaseName).Table("activities")
    usersTable = r.DB(databaseName).Table("users")
)

type Message struct {
    Action string `json:"action"`
    Activity Activity `json:"activity"`
    Activities []Activity `json:"activities"`
    NewActivity Activity `json:"newActivity"`
    OldActivity Activity `json:"oldActivity"`
    User User `json:"user"`
    ErrorMessage string `json:"errorMessage"`
}

func main() {
    r := mux.NewRouter()
    r.HandleFunc("/ws", Socket(handleMessage))
    r.HandleFunc("/manifest.json", buildManifest)
    r.HandleFunc("/notification", getNotificationData)
    r.PathPrefix("/").Handler(http.FileServer(http.Dir("./public/")))
    http.Handle("/", r)
    if port == "" {
        port = "3000"
    }
    http.ListenAndServe("127.0.0.1:" + port, nil)
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
            res, err := activitiesTable.Get(msg.Activity.Id).Delete().Run(session)
            defer res.Close()
            if err != nil {
                log.Println(err.Error())
            }
        }
    case "join-activity":
        if verifyAuth(msg.User, sender) {
            appendUser(msg.Activity.Id, msg.User)
        }
    case "leave-activity":
        if verifyAuth(msg.User, sender) {
            removeUserFromActivity(msg.Activity.Id, msg.User)
        }
    case "subscribe-activity":
        if verifyAuth(msg.User, sender) {
            subscribeUser(msg.Activity.Id, msg.User)
        }
    case "unsubscribe-activity":
        if verifyAuth(msg.User, sender) {
            unsubscribeUser(msg.Activity.Id, msg.User)
        }
    case "reset-activity":
        if valid, isAdmin := authenticateUser(msg.User); valid {
            msg.User.IsAdmin = isAdmin
            resetActivity(msg.Activity.Id, msg.User, sender)
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
    case "set-numusers":
        if valid, isAdmin := authenticateUser(msg.User); valid {
            msg.User.IsAdmin = isAdmin
            setNumUsers(msg.Activity, msg.User)
        }
    }
}
