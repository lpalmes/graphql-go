package subscriptions

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	graphql "github.com/lpalmes/graphql-go"
	"github.com/lpalmes/graphql-go/internal/query"
)

type ConnectionContext struct {
	conn           *websocket.Conn
	operationQuery OperationQuery
	events         []string
	request        *http.Request
	ctx            context.Context
}

type SubscriptionPayload struct {
	Name    string
	Payload interface{}
}

type SubscriptionChannel chan SubscriptionPayload

var connections = []*ConnectionContext{}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	Subprotocols: []string{"graphql-ws"},
}

var first = false

type QueryParams struct {
	Query         string                 `json:"query"`
	OperationName string                 `json:"operationName"`
	Variables     map[string]interface{} `json:"variables"`
}

type OperationQuery struct {
	Payload       QueryParams `json:"payload"`
	ID            string      `json:"id"`
	OperationType string      `json:"type"`
}

type OperationMessage struct {
	Payload       interface{} `json:"payload"`
	ID            string      `json:"id"`
	OperationType string      `json:"type"`
}

type Handler struct {
	Schema  *graphql.Schema
	Channel *SubscriptionChannel
}

func Listen(schema *graphql.Schema, channel *SubscriptionChannel) {
	for {
		event := <-*channel

		execQuery := false

		for _, conn := range connections {

			contextWithValue := context.WithValue(conn.ctx, "payload", event.Payload)

			for _, eventName := range conn.events {
				if eventName == event.Name {
					execQuery = true
				}
			}
			if execQuery == true {
				response := schema.Exec(contextWithValue, conn.operationQuery.Payload.Query, conn.operationQuery.Payload.OperationName, conn.operationQuery.Payload.Variables)
				if len(response.Errors) == 0 {
					_ = conn.conn.WriteJSON(OperationMessage{
						ID:            conn.operationQuery.ID,
						OperationType: GQL_DATA,
						Payload:       response,
					})
				}
			}
		}
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
	}

	defer c.Close()
	for {
		operationMessage := OperationMessage{}
		err := c.ReadJSON(&operationMessage)
		if err != nil {
			fmt.Println(err)
			break
		}

		switch operationMessage.OperationType {
		case GQL_CONNECTION_INIT:
			fmt.Println(operationMessage)
			err := c.WriteJSON(OperationMessage{
				OperationType: GQL_CONNECTION_ACK,
			})
			if err != nil {
				fmt.Println(err)
			}
			break

		case GQL_START:
			payload, _ := operationMessage.Payload.(map[string]interface{})
			queryParams := QueryParams{}

			if query, ok := payload["query"].(string); ok {
				queryParams.Query = query
			}

			if operationName, ok := payload["operationName"].(string); ok {
				queryParams.OperationName = operationName
			}

			if variables, ok := payload["variables"].(map[string]interface{}); ok {
				queryParams.Variables = variables
			}

			events := []string{}
			doc, _ := query.Parse(queryParams.Query)
			for _, operation := range doc.Operations {
				for _, selection := range operation.Selections {
					field, ok := selection.(*query.Field)
					if ok {
						events = append(events, field.Name.Name)
					}
				}
			}

			connections = append(connections, &ConnectionContext{
				conn: c,
				ctx:  r.Context(),
				operationQuery: OperationQuery{
					ID:            operationMessage.ID,
					OperationType: operationMessage.OperationType,
					Payload:       queryParams,
				},
				events:  events,
				request: r,
			})

			break
		}

	}
}
