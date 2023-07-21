package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/gezimbll/copr_builds/rpm"
	"github.com/google/uuid"
	jsoniter "github.com/json-iterator/go"
	amqp "github.com/rabbitmq/amqp091-go"
)

type CoprBuild struct {
	Build   int    `json:"build"`
	Chroot  string `json:"chroot"`
	Copr    string `json:"copr"`
	IP      string `json:"ip"`
	Owner   string `json:"owner"`
	PID     int    `json:"pid"`
	Pkg     string `json:"pkg"`
	Status  int    `json:"status"`
	User    string `json:"user"`
	Version string `json:"version"`
	What    string `json:"what"`
	Who     string `json:"who"`
}

func main() {
	cert, err := tls.LoadX509KeyPair("/etc/fedora-messaging/fedora-cert.pem", "/etc/fedora-messaging/fedora-key.pem")
	if err != nil {
		log.Fatal(err)
	}

	caCert, err := os.ReadFile("/etc/fedora-messaging/cacert.pem")
	if err != nil {
		log.Fatal(err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
	}
	conn, err := amqp.DialTLS_ExternalAuth("amqps://fedora:@rabbitmq.fedoraproject.org/%2Fpublic_pubsub", tlsConfig)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to open a channel: %v", err)
	}

	queueUUID := uuid.New()
	queue, err := ch.QueueDeclare(
		queueUUID.String(),
		false,
		true,
		false,
		false,
		nil,
	)
	if err != nil {
		fmt.Println("Error declaring queue:", err)
		return
	}
	err = ch.QueueBind(
		queue.Name,
		"org.fedoraproject.prod.copr.build.end",
		"amq.topic",
		false,
		nil,
	)
	if err != nil {
		fmt.Println("Error binding queue:", err)
		return
	}

	msgs, err := ch.Consume(
		queue.Name,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		fmt.Println("Error consuming messages:", err)
		return
	}
	var wg sync.WaitGroup
	done := make(chan bool)
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	c := &CoprBuild{}

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case msg, ok := <-msgs:
					if !ok {
						return
					}
					iter := jsoniter.ParseBytes(json, msg.Body)
					var owner string
					for field := iter.ReadObject(); field != ""; field = iter.ReadObject() {
						if field == "owner" {
							owner = iter.ReadString()
							break
						} else {
							iter.Skip()
						}
					}
					if owner == "gzim07" {
						if err := json.Unmarshal(msg.Body, c); err != nil {
							log.Printf("error marshalling ,<%v>", err)
						}
						if c.Version != "" {
							val, err := rpm.GenerateFiles(c.Chroot, c.Copr)
							if err != nil {
								log.Printf("error generating files ,<%v>", err)
							}
							fmt.Printf("Generated package %s\n", val)
						}
					}
				case <-done:
					return
				}
			}
		}()
	}
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		log.Println("Signal received. Shutting down...")
		close(done)
	}()

	wg.Wait()
	log.Println("All workers have finished processing. Closing connections...")

	if err := ch.Close(); err != nil {
		log.Printf("Failed to close channel: %v", err)
	}
	if err := conn.Close(); err != nil {
		log.Printf("Failed to close connection: %v", err)
	}
	log.Println("Connections closed. Exiting...")

}
