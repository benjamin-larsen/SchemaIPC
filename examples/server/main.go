package main

import schemaipc "github.com/benjamin-larsen/goschemaipc"

var server = schemaipc.Server{
	SchemaPath: "",
	MessageOverflowPolicy: schemaipc.MessageOverflowDiscard,
	MaxMessageSize: 1024,
}

func main() {
	server.ListenAndServe("tcp", ":6000")
}