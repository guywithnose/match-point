package main

import (
    "log"
)

type User struct {
    Name string `json:"name" gorethink:"id,omitempty"`
    Password string `json:"password"`
    Salt string `json:"salt"`
    IsAdmin bool `json:"isAdmin"`
    NotifyIds []string `json:"notifyIds"`
    Notification Notification `json:"notification"`
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
