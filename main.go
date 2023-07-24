package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
	"os/signal"
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

const (
	FedoraBroker = "amqps://fedora:@rabbitmq.fedoraproject.org/%2Fpublic_pubsub"
)

func setupTLS() (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair("/etc/fedora-messaging/fedora-cert.pem", "/etc/fedora-messaging/fedora-key.pem")
	if err != nil {
		return nil, err
	}

	caCert, err := os.ReadFile("/etc/fedora-messaging/cacert.pem")
	if err != nil {
		return nil, err
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
	}
	return tlsConfig, nil
}

func setupConn(tls *tls.Config) (*amqp.Connection, *amqp.Channel, error) {
	conn, err := amqp.DialTLS_ExternalAuth(FedoraBroker, tls)
	if err != nil {
		return nil, nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		return nil, nil, err
	}
	return conn, ch, err
}

func consumeMessage(ctx context.Context, ch *amqp.Channel, queueName string) {
	errChan := make(chan error)
	fileChan := make(chan string)

	go func() {
		for {
			select {
			case err := <-errChan:
				log.Println("Error:", err)
			case file := <-fileChan:
				log.Println("File created:", file)
			case <-ctx.Done():
				log.Println("Stopping error and file logging due to context cancellation")
				return
			}
		}
	}()

	msgs, err := ch.Consume(
		queueName,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Println("Error consuming messages:", err)
		return
	}

	json := jsoniter.ConfigCompatibleWithStandardLibrary
	c := &CoprBuild{}

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping message consumption due to context cancellation")
			return
		case msg, ok := <-msgs:
			if !ok {
				return
			}
			processMessage(errChan, fileChan, msg, json, c)
		}
	}
}

func processMessage(errc chan<- error, filech chan<- string, msg amqp.Delivery, json jsoniter.API, c *CoprBuild) {
	//fmt.Println(string(msg.Body))
	//times := time.Now()
	defer msg.Ack(false)
	var owner string
	iter := jsoniter.ParseBytes(json, msg.Body)

	for field := iter.ReadObject(); field != ""; field = iter.ReadObject() {
		if field == "owner" {
			owner = iter.ReadString()
			break
		}
		iter.Skip()

	}
	if owner != "gzim07" {
		//fmt.Println(time.Since(times).Nanoseconds())
		return
	}
	if err := json.Unmarshal(msg.Body, c); err != nil {
		return
	}
	if c.Version != "" {
		go rpm.GenerateFiles(errc, filech, c.Owner, c.Chroot, c.Copr, c.Version, c.Build)
	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tlsConfig, err := setupTLS()
	if err != nil {
		log.Fatal(err)
	}
	conn, ch, err := setupConn(tlsConfig)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	defer ch.Close()

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

	go consumeMessage(ctx, ch, queue.Name)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	<-sigs
	cancel()

	log.Println("All workers have finished processing. Closing connections...")
	log.Println("Connections closed. Exiting...")
}
