package main

import (
	"log"
	"os"
	"strconv"

	calcpb "goa.design/goa/examples/calc/gen/grpc/calc"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

const (
	address = "localhost:8081"
)

func main() {
	// Set up a connection to the server.
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := calcpb.NewCalcClient(conn)

	var aStr, bStr string
	var a, b int64
	if len(os.Args) > 1 {
		aStr = os.Args[1]
		bStr = os.Args[2]
	}
	a, _ = strconv.ParseInt(aStr, 10, 32)
	b, _ = strconv.ParseInt(bStr, 10, 32)
	r, err := c.Add(context.Background(), &calcpb.AddRequest{A: int32(a), B: int32(b)})
	if err != nil {
		log.Fatalf("could not greet: %v", err)
	}
	log.Printf("Added: %d", r.Field)
}
