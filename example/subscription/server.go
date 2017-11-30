package main

import (
	"log"
	"net/http"
	"time"

	graphql "github.com/lpalmes/graphql-go"
	"github.com/lpalmes/graphql-go/relay"
	"github.com/lpalmes/graphql-go/subscriptions"
	"github.com/rs/cors"
)

var channel = make(subscriptions.SubscriptionChannel)

var schema *graphql.Schema

var Schema = `
	schema {
		query: Query
		subscription: Subscription
	}
	# The query type, represents all of the entry points into our object graph
	type Query {
		greet: Greeter
		bye: String!
	}

	type Greeter {
		hello(name: String!): String!
	}

	type Subscription {
		ping: String!
		greet: Greeter
	}
`

type Resolver struct{}

type greeterResolver struct{}

func (r *Resolver) Greet() *greeterResolver {
	return &greeterResolver{}
}

func (r *greeterResolver) Hello(args *struct {
	Name string
}) string {
	return "hello " + args.Name
}

func (r *Resolver) Bye() string {
	return "friend"
}

func (r *Resolver) Ping() string {
	return "Pong"
}

func init() {
	schema = graphql.MustParseSchema(Schema, &Resolver{})
}

func main() {
	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(page)
	}))

	go func() {
		for {
			channel <- subscriptions.SubscriptionPayload{
				Name:    "ping",
				Payload: "Blue",
			}
			time.Sleep(time.Second * 2)
		}
	}()

	mux.Handle("/graphql", &relay.Handler{Schema: schema})
	go subscriptions.Listen(schema, &channel)
	mux.Handle("/subscriptions", &subscriptions.Handler{Schema: schema})

	handler := cors.Default().Handler(mux)
	log.Fatal(http.ListenAndServe(":8080", handler))
}

var page = []byte(`
<!DOCTYPE html>
<html>
	<head>
		<link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/graphiql/0.10.2/graphiql.css" />
		<script src="https://cdnjs.cloudflare.com/ajax/libs/fetch/1.1.0/fetch.min.js"></script>
		<script src="https://cdnjs.cloudflare.com/ajax/libs/react/15.5.4/react.min.js"></script>
		<script src="https://cdnjs.cloudflare.com/ajax/libs/react/15.5.4/react-dom.min.js"></script>
		<script src="https://cdnjs.cloudflare.com/ajax/libs/graphiql/0.10.2/graphiql.js"></script>
	</head>
	<body style="width: 100%; height: 100%; margin: 0; overflow: hidden;">
		<div id="graphiql" style="height: 100vh;">Loading...</div>
		<script>
			function graphQLFetcher(graphQLParams) {
				return fetch("/query", {
					method: "post",
					body: JSON.stringify(graphQLParams),
					credentials: "include",
				}).then(function (response) {
					return response.text();
				}).then(function (responseBody) {
					try {
						return JSON.parse(responseBody);
					} catch (error) {
						return responseBody;
					}
				});
			}

			ReactDOM.render(
				React.createElement(GraphiQL, {fetcher: graphQLFetcher}),
				document.getElementById("graphiql")
			);
		</script>
	</body>
</html>
`)
