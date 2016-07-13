package main

import (
	"log"
	"time"
)

var (
	updateExpiration chan Activity          = make(chan Activity)
	timers           map[string]*time.Timer = make(map[string]*time.Timer)
)

func handleExpirations() {
	go func() {
		for {
			select {
			case activity := <-updateExpiration:
				if timers[activity.Id] != nil {
					active := timers[activity.Id].Reset(time.Unix(activity.Expires, 0).Sub(time.Now()))
					if !active {
						go expire(activity)
					}
				} else {
					timers[activity.Id] = time.NewTimer(time.Unix(activity.Expires, 0).Sub(time.Now()))
					go expire(activity)
				}
			}
		}
	}()

	res, err := activitiesTable.Run(session)
	defer res.Close()
	if err != nil {
		log.Println(err.Error())
		return
	}

	var a Activity
	for res.Next(&a) {
		timers[a.Id] = time.NewTimer(time.Unix(a.Expires, 0).Sub(time.Now()))
		go expire(a)
	}
}

func expire(activity Activity) {
	<-timers[activity.Id].C
	handleReset(activity.Id)
}
