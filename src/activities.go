package main

import (
    "log"
    "time"
)

var (
    inactivityExpires = 15 * time.Minute
    activeGameExpires = 1 * time.Hour
)

type Activity struct {
    Id string `json:"id" gorethink:",omitempty"`
    Name string `json:"name"`
    Users []User `json:"users"`
    Subscribers []string `json:"subscribers"`
    NumUsers int `json:"numUsers"`
    Active bool `json:"active"`
    Initiator User `json:"initiator"`
    Created int64 `json:"created"`
    Expires int64 `json:"expires"`
}

type ActivityChangeResponse struct {
    NewValue Activity `gorethink:"new_val"`
    OldValue Activity `gorethink:"old_val"`
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

    if len(a.Users) == 0 {
        a.Expires = 0;
    } else {
        a.Expires = time.Now().Add(inactivityExpires).Unix();
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
        handleReset(activityId);
    } else {
        var m Message
        m.Action = "error"
        m.ErrorMessage = "You are not authorized to perform this action."

        sender <- &m
    }
}

func handleReset(activityId string) {
    var a Activity = getActivity(activityId)
    a.Users = make([]User, 0)
    a.Initiator.Name = "notauser"
    res, err := activitiesTable.Get(activityId).Update(a).Run(session)
    defer res.Close()
    if err != nil {
        log.Println(err.Error())
        return
    }
}

func appendUser(id string, user User) {
    var a Activity = getActivity(id)
    if len(a.Users) >= a.NumUsers {
        return
    }

    for key := range(a.Users) {
        if a.Users[key].Name == user.Name {
            return
        }
    }
    a.Users = append(a.Users, user)
    if len(a.Users) == 1 {
      a.Created = time.Now().Unix();
      a.Initiator = a.Users[0]
      notifySubscribers(a.Subscribers, user.Name + " wants you to come play " + a.Name, user.Name)
    }

    a.Expires = time.Now().Add(inactivityExpires).Unix();

    if len(a.Users) == a.NumUsers {
      notifySubscribers(a.Subscribers, a.Name + " is now full", user.Name)
      a.Expires = time.Now().Add(activeGameExpires).Unix();
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

func setNumUsers(activity Activity, user User) {
    var a Activity = getActivity(activity.Id)
    if a.Initiator.Name != user.Name && user.IsAdmin == false && activity.NumUsers >= len(activity.Users) {
        return
    }

    a.NumUsers = activity.NumUsers
    res, err := activitiesTable.Get(activity.Id).Update(a).Run(session)
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

func listenForChanges() {
    var m Message
    m.Action = "update-activity"
    res, err := activitiesTable.Changes().Run(session)
    defer res.Close()
    if err != nil {
        log.Println(err.Error())
        return
    }
    var row ActivityChangeResponse
    for res.Next(&row) {
        m.NewActivity = row.NewValue
        m.OldActivity = row.OldValue
        for _, sender := range(openSockets) {
            sender <- &m
        }

        updateExpiration <- row.NewValue
    }
}
