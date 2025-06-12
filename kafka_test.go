package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
)

func TestKafkaPublishing(t *testing.T) {
	// Test message
	msg := KafkaMessage{
		HostIP:         "10.27.0.21",
		Region:         "s2r1",
		NumHypervisors: 1,
		RegionID:       98,
		Token:          "zyx",
		CloudProvider:  "oracle",
		Operation:      "setup_hypervisor",
	}

	// Create Kafka writer
	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers:      []string{kafkaBroker},
		Topic:        kafkaTopic,
		Balancer:     &kafka.LeastBytes{},
		Async:        true,
		RequiredAcks: 1,
	})
	defer writer.Close()

	// Marshal message
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Publish message
	err = writer.WriteMessages(ctx, kafka.Message{
		Value: msgBytes,
	})
	if err != nil {
		t.Fatalf("Failed to publish message: %v", err)
	}

	t.Logf("Successfully published message to Kafka: %+v", msg)
}

// TestKafkaConnection tests if we can connect to Kafka
func TestKafkaConnection(t *testing.T) {
	// Create Kafka writer
	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers:      []string{kafkaBroker},
		Topic:        kafkaTopic,
		Balancer:     &kafka.LeastBytes{},
		Async:        true,
		RequiredAcks: 1,
	})
	defer writer.Close()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try to write a test message
	err := writer.WriteMessages(ctx, kafka.Message{
		Value: []byte("test connection"),
	})
	if err != nil {
		t.Fatalf("Failed to connect to Kafka: %v", err)
	}

	t.Log("Successfully connected to Kafka")
}
