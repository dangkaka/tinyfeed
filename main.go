package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/segmentio/kafka-go"
)

var kafkaHost []string = []string{"192.168.99.100:32788", "192.168.99.100:32787", "192.168.99.100:32786"}
var kafkaTopic = "feed"
var kafkaProcessedTopic = "processedFeed"

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/", writeHandler).Methods("POST")
	r.HandleFunc("/feed", viewFeed).Methods("GET")

	go readHandler()

	fmt.Println("Listening on port 8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}

func viewFeed(w http.ResponseWriter, r *http.Request) {
	k := kafka.NewReader(kafka.ReaderConfig{
		Brokers:   kafkaHost,
		Topic:     kafkaProcessedTopic,
		Partition: 0,
		MinBytes:  10e3, // 10KB
		MaxBytes:  10e6, // 10MB
	})
	var result map[int64]string
	result = make(map[int64]string)
	for {
		m, err := k.ReadMessage(context.Background())
		if err != nil {
			break
		}
		result[m.Offset] = string(m.Value)
		k.Close()
	}
	var response []byte
	json, err := json.Marshal(result)
	if err != nil {
		response = []byte("{\"status\":\"not ok\",\"message\":\"unable to marshal json\"}")
	}

	response = []byte(json)
	w.Write(response)
}

func writeHandler(w http.ResponseWriter, r *http.Request) {
	var key string
	if val, ok := r.URL.Query()["key"]; ok {
		key = val[0]
	} else {
		w.Write(respond(map[string]string{
			"status":  "not ok",
			"message": "parameter `key` is missing",
		}))
		return
	}

	var value string
	if val, ok := r.URL.Query()["value"]; ok {
		value = val[0]
	} else {
		w.Write(respond(map[string]string{
			"status":  "not ok",
			"message": "parameter `value` is missing",
		}))
		return
	}

	k := kafka.NewWriter(kafka.WriterConfig{
		Brokers:  kafkaHost,
		Topic:    kafkaTopic,
		Balancer: &kafka.LeastBytes{},
	})

	defer k.Close()

	k.WriteMessages(
		context.Background(),
		kafka.Message{
			Key:   []byte(key),
			Value: []byte(value),
		},
	)

	w.Write(respond(map[string]string{
		"status": "ok",
		"key":    key,
		"value":  value,
	}))
}

func readHandler() {
	k := kafka.NewReader(kafka.ReaderConfig{
		Brokers:   kafkaHost,
		Topic:     kafkaTopic,
		Partition: 0,
		MinBytes:  10e3, // 10KB
		MaxBytes:  10e6, // 10MB
	})

	w := kafka.NewWriter(kafka.WriterConfig{
		Brokers:  kafkaHost,
		Topic:    kafkaProcessedTopic,
		Balancer: &kafka.LeastBytes{},
	})

	defer k.Close()

	for {
		m, err := k.ReadMessage(context.Background())
		if err != nil {
			break
		}

		fmt.Printf(
			"[CONSUMER] message at offset %d: %s = %s\n",
			m.Offset,
			string(m.Key),
			string(m.Value),
		)

		w.WriteMessages(
			context.Background(),
			kafka.Message{
				Key:   []byte(string(m.Key)),
				Value: []byte("Processed " + string(m.Value)),
			},
		)
	}
}

func respond(o map[string]string) []byte {
	json, err := json.Marshal(o)
	if err != nil {
		return []byte("{\"status\":\"not ok\",\"message\":\"unable to marshal json\"}")
	}

	return []byte(json)
}
