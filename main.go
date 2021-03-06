package main

import (
	"errors"
	"fmt"
	"log"

	// Import the generated protobuf code
	userService "github.com/JekaTka/shippy-user-service/proto/auth"
	vesselProto "github.com/JekaTka/shippy-vessel-service/proto/vessel"
	"golang.org/x/net/context"

	pb "github.com/JekaTka/shippy-consignment-service/proto/consignment"

	micro "github.com/micro/go-micro"
	"github.com/micro/go-micro/client"
	"github.com/micro/go-micro/metadata"
	"github.com/micro/go-micro/server"

	"os"
)

const (
	defaultHost = "0.0.0.0:27017"
)

func main() {

	// Database host from the environment variables
	host := os.Getenv("DB_HOST")

	if host == "" {
		host = defaultHost
	}

	session, err := CreateSession(host)

	// Mgo creates a 'master' session, we need to end that session
	// before the main function closes.
	defer session.Close()

	if err != nil {

		// We're wrapping the error returned from our CreateSession
		// here to add some context to the error.
		log.Panicf("Could not connect to datastore with host %s - %v", host, err)
	}

	// Create a new service. Optionally include some options here.
	srv := micro.NewService(

		// This name must match the package name given in your protobuf definition
		micro.Name("shippy.consignment"),
		// Our auth middleware
		micro.WrapHandler(AuthWrapper),
	)

	fmt.Println("Connect to vessel")
	vesselClient := vesselProto.NewVesselServiceClient("shippy.vessel", srv.Client())

	// Init will parse the command line flags.
	srv.Init()

	fmt.Println("Register shipping service handler")

	// Register handler
	pb.RegisterShippingServiceHandler(srv.Server(), &service{session, vesselClient})

	// Run the server
	if err := srv.Run(); err != nil {
		log.Println("Some error found:", err)
		fmt.Println(err)
	}
}

// AuthWrapper is a high-order function which takes a HandlerFunc
// and returns a function, which takes a context, request and response interface.
// The token is extracted from the context set in our consignment-cli, that
// token is then sent over to the user service to be validated.
// If valid, the call is passed along to the handler. If not,
// an error is returned.
func AuthWrapper(fn server.HandlerFunc) server.HandlerFunc {
	return func(ctx context.Context, req server.Request, resp interface{}) error {
		meta, ok := metadata.FromContext(ctx)
		if !ok {
			return errors.New("no auth meta-data found in request")
		}

		// Note this is now uppercase (not entirely sure why this is...)
		token := meta["Token"]
		log.Println("Authenticating with token: ", token)

		// Auth here
		authClient := userService.NewAuthClient("shippy.auth", client.DefaultClient)
		_, err := authClient.ValidateToken(context.Background(), &userService.Token{
			Token: token,
		})
		if err != nil {
			log.Println("Some error here")
			return err
		}
		err = fn(ctx, req, resp)
		log.Println("Err", err)
		return err
	}
}
