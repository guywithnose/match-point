package main

import "os"

var (
	gcmSenderId      = os.Getenv("MATCH_POINT_GCM_ID")
	gcmSenderSecret  = os.Getenv("MATCH_POINT_GCM_SECRET")
	databasePassword = os.Getenv("MATCH_POINT_RETHINKDB_PASSWORD")
	databaseAddress  = os.Getenv("MATCH_POINT_RETHINKDB_ADDRESS")
	databaseName     = os.Getenv("MATCH_POINT_DATABASE")
	port             = os.Getenv("PORT")
)
